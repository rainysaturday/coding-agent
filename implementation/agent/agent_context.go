package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/coding-agent/harness/inference"
)

// reportContextSize reports the current actual context size to the TUI callback.
// The actualSize parameter should be pre-computed by the caller (with lock held
// if accuracy is critical). This function only handles the callback dispatch.
func (a *Agent) reportContextSize(callback ContextSizeCallback, actualSize int, maxContextSize int) {
	if callback != nil {
		callback(actualSize, maxContextSize)
	}
}

func (a *Agent) recordIteration() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.recordIterationUnlocked()
}

// recordIterationUnlocked is the internal, unlocked version.
// Must be called while holding a.mu.
func (a *Agent) recordIterationUnlocked() {
	// Build the full context for this iteration: system prompt + current messages
	fullContext := make([]*inference.Message, 0, 1+len(a.context))
	fullContext = append(fullContext, &inference.Message{
		Role:    "system",
		Content: a.systemPrompt,
	})

	// Deep copy messages
	for _, msg := range a.context {
		cpy := *msg
		fullContext = append(fullContext, &cpy)
	}

	a.iterationHistory = append(a.iterationHistory, Iteration{
		Index:    a.stats.Iterations,
		Messages: fullContext,
		Stats: StatsInfo{
			InputTokens:     a.stats.InputTokens,
			OutputTokens:    a.stats.OutputTokens,
			ToolCalls:       a.stats.ToolCalls,
			FailedToolCalls: a.stats.FailedToolCalls,
		},
	})
}

func (a *Agent) ClearContext() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = make([]*inference.Message, 0)
	a.lastTotalTokens = 0
	a.toolResultMsgsSinceLastAPI = make(map[int]bool)
}

// AddUserMessage adds a user message to the context.
func (a *Agent) AddUserMessage(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = append(a.context, &inference.Message{
		Role:    "user",
		Content: message,
	})
}

// AddAssistantMessage adds an assistant message to the context.
func (a *Agent) AddAssistantMessage(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = append(a.context, &inference.Message{
		Role:    "assistant",
		Content: message,
	})
}

// GetContextSize returns the current context size.
// Uses total_tokens from the last API response as the authoritative count.
func (a *Agent) GetContextSize() int {
	return a.GetActualContextSize()
}

// GetActualContextSize returns the exact total_tokens from the last API response,
// which is the authoritative count of everything the API processed:
// system prompt + messages + tools + completion.
// Then adds estimated tokens for any new messages added after the response
// (e.g., tool results that were appended during tool execution).
func (a *Agent) GetActualContextSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.getActualContextSizeUnlocked()
}

// getActualContextSizeUnlocked is the internal, unlocked version.
// Must be called while holding a.mu.
func (a *Agent) getActualContextSizeUnlocked() int {
	if a.lastTotalTokens > 0 {
		// Only count tool messages added AFTER the last API call.
		// Messages before the API call were already included in total_tokens.
		delta := 0
		for idx, msg := range a.context {
			if msg.Role == "tool" && a.toolResultMsgsSinceLastAPI[idx] {
				delta += 3 + inference.EstimateTokens(msg.Content)
			}
		}
		return a.lastTotalTokens + delta
	}
	// No API response yet, estimate from scratch
	return inference.EstimateContextSize(a.context, a.inference.GetTools(), a.systemPrompt)
}

// shouldCompress checks if context compression is needed based on actual context window usage.
func (a *Agent) shouldCompress() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	total := a.getActualContextSizeUnlocked()
	maxSize := a.maxContextSize
	return total > int(float64(maxSize)*0.8)
}

// compressContext compresses the conversation history while preserving system prompt.
// After compression, the context is: [first_user_message, assistant_summary, ...last_3_messages...]
func (a *Agent) compressContext(ctx context.Context) error {
	a.mu.Lock()
	if len(a.context) <= 2 {
		a.mu.Unlock()
		return nil // Nothing to compress
	}

	// Keep the first user message and last few messages
	preserveCount := 3
	if len(a.context) <= preserveCount+1 {
		a.mu.Unlock()
		return nil
	}

	messages := make([]*inference.Message, len(a.context))
	copy(messages, a.context)
	a.mu.Unlock()

	// Track duration for display
	startTime := time.Now()

	// First user message is always preserved
	firstUserIdx := 0
	firstUserMsg := messages[0]
	// Ensure we actually have a user message as the first message.
	// If the first message is not a user message (e.g., assistant added
	// programmatically, a goal-injected message, or a loaded context), find the
	// first real user message so the summary range starts after it. Otherwise use
	// the first message as fallback.
	if firstUserMsg.Role != "user" {
		for i, msg := range messages {
			if msg.Role == "user" {
				firstUserMsg = msg
				firstUserIdx = i
				break
			}
		}
	}

	// Messages to summarize: everything after the first user msg (exclusive) and
	// before the last N preserved messages.
	// Clamp the range so we never slice past the end or produce start > end. This
	// guards against the case where the first user message sits within the last
	// preserveCount messages (e.g. a goal-injected or loaded context), which would
	// otherwise trigger a slice-bounds panic. If the range is empty, the summary is
	// simply a no-op over the (non-existent) middle section.
	start := firstUserIdx + 1
	end := len(messages) - preserveCount
	if start > end {
		start = end
	}
	summaryMessages := messages[start:end]

	// Filter summary messages to remove orphaned tool results (tool messages without
	// a preceding assistant message that made the tool call). These can occur if the
	// boundary falls within a tool call sequence, and including them would confuse
	// the summarizer since they have no context.
	var filteredSummary []*inference.Message
	lastNonToolRole := ""
	for _, msg := range summaryMessages {
		if msg.Role == "tool" && lastNonToolRole != "assistant" {
			// Orphaned tool result — skip it
			continue
		}
		filteredSummary = append(filteredSummary, msg)
		if msg.Role != "tool" {
			lastNonToolRole = msg.Role
		}
	}

	// Build summary prompt
	summaryReq := fmt.Sprintf("Summarize the following conversation history concisely, preserving key information, decisions, and results:\n\n")
	for _, msg := range filteredSummary {
		summaryReq += fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content)
	}
	summaryReq += "\nProvide a concise summary that captures all essential information."

	// Get summary from LLM
	summaryMsg := &inference.Message{Role: "user", Content: summaryReq}
	response, err := a.inference.InferenceRequest(ctx, []*inference.Message{summaryMsg}, "You are a conversation summarizer.")
	if err != nil {
		// Log compression failure via stream callback
		if a.streamCallback != nil {
			a.streamCallback(inference.StreamingChunk{
				Text:        fmt.Sprintf("\n[Warning] Context compression failed: %v\n", err),
				ContentType: inference.StreamingContentTypeNormal,
			})
		}
		return fmt.Errorf("failed to compress context: %w", err)
	}

	// Rebuild context: first_user_message + assistant_summary + preserved messages
	// Note: we do NOT add the system prompt here because buildMessages() in the
	// inference client prepends it on every API call. Adding it here would cause
	// duplicate system prompts.
	a.mu.Lock()
	newContext := make([]*inference.Message, 0, preserveCount+2)
	newContext = append(newContext, firstUserMsg) // Preserve the original user prompt

	// Summary is stored as an assistant message to maintain conversation flow
	summaryContent := "Conversation summary: " + response.Content
	newContext = append(newContext, &inference.Message{Role: "assistant", Content: summaryContent})

	// Preserve the last N messages, but reorder to maintain conversation integrity.
	// Group consecutive tool results with their preceding assistant message so
	// that we never have a tool message without its corresponding assistant message.
	preserved := messages[len(messages)-preserveCount:]
	grouped := a.groupAssistantToolMessages(preserved)
	newContext = append(newContext, grouped...)

	// Record the pre-compression state so the full history is preserved before context is modified
	a.recordIterationUnlocked()

	a.context = newContext
	a.compressionCount++

	// Set lastTotalTokens to the estimated size of the compressed context so that
	// context size reporting remains consistent until the next API response.
	// This prevents a temporary distortion where getActualContextSizeUnlocked()
	// falls back to a rough EstimateContextSize() heuristic.
	a.lastTotalTokens = inference.EstimateContextSize(newContext, a.inference.GetTools(), a.systemPrompt)

	// Reset tool result tracking since the context has been rebuilt.
	a.toolResultMsgsSinceLastAPI = make(map[int]bool)

	// Preserve cumulative token stats. The stats represent the total tokens used
	// across the entire session. We should NOT overwrite them with the compressed
	// context size; instead, the next API call will add its own token usage on top
	// of the existing cumulative totals.
	// (No change to a.stats.InputTokens / a.stats.OutputTokens)
	a.mu.Unlock()

	// Display compression success via stream callback with duration
	if a.streamCallback != nil {
		duration := time.Since(startTime)
		a.streamCallback(inference.StreamingChunk{
			Text:        fmt.Sprintf("[Context compressed in %.2fs]\n", duration.Seconds()),
			ContentType: inference.StreamingContentTypeCompression,
		})
	}

	return nil
}

// groupAssistantToolMessages takes a slice of messages and reorders them so that
// tool results are always immediately after their corresponding assistant message.
// This prevents broken sequences like: [user_summary, tool_result, assistant, user].
func (a *Agent) groupAssistantToolMessages(messages []*inference.Message) []*inference.Message {
	if len(messages) == 0 {
		return messages
	}

	// Build groups: each group starts with a non-tool message, followed by its
	// tool results (if the preceding message was an assistant with tool calls).
	var groups [][]*inference.Message
	currentGroup := []*inference.Message{messages[0]}

	for i := 1; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role == "tool" {
			// Tool results should be grouped with the preceding assistant message.
			// If the current group's last message is an assistant, keep it here.
			// Otherwise, we need to move it.
			if len(currentGroup) > 0 && currentGroup[len(currentGroup)-1].Role == "assistant" {
				currentGroup = append(currentGroup, msg)
			} else {
				// This tool result's assistant is not in the preserved set.
				// Skip it to avoid broken causality.
				continue
			}
		} else {
			// Non-tool message starts a new group.
			if len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
			}
			currentGroup = []*inference.Message{msg}
		}
	}
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	// Flatten groups, preserving order within each group.
	result := make([]*inference.Message, 0, len(messages))
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}
