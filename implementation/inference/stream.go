package inference

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/coding-agent/harness/tools"
)

// accumulatedToolCall tracks a single tool call across streaming deltas.
type accumulatedToolCall struct {
	ID        string
	Type      string
	Name      string
	Arguments string
}

// streamState holds the state for processing a streaming response.
type streamState struct {
	fullContent     strings.Builder
	fullReasoning   strings.Builder
	fullRefusal     strings.Builder
	reasoningType   string
	totalTokens     int
	inputTokens     int
	outputTokens    int
	streamEnded     bool
	toolCallsList   []*accumulatedToolCall
	notifiedCalls   map[int]bool
	lastActiveIndex int
	callback        StreamingCallbackWithType
	maxDisplayWidth int
}

// newStreamState creates a new streamState with initialized fields.
func newStreamState(callback StreamingCallbackWithType, maxDisplayWidth int) *streamState {
	return &streamState{
		notifiedCalls:   make(map[int]bool),
		lastActiveIndex: -1,
		callback:        callback,
		maxDisplayWidth: maxDisplayWidth,
	}
}

// processToolCallDelta accumulates a single tool call delta into the shared list.
func (ss *streamState) processToolCallDelta(deltaTC *APIToolCall) {
	targetIndex := -1

	// 1. Try to use the index field if present
	if deltaTC.Index != nil && *deltaTC.Index >= 0 {
		targetIndex = *deltaTC.Index
	}

	// 2. If no index, try to find by ID
	if targetIndex == -1 && deltaTC.ID != "" {
		for i, tc := range ss.toolCallsList {
			if tc.ID == deltaTC.ID {
				targetIndex = i
				break
			}
		}
	}

	// 3. If still not found and this is a continuation delta (no ID, no name),
	// find the first tool call with empty arguments to match it correctly
	if targetIndex == -1 && deltaTC.ID == "" && deltaTC.Function.Name == "" && len(ss.toolCallsList) > 0 {
		if deltaTC.Function.Arguments != "" {
			found := false
			for i, tc := range ss.toolCallsList {
				if tc.Arguments == "" {
					targetIndex = i
					found = true
					break
				}
			}
			if !found {
				targetIndex = ss.lastActiveIndex
				if targetIndex < 0 {
					targetIndex = len(ss.toolCallsList) - 1
				}
			}
		}
	}

	if targetIndex == -1 {
		// New tool call - create a new entry at the next index
		targetIndex = len(ss.toolCallsList)
	}

	// Ensure the slice is large enough
	for len(ss.toolCallsList) <= targetIndex {
		ss.toolCallsList = append(ss.toolCallsList, &accumulatedToolCall{})
	}

	existing := ss.toolCallsList[targetIndex]

	// Update last active tool call index when we see a new tool call start
	if deltaTC.ID != "" || deltaTC.Function.Name != "" {
		ss.lastActiveIndex = targetIndex
	}

	// Merge with existing tool call - accumulate fields
	if deltaTC.ID != "" {
		existing.ID = deltaTC.ID
	}
	// Type - normalize empty values to "function" for Copilot compatibility
	if deltaTC.Type != "" {
		existing.Type = deltaTC.Type
	} else if existing.Type == "" {
		existing.Type = "function"
	}
	if deltaTC.Function.Name != "" {
		existing.Name = deltaTC.Function.Name
	}
	if deltaTC.Function.Arguments != "" {
		existing.Arguments += deltaTC.Function.Arguments
	}

	// Notify about new tool call if callback is available
	if ss.callback != nil && deltaTC.Function.Name != "" && !ss.notifiedCalls[targetIndex] {
		toolName := deltaTC.Function.Name
		notification := fmt.Sprintf("\n[Tool Call] %s", toolName)
		ss.callback(StreamingChunk{
			Text:        notification,
			ContentType: StreamingContentTypeNormal,
			IsToolCall:  true,
		})
		ss.notifiedCalls[targetIndex] = true
	}

	// Stream accumulated arguments as they arrive (if we have any and have seen the name)
	if ss.callback != nil && existing.Name != "" && existing.Arguments != "" {
		// Format the accumulated arguments for display
		var prettyArgs string
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(existing.Arguments), &params); err == nil {
			// Calculate available width for arguments
			// Format: " (key: value, ...)"
			// Reserve space for " (" (2 chars) and ")" (1 char)
			argsMaxWidth := ss.maxDisplayWidth - 3
			if argsMaxWidth < 20 {
				argsMaxWidth = 20 // Minimum width for readability
			}
			prettyArgs = formatToolCallArgs(params, argsMaxWidth)
		} else {
			// If JSON parsing fails, show raw arguments (truncated if needed)
			prettyArgs = existing.Arguments
			if ss.maxDisplayWidth > 0 && len(prettyArgs) > ss.maxDisplayWidth-3 {
				prettyArgs = prettyArgs[:ss.maxDisplayWidth-6] + "..."
			}
		}
		argsUpdate := fmt.Sprintf("[Tool Call] %s (%s)", existing.Name, prettyArgs)
		ss.callback(StreamingChunk{
			Text:        argsUpdate,
			ContentType: StreamingContentTypeNormal,
			IsToolCall:  true,
		})
	}
}

// extractTokenUsage extracts token usage from a chunk's Usage and Timings fields.
func (ss *streamState) extractTokenUsage(usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}, timings struct {
	CacheN     int `json:"cache_n"`
	PromptN    int `json:"prompt_n"`
	PredictedN int `json:"predicted_n"`
}) {
	if usage.TotalTokens > 0 {
		ss.totalTokens = usage.TotalTokens
		ss.inputTokens = usage.PromptTokens
		ss.outputTokens = usage.CompletionTokens
	} else if timings.CacheN+timings.PromptN > 0 {
		// llama.cpp timings format - cache_n is NOT included in prompt_n
		ss.inputTokens = timings.CacheN + timings.PromptN
		ss.outputTokens = timings.PredictedN
		ss.totalTokens = ss.inputTokens + ss.outputTokens
	}
}

// processDelta processes a single delta from a streaming chunk.
func (ss *streamState) processDelta(delta Message) {
	if delta.Content != "" {
		ss.fullContent.WriteString(delta.Content)
		if ss.callback != nil {
			ss.callback(StreamingChunk{
				Text:        delta.Content,
				ContentType: StreamingContentTypeNormal,
			})
		}
	}
	if delta.Reasoning != "" {
		ss.fullReasoning.WriteString(delta.Reasoning)
		if ss.reasoningType == "" {
			ss.reasoningType = "reasoning"
		}
		if ss.callback != nil {
			ss.callback(StreamingChunk{
				Text:        delta.Reasoning,
				ContentType: StreamingContentTypeReasoning,
			})
		}
	}
	if delta.ReasoningContent != "" {
		ss.fullReasoning.WriteString(delta.ReasoningContent)
		if ss.reasoningType == "" {
			ss.reasoningType = "reasoning_content"
		}
		if ss.callback != nil {
			ss.callback(StreamingChunk{
				Text:        delta.ReasoningContent,
				ContentType: StreamingContentTypeReasoning,
			})
		}
	}
	if delta.Refusal != "" {
		ss.fullRefusal.WriteString(delta.Refusal)
		if ss.callback != nil {
			ss.callback(StreamingChunk{
				Text:        delta.Refusal,
				ContentType: StreamingContentTypeNormal,
			})
		}
	}
	for i := range delta.ToolCalls {
		ss.processToolCallDelta(delta.ToolCalls[i])
	}
}

// buildStreamResponse converts accumulated streaming data into a Response.
func (ss *streamState) buildStreamResponse() *Response {
	content := ss.fullContent.String()

	var apiToolCalls []*APIToolCall
	var toolCalls []*tools.ToolCall

	// Process tool calls in order
	for _, accTC := range ss.toolCallsList {
		// Skip empty tool calls (those that were only created for merging)
		if accTC.Name == "" && accTC.Arguments == "" {
			continue
		}

		// Ensure type is set for compatibility - normalize empty values to "function"
		if accTC.Type == "" {
			accTC.Type = "function"
		}

		// Create API tool call for reference
		apiTC := &APIToolCall{
			ID:   accTC.ID,
			Type: accTC.Type,
			Function: FunctionCall{
				Name:      accTC.Name,
				Arguments: accTC.Arguments,
			},
		}
		apiToolCalls = append(apiToolCalls, apiTC)

		// Parse the accumulated arguments JSON string
		var params map[string]interface{}
		if accTC.Arguments != "" {
			if err := json.Unmarshal([]byte(accTC.Arguments), &params); err != nil {
				params = map[string]interface{}{
					"_raw_arguments": accTC.Arguments,
					"_parse_error":   err.Error(),
				}
			}
		}

		toolCall := &tools.ToolCall{
			ID:         accTC.ID,
			Name:       accTC.Name,
			Parameters: params,
			Raw:        accTC.Arguments,
		}
		toolCalls = append(toolCalls, toolCall)
	}

	return &Response{
		Content:              content,
		Reasoning:            ss.fullReasoning.String(),
		ReasoningContentType: ss.reasoningType,
		Refusal:              ss.fullRefusal.String(),
		ToolCalls:            toolCalls,
		APIToolCalls:         apiToolCalls,
		TokenUsage:           ss.totalTokens,
		InputTokens:          ss.inputTokens,
		OutputTokens:         ss.outputTokens,
	}
}

// sseChunk is a single parsed SSE data payload from a streaming response.
// It is a named type so each loop iteration can allocate a fresh, zero-valued
// instance instead of reusing one struct and manually resetting its fields
// (which risks stale data if a new field is added later).
type sseChunk struct {
	Choices []struct {
		Delta        Message `json:"delta"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Timings struct {
		CacheN     int `json:"cache_n"`
		PromptN    int `json:"prompt_n"`
		PredictedN int `json:"predicted_n"`
	} `json:"timings"`
}

// handleStreamResponse processes a streaming response from the LLM API.
// It reads SSE lines, accumulates content, reasoning, and tool calls,
// and returns a complete Response.
func (ic *InferenceClient) handleStreamResponse(body io.Reader, callback StreamingCallbackWithType) (*Response, error) {
	ss := newStreamState(callback, ic.maxDisplayWidth)

	scanner := bufio.NewScanner(body)
	// Increase the scanner's maximum token size beyond the default 64KB limit so
	// that large SSE data lines (e.g. big tool-call argument JSON blobs) do not
	// cause bufio.ErrTooLong and abort the whole response.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var jsonBuffer strings.Builder
	inJSON := false

	for scanner.Scan() {
		line := scanner.Text()

		// Allocate a fresh, zero-valued chunk for each SSE payload so no stale
		// fields leak between iterations (json.Unmarshal does not clear existing
		// slice values, so reusing a struct requires error-prone manual resets).
		chunk := sseChunk{}

		// Handle SSE data lines
		if strings.HasPrefix(line, "data: ") {
			// If we were accumulating multi-line JSON, flush and process it first
			if inJSON && jsonBuffer.Len() > 0 {
				inJSON = false
				var bufferChunk sseChunk
				if err := json.Unmarshal([]byte(jsonBuffer.String()), &bufferChunk); err != nil {
					// On parse failure, keep the buffer and continue accumulating.
					// The next chunk may complete the JSON. Only reset if we see a
					// new "data:" line that parses successfully.
					inJSON = true
				} else {
					// Process the buffered chunk data immediately
					// Extract token usage unconditionally so a usage-only
					// chunk (empty choices) is still recorded.
					ss.extractTokenUsage(bufferChunk.Usage, bufferChunk.Timings)
					if len(bufferChunk.Choices) > 0 {
						ss.processDelta(bufferChunk.Choices[0].Delta)
					}
					jsonBuffer.Reset()
				}
			} else if inJSON {
				jsonBuffer.Reset()
			}

			// Reset inJSON since we're now handling a new data line
			inJSON = false

			data := strings.TrimPrefix(line, "data: ")

			// Debug verbose-verbose: log raw SSE data before any processing
			if ic.debugVerboseVerbose {
				fmt.Fprintf(os.Stderr, "[DEBUG-VERBOSE-VERBOSE] >>> SSE raw body: %s\n", data)
			}

			// Check for end of stream
			if data == "[DONE]" {
				ss.streamEnded = true
				break
			}

			// Try to parse as complete JSON first
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// If it failed, check if this is a multi-line JSON blob
				// by seeing if the data looks like it's incomplete
				jsonBuffer.WriteString(data)
				inJSON = true
				continue
			}
		} else if inJSON {
			// We are in the middle of accumulating multi-line JSON,
			// append this line (without SSE prefix) to the buffer
			jsonBuffer.WriteString(line)
			continue
		} else {
			// Skip empty lines and non-SSE data when not accumulating JSON
			continue
		}

		// Get token usage - also track input/output separately. This is done
		// unconditionally because OpenAI-compatible servers report usage in a
		// final chunk with an empty "choices" array (stream_options
		// include_usage); extracting it only inside the choices guard would drop
		// those tokens.
		ss.extractTokenUsage(chunk.Usage, chunk.Timings)

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta

			// Accumulate content and reasoning
			if delta.Content != "" {
				ss.fullContent.WriteString(delta.Content)
			}
			if delta.Reasoning != "" {
				ss.fullReasoning.WriteString(delta.Reasoning)
				if ss.reasoningType == "" {
					ss.reasoningType = "reasoning"
				}
			}
			if delta.ReasoningContent != "" {
				ss.fullReasoning.WriteString(delta.ReasoningContent)
				if ss.reasoningType == "" {
					ss.reasoningType = "reasoning_content"
				}
			}
			if delta.Refusal != "" {
				ss.fullRefusal.WriteString(delta.Refusal)
			}

			// Accumulate tool calls from streaming delta
			for i := range delta.ToolCalls {
				ss.processToolCallDelta(delta.ToolCalls[i])
			}

			// Stream reasoning content with appropriate type
			if delta.ReasoningContent != "" && callback != nil {
				callback(StreamingChunk{
					Text:        delta.ReasoningContent,
					ContentType: StreamingContentTypeReasoning,
				})
			}

			// Stream reasoning content with appropriate type
			if delta.Reasoning != "" && callback != nil {
				callback(StreamingChunk{
					Text:        delta.Reasoning,
					ContentType: StreamingContentTypeReasoning,
				})
			}

			// Stream normal content with appropriate type
			if delta.Content != "" && callback != nil {
				callback(StreamingChunk{
					Text:        delta.Content,
					ContentType: StreamingContentTypeNormal,
				})
			}

			// Stream refusal content (content-policy refusals) like normal text
			if delta.Refusal != "" && callback != nil {
				callback(StreamingChunk{
					Text:        delta.Refusal,
					ContentType: StreamingContentTypeNormal,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream error (scanner failure): %w", err)
	}

	// Check if the stream ended unexpectedly (without [DONE] marker)
	if !ss.streamEnded {
		return nil, fmt.Errorf("stream ended unexpectedly (EOF before [DONE] marker)")
	}

	return ss.buildStreamResponse(), nil
}
