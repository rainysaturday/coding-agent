// Package inference implements an OpenAI compatible chat completion client
// with streaming, tool calls, reasoning field detection and backend specific
// quirks (llama.cpp timings, GitHub Copilot, GitHub Models).
package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/debug"
	"github.com/coding-agent/coding-agent/version"
)

// Client talks to an OpenAI compatible inference backend.
type Client struct {
	cfg    *config.Config
	http   *http.Client
	mu     sync.Mutex
	reason string // detected reasoning field name
}

// New creates a client for the given configuration.
func New(cfg *config.Config) *Client {
	// Connection and read timeouts are configured separately (requirement
	// 010). ConnectTimeout bounds dialing and the TLS handshake; ReadTimeout
	// bounds the wait for response headers. A zero ReadTimeout falls back to
	// the initial token timeout so a server that accepts the connection but
	// never responds is still caught.
	connect := time.Duration(cfg.ConnectTimeout) * time.Second
	if connect <= 0 {
		connect = time.Duration(config.DefaultConnectTimeout) * time.Second
	}
	tr := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: connect, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        8,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: connect,
	}
	read := time.Duration(cfg.ReadTimeout) * time.Second
	if read <= 0 && cfg.InitialTokenTimeout > 0 {
		read = time.Duration(cfg.InitialTokenTimeout) * time.Second
	}
	if read > 0 {
		tr.ResponseHeaderTimeout = read
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Transport: tr},
		// No overall timeout: streaming may legitimately run for hours.
	}
}

// Config returns the configuration in use.
func (c *Client) Config() *config.Config { return c.cfg }

// ReasoningField returns the detected reasoning field name.
func (c *Client) ReasoningField() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.reason == "" {
		return ReasoningFieldLlamaCPP
	}
	return c.reason
}

func (c *Client) setReasoningField(field string) {
	if field == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.reason == "" {
		c.reason = field
	}
}

// ChatURL returns the endpoint used for chat completions.
//
// llama.cpp and most servers expose /v1/chat/completions; GitHub Copilot uses
// /chat/completions; GitHub Models uses /inference/chat/completions.
func (c *Client) ChatURL() string {
	base := strings.TrimRight(c.cfg.APIEndpoint, "/")
	switch {
	case config.IsGitHubModelsEndpoint(base):
		if strings.HasSuffix(base, "/inference") {
			return base + "/chat/completions"
		}
		return base + "/inference/chat/completions"
	case config.IsCopilotEndpoint(base):
		if strings.HasSuffix(base, "/chat/completions") {
			return base
		}
		return base + "/chat/completions"
	default:
		if strings.HasSuffix(base, "/v1") || strings.HasSuffix(base, "/v1/chat/completions") {
			if strings.HasSuffix(base, "/chat/completions") {
				return base
			}
			return base + "/chat/completions"
		}
		return base + "/v1/chat/completions"
	}
}

// ModelsURL returns the model listing endpoint for the backend.
func (c *Client) ModelsURL() string {
	base := strings.TrimRight(c.cfg.APIEndpoint, "/")
	switch {
	case config.IsGitHubModelsEndpoint(base):
		if strings.HasSuffix(base, "/inference") {
			return strings.TrimSuffix(base, "/inference") + "/models"
		}
		return base + "/models"
	case config.IsCopilotEndpoint(base):
		return base + "/models"
	default:
		if strings.HasSuffix(base, "/v1") {
			return base + "/models"
		}
		return base + "/v1/models"
	}
}

// AuthHeaders returns the authentication headers for the backend.
func (c *Client) AuthHeaders(req *http.Request) {
	key := c.cfg.APIKey
	switch {
	case config.IsCopilotEndpoint(c.cfg.APIEndpoint):
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Copilot-Integration-Id", "vscode-chat")
		req.Header.Set("Editor-Version", "coding-agent/"+version.String())
	case config.IsGitHubModelsEndpoint(c.cfg.APIEndpoint):
		req.Header.Set("Authorization", "Bearer "+key)
	default:
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	req.Header.Set("Content-Type", "application/json")
}

// AvailableModels lists models advertised by the backend (best effort).
func (c *Client) AvailableModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ModelsURL(), nil)
	if err != nil {
		return nil, err
	}
	c.AuthHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// retryable reports whether an HTTP status should be retried.
func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

// ChatCompletion performs a chat completion, streaming when enabled.
func (c *Client) ChatCompletion(ctx context.Context, req *Request, cb *StreamCallbacks) (*Response, error) {
	if c.cfg.Stream {
		req.Stream = true
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}
	if dbg := debug.Default(); dbg != nil {
		dbg.Logf("request payload (%d bytes): %s", len(payload), debug.Redact(string(payload)))
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			if delay > 15*time.Second {
				delay = 15 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		resp, err := c.doOnce(ctx, payload, cb)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		var apiErr *APIError
		if errors.As(err, &apiErr) && retryable(apiErr.StatusCode) {
			if dbg := debug.Default(); dbg != nil {
				dbg.Logf("retryable status %d, attempt %d", apiErr.StatusCode, attempt+1)
			}
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("request failed after retries: %w", lastErr)
}

func (c *Client) doOnce(ctx context.Context, payload []byte, cb *StreamCallbacks) (*Response, error) {
	url := c.ChatURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	c.AuthHeaders(req)

	start := time.Now()
	httpResp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if c.cfg.InitialTokenTimeout > 0 && strings.Contains(err.Error(), "timeout awaiting response headers") {
			return nil, fmt.Errorf("timed out after %s waiting for the first response token (raise --initial-token-timeout or CODING_AGENT_INITIAL_TOKEN_TIMEOUT; long prompts may need a slow prompt cache warm-up)", formatDuration(time.Duration(c.cfg.InitialTokenTimeout)*time.Second))
		}
		return nil, &APIError{
			Message: fmt.Sprintf("failed to connect to %s: %v", url, err),
			Hint:    "check that the inference server is running and that --api-endpoint is correct",
		}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
		return nil, c.httpError(httpResp, body)
	}

	if !c.cfg.Stream {
		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("response (%d bytes, %s): %s", len(body), time.Since(start), debug.Redact(string(body)))
		}
		return c.parseNonStream(body)
	}
	return c.parseStream(ctx, httpResp, cb, start)
}

func (c *Client) httpError(resp *http.Response, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Message != "" {
		msg = parsed.Error.Message
	}
	if msg == "" {
		msg = resp.Status
	}
	err := &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("inference API error (HTTP %d): %s", resp.StatusCode, msg)}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		err.Hint = authHint(c.cfg)
	case resp.StatusCode == http.StatusNotFound:
		err.Hint = fmt.Sprintf("endpoint not found at %s; llama.cpp uses /v1/chat/completions, GitHub Copilot uses /chat/completions, GitHub Models uses /inference/chat/completions", c.ChatURL())
	case resp.StatusCode == http.StatusTooManyRequests:
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			err.Hint = "rate limited, Retry-After: " + ra
		} else {
			err.Hint = "rate limited by the server; retrying with backoff"
		}
	}
	return err
}

func authHint(cfg *config.Config) string {
	if config.IsCopilotEndpoint(cfg.APIEndpoint) {
		return "GitHub Copilot requires a GitHub App user token (ghu_/gho_). A personal access token (github_pat_/ghp_) is rejected. Run 'gh auth token' or 'gh auth login' and export GITHUB_TOKEN, or pass --api-key."
	}
	if config.IsGitHubModelsEndpoint(cfg.APIEndpoint) {
		return "GitHub Models requires a GitHub token with model inference access; export GITHUB_TOKEN or pass --api-key."
	}
	return "set CODING_AGENT_API_KEY or pass --api-key (llama.cpp servers usually need no key)"
}

func (c *Client) parseNonStream(body []byte) (*Response, error) {
	var cr struct {
		ID      string    `json:"id"`
		Model   string    `json:"model"`
		Choices []Choice  `json:"choices"`
		Usage   *Usage    `json:"usage"`
		Timings *Timings  `json:"timings"`
		Error   *APIError `json:"error"`
	}
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if cr.Error != nil {
		return nil, cr.Error
	}
	resp := &Response{ID: cr.ID, Model: cr.Model}
	for _, ch := range cr.Choices {
		if ch.Message == nil {
			continue
		}
		resp.Content = ch.Message.Text()
		resp.Reasoning = ch.Message.Reasoning
		resp.ToolCalls = ch.Message.ToolCalls
		resp.FinishReason = ch.FinishReason
		if ch.Message.ReasoningField != "" {
			c.setReasoningField(ch.Message.ReasoningField)
		}
		break
	}
	resp.Usage = usageFrom(cr.Usage, cr.Timings)
	return resp, nil
}

// usageFrom combines OpenAI usage with llama.cpp timings. llama.cpp reports
// cache_n (tokens served from the prompt cache) and prompt_n (tokens actually
// processed); the total prompt tokens is the sum of both.
func usageFrom(u *Usage, t *Timings) Usage {
	if u != nil && (u.PromptTokens > 0 || u.CompletionTokens > 0) {
		out := *u
		if out.TotalTokens == 0 {
			out.TotalTokens = out.PromptTokens + out.CompletionTokens
		}
		return out
	}
	if t != nil {
		prompt := t.CacheN + t.PromptN
		return Usage{
			PromptTokens:     prompt,
			CompletionTokens: t.PredictedN,
			TotalTokens:      prompt + t.PredictedN,
			CachedTokens:     t.CacheN,
		}
	}
	if u != nil {
		return *u
	}
	return Usage{}
}

// parseStream consumes an SSE stream and assembles the completion.
func (c *Client) parseStream(ctx context.Context, resp *http.Response, cb *StreamCallbacks, start time.Time) (*Response, error) {
	out := &Response{}
	toolCalls := map[int]*ToolCall{}
	toolCallOrder := []int{}

	var usage *Usage
	var timings *Timings

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var firstToken, timedOut bool
	var timeoutErr error
	var mu sync.Mutex
	var watchdog *time.Timer

	timeout := time.Duration(c.cfg.InitialTokenTimeout) * time.Second
	stopWatchdog := func() {
		mu.Lock()
		firstToken = true
		if watchdog != nil {
			watchdog.Stop()
		}
		mu.Unlock()
	}
	if timeout > 0 {
		watchdog = time.AfterFunc(timeout, func() {
			mu.Lock()
			if !firstToken {
				timedOut = true
				timeoutErr = fmt.Errorf("timed out after %s waiting for the first response token (raise --initial-token-timeout or CODING_AGENT_INITIAL_TOKEN_TIMEOUT; long prompts may need a slow prompt cache warm-up)", formatDuration(timeout))
			}
			mu.Unlock()
			_ = resp.Body.Close()
		})
	}

	// Unblock the blocking scanner read when the context is cancelled.
	go func() {
		<-ctx.Done()
		_ = resp.Body.Close()
	}()

	type scanResult struct {
		done bool
		err  error
	}
	done := make(chan scanResult, 1)

	go func() {
		for scanner.Scan() {
			stopWatchdog()
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				done <- scanResult{done: true}
				return
			}
			var chunk Chunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				if dbg := debug.Default(); dbg != nil {
					dbg.Logf("failed to decode SSE chunk: %v (%s)", err, debug.Redact(data))
				}
				continue
			}
			if chunk.Error != nil {
				done <- scanResult{err: chunk.Error}
				return
			}
			if chunk.Model != "" {
				out.Model = chunk.Model
			}
			if chunk.ID != "" {
				out.ID = chunk.ID
			}
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
			if chunk.Timings != nil {
				timings = chunk.Timings
			}
			for _, ch := range chunk.Choices {
				if ch.FinishReason != "" {
					out.FinishReason = ch.FinishReason
				}
				d := ch.Delta
				if d == nil {
					if ch.Message == nil {
						continue
					}
					d = &Delta{Content: strPtr(ch.Message.Text())}
					if ch.Message.Reasoning != "" {
						if ch.Message.ReasoningField == ReasoningFieldOpenAI {
							d.Reasoning = strPtr(ch.Message.Reasoning)
						} else {
							d.ReasoningContent = strPtr(ch.Message.Reasoning)
						}
					}
					for i, tc := range ch.Message.ToolCalls {
						fc := tc.Function
						d.ToolCalls = append(d.ToolCalls, ToolCallDelta{Index: i, ID: tc.ID, Type: tc.Type, Function: &fc})
					}
				}
				if d.Content != nil && *d.Content != "" {
					out.Content += *d.Content
					if cb != nil && cb.Content != nil {
						cb.Content(*d.Content)
					}
				}
				field, reasoning := pickReasoning(d)
				if reasoning != "" {
					out.Reasoning += reasoning
					c.setReasoningField(field)
					if cb != nil && cb.Reasoning != nil {
						cb.Reasoning(reasoning)
					}
				}
				for _, tcd := range d.ToolCalls {
					if tcd.Type != "" && tcd.Type != "function" {
						tcd.Type = "function"
					}
					tc, ok := toolCalls[tcd.Index]
					if !ok {
						tc = &ToolCall{Type: "function"}
						toolCalls[tcd.Index] = tc
						toolCallOrder = append(toolCallOrder, tcd.Index)
					}
					if tcd.ID != "" {
						tc.ID = tcd.ID
					}
					if tcd.Type != "" {
						tc.Type = tcd.Type
					}
					if tcd.Function != nil {
						tc.Function.Name += tcd.Function.Name
						tc.Function.Arguments += tcd.Function.Arguments
					}
					if cb != nil && cb.ToolCallDelta != nil {
						cb.ToolCallDelta(tcd.Index, tcd)
					}
				}
			}
		}
		done <- scanResult{err: scanner.Err()}
	}()

	res := <-done
	if watchdog != nil {
		watchdog.Stop()
	}
	mu.Lock()
	tErr := timeoutErr
	timed := timedOut
	mu.Unlock()
	if timed {
		return nil, tErr
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if res.err != nil {
		return nil, fmt.Errorf("failed reading stream: %w", res.err)
	}

	for _, idx := range toolCallOrder {
		tc := toolCalls[idx]
		if tc.ID == "" {
			tc.ID = fmt.Sprintf("call_%d", idx)
		}
		if tc.Type == "" {
			tc.Type = "function"
		}
		out.ToolCalls = append(out.ToolCalls, *tc)
	}
	out.Usage = usageFrom(usage, timings)
	if dbg := debug.Default(); dbg != nil {
		dbg.Logf("stream finished in %s: content=%d chars reasoning=%d chars tool_calls=%d usage=%d/%d tokens",
			time.Since(start), len(out.Content), len(out.Reasoning), len(out.ToolCalls), out.Usage.PromptTokens, out.Usage.CompletionTokens)
	}
	return out, nil
}
func pickReasoning(d *Delta) (field, text string) {
	if d.ReasoningContent != nil && *d.ReasoningContent != "" {
		return ReasoningFieldLlamaCPP, *d.ReasoningContent
	}
	if d.Reasoning != nil && *d.Reasoning != "" {
		return ReasoningFieldOpenAI, *d.Reasoning
	}
	return "", ""
}

func strPtr(s string) *string { return &s }

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.String()
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if m == 0 && s == 0 {
		return fmt.Sprintf("%dh", h)
	}
	if s == 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dh%dm%ds", h, m, s)
}

// ParseRetryAfterSeconds returns the Retry-After value in seconds, if present.
func ParseRetryAfterSeconds(resp *http.Response) (int, bool) {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return int(d.Seconds()), true
	}
	return 0, false
}
