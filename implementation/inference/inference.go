package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"coding-agent/implementation/config"
	"coding-agent/implementation/types"
)

type Client struct {
	cfg  config.Config
	http *http.Client
}

type StreamCallback func(delta types.StreamDelta)

func New(cfg config.Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: config.DefaultHTTPTimeout()}}
}

func (c *Client) endpoint() string {
	base := strings.TrimRight(c.cfg.APIEndpoint, "/")
	if c.isCopilotEndpoint() {
		return base + "/chat/completions"
	}
	if c.isGitHubModelsEndpoint() {
		return base + "/inference/chat/completions"
	}
	return base + "/v1/chat/completions"
}

func (c *Client) isCopilotEndpoint() bool {
	return strings.Contains(strings.ToLower(c.cfg.APIEndpoint), "githubcopilot.com")
}

func (c *Client) isGitHubModelsEndpoint() bool {
	return strings.Contains(strings.ToLower(c.cfg.APIEndpoint), "models.github.ai")
}

func (c *Client) formatHTTPError(statusCode int, status string, raw []byte) error {
	body := strings.TrimSpace(string(raw))
	lower := strings.ToLower(body)
	tokenKindHint := ""
	if strings.HasPrefix(c.cfg.APIKey, "github_pat_") {
		tokenKindHint = " Detected token type: github_pat (PAT)."
	} else if strings.HasPrefix(c.cfg.APIKey, "gho_") {
		tokenKindHint = " Detected token type: gho_ (GitHub OAuth token)."
	}

	if c.isCopilotEndpoint() && strings.Contains(lower, "personal access tokens are not supported") {
		return fmt.Errorf("inference failed: %s: %s\nhint: api.githubcopilot.com does not accept Personal Access Tokens (github_pat). Use a Copilot user token (ghu_) for this endpoint.%s Alternatively switch to CODING_AGENT_API_ENDPOINT=https://models.github.ai when using PAT/OAuth tokens", status, body, tokenKindHint)
	}

	if c.isCopilotEndpoint() && (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) {
		return fmt.Errorf("inference failed: %s: %s\nhint: GitHub Copilot authentication failed. api.githubcopilot.com requires a Copilot user token (ghu_).%s If you only have PAT/OAuth tokens, use CODING_AGENT_API_ENDPOINT=https://models.github.ai", status, body, tokenKindHint)
	}

	return fmt.Errorf("inference failed: %s: %s", status, body)
}

func shouldRetryStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= http.StatusInternalServerError
}

func normalizeMessages(messages []types.Message) []types.Message {
	out := make([]types.Message, len(messages))
	copy(out, messages)

	for i := range out {
		if len(out[i].ToolCalls) == 0 {
			continue
		}
		calls := make([]types.ToolCall, len(out[i].ToolCalls))
		copy(calls, out[i].ToolCalls)
		for j := range calls {
			if strings.TrimSpace(calls[j].Type) == "" {
				calls[j].Type = "function"
			}
		}
		out[i].ToolCalls = calls
	}

	return out
}

func mergeToolCallDelta(accum []types.ToolCall, index int, deltaID, deltaType, deltaName, deltaArgs string) []types.ToolCall {
	for len(accum) <= index {
		accum = append(accum, types.ToolCall{})
	}
	c := &accum[index]
	if strings.TrimSpace(deltaID) != "" {
		c.ID = deltaID
	}
	if strings.TrimSpace(deltaType) != "" {
		c.Type = deltaType
	}
	if strings.TrimSpace(deltaName) != "" {
		c.Function.Name = deltaName
	}
	if deltaArgs != "" {
		c.Function.Arguments += deltaArgs
	}
	if strings.TrimSpace(c.Type) == "" {
		c.Type = "function"
	}
	return accum
}

func (c *Client) Infer(ctx context.Context, messages []types.Message, tools []types.ToolDefinition) (types.ChatResponse, error) {
	body := map[string]interface{}{
		"model":       c.cfg.Model,
		"messages":    normalizeMessages(messages),
		"tools":       tools,
		"temperature": c.cfg.Temperature,
		"max_tokens":  c.cfg.MaxTokens,
		"stream":      false,
	}

	var out types.ChatResponse
	if err := c.doJSON(ctx, body, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *Client) InferStream(ctx context.Context, messages []types.Message, tools []types.ToolDefinition, cb StreamCallback) (types.Usage, error) {
	body := map[string]interface{}{
		"model":       c.cfg.Model,
		"messages":    normalizeMessages(messages),
		"tools":       tools,
		"temperature": c.cfg.Temperature,
		"max_tokens":  c.cfg.MaxTokens,
		"stream":      true,
		"stream_options": map[string]interface{}{
			"include_usage": true,
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return types.Usage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(b))
	if err != nil {
		return types.Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	if strings.Contains(strings.ToLower(c.cfg.APIEndpoint), "copilot") {
		req.Header.Set("Editor-Version", "vscode/1.99.0")
		req.Header.Set("Copilot-Integration-Id", "vscode-chat")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return types.Usage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return types.Usage{}, c.formatHTTPError(resp.StatusCode, resp.Status, raw)
	}

	var usage types.Usage
	var aggregatedCalls []types.ToolCall
	s := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 1024*1024)
	s.Buffer(buf, 10*1024*1024)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			cb(types.StreamDelta{Done: true, Usage: usage})
			return usage, nil
		}
		var evt struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage types.Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}
		if evt.Usage.TotalTokens > 0 {
			usage = evt.Usage
		}
		if len(evt.Choices) == 0 {
			continue
		}
		for _, tc := range evt.Choices[0].Delta.ToolCalls {
			aggregatedCalls = mergeToolCallDelta(
				aggregatedCalls,
				tc.Index,
				tc.ID,
				tc.Type,
				tc.Function.Name,
				tc.Function.Arguments,
			)
		}
		d := types.StreamDelta{
			Content:          evt.Choices[0].Delta.Content,
			ReasoningContent: evt.Choices[0].Delta.ReasoningContent,
			ToolCalls:        aggregatedCalls,
		}
		cb(d)
	}
	if err := s.Err(); err != nil {
		return usage, err
	}
	return usage, nil
}

func (c *Client) doJSON(ctx context.Context, reqBody any, out *types.ChatResponse) error {
	b, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if c.cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		}
		if strings.Contains(strings.ToLower(c.cfg.APIEndpoint), "copilot") {
			req.Header.Set("Editor-Version", "vscode/1.99.0")
			req.Header.Set("Copilot-Integration-Id", "vscode-chat")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt*attempt) * 200 * time.Millisecond)
			continue
		}
		raw, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode >= 300 {
			lastErr = c.formatHTTPError(resp.StatusCode, resp.Status, raw)
			if shouldRetryStatus(resp.StatusCode) {
				time.Sleep(time.Duration(attempt*attempt) * 200 * time.Millisecond)
				continue
			}
			return lastErr
		}

		var parsed struct {
			Choices []struct {
				Message types.Message `json:"message"`
			} `json:"choices"`
			Usage types.Usage `json:"usage"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			lastErr = err
			continue
		}
		if len(parsed.Choices) == 0 {
			return fmt.Errorf("empty choices from model")
		}
		out.Message = parsed.Choices[0].Message
		out.Usage = parsed.Usage
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("inference failed")
}
