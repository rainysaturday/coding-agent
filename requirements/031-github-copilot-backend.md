# Requirement 031: GitHub Copilot Backend Support

## Description

The coding agent harness must support GitHub Copilot as an inference backend. GitHub Copilot exposes an OpenAI-compatible chat completions API at `https://api.githubcopilot.com`. In practice, this endpoint requires a Copilot user token (`ghu_...`) and rejects PAT (`github_pat_...`) tokens. OAuth tokens such as `gho_...` may also fail for this endpoint depending on account and entitlement state. Since the existing inference client already speaks the OpenAI API format, supporting Copilot primarily requires correct endpoint and authentication configuration, model selection, and handling any Copilot-specific response behavior.

The harness should also document GitHub Models as the official, publicly documented GitHub inference API for direct integrations:

- Base URL: `https://models.github.ai`
- Path: `/inference/chat/completions`
- Authentication: `Authorization: Bearer <token>` with a token that has model access (for fine-grained tokens, `user_models=read`)

## Acceptance Criteria

- [ ] GitHub Copilot endpoint is configurable via `--api-endpoint https://api.githubcopilot.com`
- [ ] GitHub Copilot token is accepted via `GITHUB_TOKEN` environment variable
- [ ] GitHub Copilot token is accepted via `--api-key` CLI flag
- [ ] GitHub Copilot token is accepted via `api_key` config file field
- [ ] `--help` output explains how to configure GitHub Copilot
- [ ] Copilot-compatible models can be selected (e.g., `gpt-4o`, `claude-sonnet-4`, `o3-mini`)
- [ ] Requests include the required `Copilot-Integration-Id` header
- [ ] Requests include the required `Editor-Version` header
- [ ] Tool calling works correctly with Copilot-hosted models
- [ ] Streaming responses work correctly with Copilot-hosted models
- [ ] Non-streaming responses work correctly with Copilot-hosted models
- [ ] Streamed `tool_calls` deltas are merged by index into complete tool calls before being stored in message history
- [ ] Outgoing `messages[*].tool_calls[*].type` is always populated with a valid value (`function` by default)
- [ ] Token usage is parsed from Copilot API responses
- [ ] Connection errors to Copilot API produce clear error messages
- [ ] Authentication failures (401/403) produce clear error messages with guidance
- [ ] Rate limiting (429) is handled with appropriate backoff and retry
- [ ] Context size limits are respected per model

## Configuration

### Environment Variables

```bash
# Copilot endpoint
export CODING_AGENT_API_ENDPOINT="https://api.githubcopilot.com"

# Copilot user token (required by api.githubcopilot.com)
export GITHUB_TOKEN="ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
# Or use the standard API key variable
export CODING_AGENT_API_KEY="ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Model selection
export CODING_AGENT_MODEL="gpt-4o"

# If you only have PAT/OAuth tokens, use GitHub Models instead
export CODING_AGENT_API_ENDPOINT="https://models.github.ai"
export CODING_AGENT_API_KEY="$(gh auth token)"
export CODING_AGENT_MODEL="openai/gpt-4.1"
```

### Config File

```ini
api_endpoint=https://api.githubcopilot.com
api_key=ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
model=gpt-4o
```

### CLI Flags

```bash
# Full Copilot configuration via CLI
coding-agent \
  --api-endpoint https://api.githubcopilot.com \
  --api-key ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --model gpt-4o

# One-shot mode with Copilot
coding-agent -p "Create a REST API" \
  --api-endpoint https://api.githubcopilot.com \
  --api-key "$GITHUB_TOKEN" \
  --model claude-sonnet-4
```

### Help Output

The command help should include a short Copilot setup section and a GitHub Models setup section so users can discover both paths directly from the CLI:

```text
$ coding-agent --help
...
GitHub Copilot setup:
  export CODING_AGENT_API_ENDPOINT="https://api.githubcopilot.com"
  export GITHUB_TOKEN="ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  coding-agent --model gpt-4o

GitHub Models setup (official API):
  export CODING_AGENT_API_ENDPOINT="https://models.github.ai"
  export CODING_AGENT_API_KEY="github_pat_xxxxxxxxxxxxxxxxxxxx"
  coding-agent --model openai/gpt-4.1
```

## Implementation Details

### Token Resolution Order

When connecting to a Copilot endpoint, the token is resolved in this order (highest priority first):

1. `--api-key` CLI flag
2. `CODING_AGENT_API_KEY` environment variable
3. `GITHUB_TOKEN` environment variable
4. `api_key` field in config file

Because `CODING_AGENT_API_KEY` has higher priority than `GITHUB_TOKEN`, a PAT in `CODING_AGENT_API_KEY` will override a valid Copilot token in `GITHUB_TOKEN`. Troubleshooting should explicitly include checking for overrides and unsetting unintended variables.

The `GITHUB_TOKEN` environment variable must be checked as a fallback for the API key when the endpoint contains `githubcopilot.com`. This allows users who already have `GITHUB_TOKEN` set in their environment to use Copilot without additional configuration.

```go
func loadEnv(cfg *Config) {
    // ... existing env loading ...

    // Fallback: use GITHUB_TOKEN if API key is not set and endpoint is Copilot
    if cfg.APIKey == "" {
        if val := os.Getenv("GITHUB_TOKEN"); val != "" {
            cfg.APIKey = val
        }
    }
}
```

### Copilot-Specific Headers

When the API endpoint contains `githubcopilot.com`, the inference client must include additional HTTP headers required by the Copilot API:

```go
req.Header.Set("Content-Type", "application/json")
if ic.apiKey != "" {
    req.Header.Set("Authorization", "Bearer "+ic.apiKey)
}

// Copilot-specific headers
if strings.Contains(ic.endpoint, "githubcopilot.com") {
    req.Header.Set("Copilot-Integration-Id", "coding-agent")
    req.Header.Set("Editor-Version", "coding-agent/1.0")
}
```

### Endpoint Detection

The inference client should detect when a Copilot endpoint is configured and adjust behavior accordingly:

```go
func (ic *InferenceClient) isCopilotEndpoint() bool {
    return strings.Contains(ic.endpoint, "githubcopilot.com")
}
```

### API Compatibility

The Copilot API uses the same OpenAI chat completions format that the harness already supports:

**Request endpoint:** `POST https://api.githubcopilot.com/chat/completions`

For GitHub Models, use:

**Request endpoint:** `POST https://models.github.ai/inference/chat/completions`

**Request body (identical to existing format):**
```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."}
  ],
  "stream": true,
  "temperature": 0.7,
  "max_tokens": 4096,
  "tools": [...],
  "tool_choice": "auto"
}
```

**Response format:** Standard OpenAI chat completion response (same parsing as existing implementation).

**Streaming format:** Standard SSE `data: {...}` lines with `data: [DONE]` terminator (same as existing implementation).

### Streaming Tool Call Assembly

When using streaming, model responses may deliver `tool_calls` in incremental deltas where fields such as `type`, `function.name`, and `function.arguments` arrive across multiple chunks. The implementation must:

1. Merge tool call deltas by `index` into a single complete tool call record.
2. Concatenate incremental `function.arguments` chunks in order.
3. Default missing/empty `tool_calls[*].type` to `function` before storing or sending history.
4. Preserve only the latest merged tool call snapshot in agent state (do not append every partial delta as a new tool call entry).

This prevents invalid request payloads such as:

```
Invalid value: ''. Supported values are: 'function', 'allowed_tools', and 'custom'.
param: messages[...].tool_calls[...].type
```

### Request Path

The existing inference client appends `/v1/chat/completions` to the endpoint. The Copilot API expects requests at `/chat/completions` (no `/v1` prefix), and GitHub Models expects `/inference/chat/completions`. The endpoint path construction must account for this:

```go
func (ic *InferenceClient) buildURL() string {
    if ic.isCopilotEndpoint() {
        return ic.endpoint + "/chat/completions"
    }
  if ic.isGitHubModelsEndpoint() {
    return ic.endpoint + "/inference/chat/completions"
  }
    return ic.endpoint + "/v1/chat/completions"
}
```

### Error Handling

#### Authentication Errors (401/403)

```
Error: GitHub Copilot authentication failed (HTTP 401)
Ensure your GITHUB_TOKEN or --api-key is a valid GitHub Copilot token.
Generate one at: https://github.com/settings/tokens
```

For invalid token type at Copilot endpoint (observed in production):

```
Error: inference failed: 400 Bad Request: checking third-party user token: bad request: Personal Access Tokens are not supported for this endpoint
hint: api.githubcopilot.com does not accept Personal Access Tokens (github_pat). Use a Copilot user token (ghu_) for this endpoint. Alternatively switch to CODING_AGENT_API_ENDPOINT=https://models.github.ai when using PAT/OAuth tokens
```

And for OAuth tokens (for example `gho_...`) that are not accepted by Copilot endpoint:

```
Error: inference failed: 403 Forbidden: Access to this endpoint is forbidden.
hint: api.githubcopilot.com requires a Copilot user token (ghu_). If you only have PAT/OAuth tokens, use CODING_AGENT_API_ENDPOINT=https://models.github.ai
```

#### Rate Limiting (429)

The existing retry logic should handle 429 responses. Copilot rate limit headers should be respected:

```go
if resp.StatusCode == 429 {
    retryAfter := resp.Header.Get("Retry-After")
    // Use Retry-After header if present, otherwise use exponential backoff
}
```

The existing retry logic only retries on 5xx errors. It must be extended to also retry on 429 status codes.

#### Model Not Available

```
Error: Model "gpt-4-turbo" is not available on GitHub Copilot
Available models: gpt-4o, claude-sonnet-4, o3-mini
```

### Available Copilot Models

Common models accessible through the Copilot API (subject to change):

| Model | Context Size | Notes |
|-------|-------------|-------|
| `gpt-4o` | 128000 | Default, general purpose |
| `gpt-4o-mini` | 128000 | Faster, lower cost |
| `claude-sonnet-4` | 200000 | Strong coding performance |
| `o3-mini` | 128000 | Reasoning model |

## Usage Examples

### Example 1: Interactive Mode with Copilot

```bash
export CODING_AGENT_API_ENDPOINT="https://api.githubcopilot.com"
export GITHUB_TOKEN="ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export CODING_AGENT_MODEL="gpt-4o"

coding-agent
```

### Example 2: One-Shot Mode with Copilot

```bash
coding-agent -p "Refactor utils.go to use generics" \
  --api-endpoint https://api.githubcopilot.com \
  --api-key "$GITHUB_TOKEN" \
  --model claude-sonnet-4
```

### Example 3: Config File

```ini
# copilot-config.txt
api_endpoint=https://api.githubcopilot.com
api_key=ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
model=gpt-4o
context_size=128000
max_iterations=1000
```

```bash
coding-agent --config copilot-config.txt
```

### Example 4: Sub-Agent Spawning with Copilot

Since the agent executable path and environment are included in the system prompt (see requirement 029), sub-agents spawned via `coding-agent -p` will inherit the parent's environment variables including `GITHUB_TOKEN` and `CODING_AGENT_API_ENDPOINT`.

```bash
# Parent agent's environment carries over to sub-agents
export CODING_AGENT_API_ENDPOINT="https://api.githubcopilot.com"
export GITHUB_TOKEN="ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export CODING_AGENT_MODEL="gpt-4o"

coding-agent -p "Parallelize the test suite across 3 sub-agents"
```

### Example 5: GitHub Models with PAT/OAuth Token

```bash
export CODING_AGENT_API_ENDPOINT="https://models.github.ai"
export CODING_AGENT_API_KEY="$(gh auth token)"  # often gho_..., valid for models endpoint

coding-agent -p "Create a REST API" --model openai/gpt-4.1
```

## Testing

### Unit Tests

- [ ] `GITHUB_TOKEN` fallback sets `APIKey` when `CODING_AGENT_API_KEY` is not set
- [ ] `GITHUB_TOKEN` does NOT override `CODING_AGENT_API_KEY` when both are set
- [ ] `isCopilotEndpoint()` returns `true` for `https://api.githubcopilot.com`
- [ ] `isCopilotEndpoint()` returns `false` for `http://localhost:8080`
- [ ] `buildURL()` returns `/chat/completions` for Copilot endpoint
- [ ] `buildURL()` returns `/v1/chat/completions` for non-Copilot endpoint
- [ ] Copilot-specific headers are set when endpoint is Copilot
- [ ] Copilot-specific headers are NOT set for non-Copilot endpoints
- [ ] 429 responses trigger retry with backoff
- [ ] 401/403 responses produce helpful error messages
- [ ] PAT-on-Copilot 400 errors produce explicit guidance to use `ghu_` or switch to `models.github.ai`
- [ ] Non-429 4xx responses do not trigger retry loops
- [ ] Streaming tool-call delta merge correctly combines partial chunks by index
- [ ] Empty streamed tool-call `type` values are normalized to `function`
- [ ] Agent stores merged streaming tool-calls as the latest snapshot rather than appending partial entries

### Integration Tests

- [ ] End-to-end tool calling works against Copilot API
- [ ] Streaming works against Copilot API
- [ ] Non-streaming works against Copilot API
- [ ] Prompt that triggers tool use (for example, creating a file) succeeds without `messages[*].tool_calls[*].type` validation errors
- [ ] Context compression works with Copilot-hosted models

## Security Considerations

- GitHub tokens must never be logged, even in debug mode (existing `redactSensitiveData` in debug logger already handles `ghu_` prefix patterns)
- Tokens should only be transmitted over HTTPS
- The `GITHUB_TOKEN` fallback should only activate when the endpoint is a Copilot URL, to avoid accidentally sending GitHub tokens to arbitrary servers

## Performance Considerations

- Copilot API may have higher latency than local llama.cpp; the default 2-hour initial token timeout (requirement 010) accommodates this
- Rate limiting may require longer backoff periods than local inference
- Context size should match the selected model's capabilities

## Related Requirements

- **007-inference-backend.md**: Base inference backend configuration
- **008-context-size.md**: Context size management (varies by Copilot model)
- **010-streaming-inference.md**: Streaming support (same SSE format)
- **014-tool-calling-format.md**: Tool calling format (same OpenAI format)
- **024-zero-external-dependencies.md**: Use stdlib only for HTTP and auth
- **025-non-interactive-one-shot-mode.md**: One-shot mode works with Copilot
- **028-debug-flag.md**: Debug logging with token redaction
- **029-system-prompt-environment-info.md**: Sub-agent environment inheritance

## Summary Checklist

- [ ] `GITHUB_TOKEN` fallback for authentication
- [ ] Help output includes Copilot and GitHub Models setup guidance
- [ ] Copilot-specific HTTP headers (`Copilot-Integration-Id`, `Editor-Version`)
- [ ] Correct request paths (`/chat/completions` for Copilot, `/inference/chat/completions` for GitHub Models, `/v1/chat/completions` default)
- [ ] 429 rate limit retry support
- [ ] Clear authentication error messages
- [ ] Token-type guidance is documented (`ghu_` for Copilot endpoint, PAT/OAuth for GitHub Models endpoint)
- [ ] Token redaction in debug logs
- [ ] Works in interactive and one-shot modes
- [ ] Works with streaming and non-streaming
- [ ] Tool calling works with Copilot-hosted models
- [ ] Streaming tool-call assembly is robust (merge by index, normalize `type=function`, avoid partial-entry accumulation)
- [ ] Documentation updated
