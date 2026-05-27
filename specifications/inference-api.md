# Inference API Specification

## Overview

The inference API provides a chat completion endpoint compatible with the OpenAI API format (and llama.cpp). It supports both streaming and non-streaming modes for generating responses from LLM models.

## Base URL

```
http://localhost:8080/v1/chat/completions
```

For GitHub Models endpoints:

```
https://models.github.ai/inference/chat/completions
```

## Authentication

Most deployments require an API key. Include it in the request header:

```bash
-H "Authorization: Bearer YOUR_API_KEY"
```

## Endpoints

### Chat Completions

Generate a chat completion response.

**Endpoint:** `POST /v1/chat/completions`

---

## Request Body

### Type Definitions

```typescript
interface ChatCompletionRequest {
  model: string; // Model name (e.g., "llama", "gemma")
  messages: Message[]; // Conversation history
  stream?: boolean; // Enable streaming mode (default: false)
  temperature?: number; // Sampling temperature (optional)
  max_tokens?: number; // Maximum tokens to generate
  tools?: ToolDefinition[]; // Available tools for tool calling (optional)
  tool_choice?: string; // Tool choice strategy: "auto" | "none" | "required" (optional)
}

interface Message {
  role: "system" | "user" | "assistant" | "tool";
  content: string | ContentPart[]; // Plain text or multi-modal parts (see Image/Vision Support)
  tool_call_id?: string; // For tool result messages
  tool_calls?: ToolCall[]; // For assistant messages with tool calls
}

interface ContentPart {
  type: "text" | "image_url";
}

interface TextContentPart extends ContentPart {
  type: "text";
  text: string;
}

interface ImageContentPart extends ContentPart {
  type: "image_url";
  image_url: {
    url: string; // Public URL or base64 data URI (data:[mime_type];base64,[base64_string])
    detail?: "auto" | "low" | "high"; // Image detail level (affects token cost)
  };
}

interface ToolCall {
  id: string;
  type: string;
  function: {
    name: string;
    arguments: string; // JSON string of parameters
  };
}

interface ToolDefinition {
  type: "function";
  function: {
    name: string;
    description: string;
    parameters: {
      type: "object";
      properties: Record<string, Property>;
      required?: string[];
    };
  };
}

interface Property {
  type: string;
  description: string;
  items?: Property; // For array types
}
```

### Example Request

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is 2+2?"}
    ],
    "max_tokens": 50,
    "stream": false
  }'
```

---

## Response (Non-Streaming Mode)

When `stream: false` (default), the API returns a single complete response.

### Response Type

```typescript
interface ChatCompletionResponse {
  id: string; // Unique request ID
  object: string; // Always "chat.completion"
  created: number; // Unix timestamp
  model: string; // Model name
  choices: Choice[];
  usage: Usage;
  system_fingerprint?: string; // Backend fingerprint (optional)
}

interface Choice {
  index: number;
  finish_reason: string; // "stop" | "length" | "tool_calls"
  message: AssistantMessage;
}

interface AssistantMessage {
  role: "assistant";
  content: string; // Response text
  reasoning?: string; // Reasoning content (OpenAI standard field for reasoning models like o1, o3-mini)
  reasoning_content?: string; // Alternative field name used by llama.cpp for reasoning/thinking content
  tool_calls?: ToolCall[]; // Tool calls made
}

interface Usage {
  prompt_tokens: number; // Total tokens in prompt
  completion_tokens: number; // Tokens generated
  total_tokens: number; // prompt_tokens + completion_tokens
  prompt_tokens_details?: {
    cached_tokens: number; // Tokens served from cache (optional)
  };
}
```

### Example Response

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1777042655,
  "model": "unsloth/Qwen3.6-35B-A3B-GGUF:Q8_0",
  "choices": [
    {
      "finish_reason": "length",
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "2+2 equals 4.",
        "reasoning_content": "Here's a thinking process:\n\n1. Analyze User Input: The user asks \"What is 2+2?\"\n2. Calculate: 2 + 2 = 4"
      }
    }
  ],
  "usage": {
    "prompt_tokens": 17,
    "completion_tokens": 50,
    "total_tokens": 67,
    "prompt_tokens_details": {
      "cached_tokens": 0
    }
  },
  "system_fingerprint": "b8701-66c4f9ded"
}
```

### curl Example (Non-Streaming)

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama",
    "messages": [
      {"role": "user", "content": "What is 2+2?"}
    ],
    "max_tokens": 50,
    "stream": false
  }' | python3 -m json.tool
```

---

## Response (Streaming Mode)

When `stream: true`, the API returns a stream of Server-Sent Events (SSE) with incremental chunks.

### Streaming Response Type

```typescript
interface StreamChunk {
  id: string; // Unique request ID, consistent across all chunks
  object: string; // Always "chat.completion.chunk"
  created: number; // Unix timestamp (seconds) when the request was created
  model: string; // Model name that processed the request
  choices: StreamChoice[]; // Array of completion choices (usually 1 element)
  timings?: Timings; // Token counts and timing info (only on final chunk)
  usage?: Usage; // Token counts (only on final chunk, OpenAI format)
}

interface StreamChoice {
  index: number; // Index of this choice (0-based, usually 0)
  finish_reason: string | null; // Reason for finishing, null while streaming
  delta: Delta; // The delta/content for this chunk
}

interface Delta {
  role?: string; // Message role ("assistant"), only in first chunk
  content: string | null; // Text content fragment, null if only tool call
  reasoning?: string | null; // Reasoning content fragment (OpenAI standard field)
  reasoning_content?: string | null; // Alternative field name used by llama.cpp for reasoning content
  tool_calls?: ToolCallDelta[]; // Tool call deltas (if model uses tools)
}

interface ToolCallDelta {
  index: number; // Index of the tool call being built (0-based)
  id?: string; // Tool call ID, only present in first chunk of that call
  type?: string; // Tool call type ("function"), only in first chunk
  function?: FunctionDelta; // Function call delta containing name and arguments
}

interface FunctionDelta {
  name?: string; // Function name, only in first chunk
  arguments: string; // Incremental JSON argument string (accumulated)
}

interface Timings {
  cache_n: number; // Number of prompt tokens served from cache (NOT included in prompt_n)
  prompt_n: number; // Number of prompt tokens actually processed (computed)
  prompt_ms: number; // Time spent processing prompt (milliseconds)
  prompt_per_token_ms: number; // Average time per prompt token (milliseconds)
  prompt_per_second: number; // Prompt tokens processed per second
  predicted_n: number; // Number of tokens predicted/generated
  predicted_ms: number; // Time spent generating tokens (milliseconds)
  predicted_per_token_ms: number; // Average time per generated token (milliseconds)
  predicted_per_second: number; // Generated tokens per second
}
```

**Important**: In llama.cpp streaming mode, `cache_n` and `prompt_n` are SEPARATE counts. To get the total prompt tokens in the context window, you need to add them together: `total_prompt_tokens = cache_n + prompt_n`.

For example, if `cache_n` is 10 and `prompt_n` is 17, the total prompt tokens are 27 (not 17).

```

### Streaming Response Format

Each chunk is sent as a Server-Sent Event with the prefix `data: `:

```

data: <JSON object>
data: <JSON object>
...
data: [DONE]

````

The stream ends with `data: [DONE]`.

### Complete Example: First Chunk (Empty Content)

The first chunk typically announces the assistant role with no content:

```json
{
  "choices": [
    {
      "finish_reason": null,
      "index": 0,
      "delta": {
        "role": "assistant",
        "content": null
      }
    }
  ],
  "created": 1777047505,
  "id": "chatcmpl-m18aDztzZLeYqv4w0aTcGFwuwhy4pQfI",
  "model": "unsloth/Qwen3.6-35B-A3B-GGUF:Q8_0",
  "system_fingerprint": "b8701-66c4f9ded",
  "object": "chat.completion.chunk"
}
````

**Property descriptions:**

| Property                   | Value                     | Description                                        |
| -------------------------- | ------------------------- | -------------------------------------------------- |
| `choices[0].finish_reason` | `null`                    | Not yet finished, still streaming                  |
| `choices[0].index`         | `0`                       | First (and usually only) choice                    |
| `choices[0].delta.role`    | `"assistant"`             | Indicates this is an assistant response            |
| `choices[0].delta.content` | `null`                    | No text content in this first chunk                |
| `created`                  | `1777047505`              | Unix timestamp when the request was created        |
| `id`                       | `"chatcmpl-..."`          | Unique identifier for this chat completion request |
| `model`                    | `"unsloth/..."`           | The model name that processed the request          |
| `system_fingerprint`       | `"b8701-..."`             | Backend system fingerprint (varies by deployment)  |
| `object`                   | `"chat.completion.chunk"` | Always this value for streaming chunks             |

### Complete Example: Content Chunk (Normal Text)

A chunk containing text content:

```json
{
  "choices": [
    {
      "finish_reason": null,
      "index": 0,
      "delta": {
        "content": "Here"
      }
    }
  ],
  "created": 1777047505,
  "id": "chatcmpl-m18aDztzZLeYqv4w0aTcGFwuwhy4pQfI",
  "model": "unsloth/Qwen3.6-35B-A3B-GGUF:Q8_0",
  "system_fingerprint": "b8701-66c4f9ded",
  "object": "chat.completion.chunk"
}
```

**Property descriptions:**

| Property                   | Value               | Description                   |
| -------------------------- | ------------------- | ----------------------------- |
| `choices[0].finish_reason` | `null`              | Still streaming, not finished |
| `choices[0].delta.content` | `"Here"`            | Text fragment of the response |
| All other properties       | Same as first chunk | Unchanged from previous chunk |

### Complete Example: Reasoning Content Chunk

A chunk containing reasoning/thinking content (for reasoning models):

```json
{
  "choices": [
    {
      "finish_reason": null,
      "index": 0,
      "delta": {
        "reasoning_content": "1"
      }
    }
  ],
  "created": 1777047506,
  "id": "chatcmpl-m18aDztzZLeYqv4w0aTcGFwuwhy4pQfI",
  "model": "unsloth/Qwen3.6-35B-A3B-GGUF:Q8_0",
  "system_fingerprint": "b8701-66c4f9ded",
  "object": "chat.completion.chunk"
}
```

**Property descriptions:**

| Property                             | Value           | Description                            |
| ------------------------------------ | --------------- | -------------------------------------- |
| `choices[0].delta.reasoning_content` | `"1"`           | Fragment of reasoning/thinking content |
| `choices[0].delta.content`           | _(not present)_ | No normal text content in this chunk   |

### Complete Example: Final Chunk (with Timings)

The last chunk before `[DONE]` contains the finish reason and performance timings:

```json
{
  "choices": [
    {
      "finish_reason": "length",
      "index": 0,
      "delta": {}
    }
  ],
  "created": 1777047507,
  "id": "chatcmpl-m18aDztzZLeYqv4w0aTcGFwuwhy4pQfI",
  "model": "unsloth/Qwen3.6-35B-A3B-GGUF:Q8_0",
  "system_fingerprint": "b8701-66c4f9ded",
  "object": "chat.completion.chunk",
  "timings": {
    "cache_n": 0,
    "prompt_n": 17,
    "prompt_ms": 246.713,
    "prompt_per_token_ms": 14.512529411764705,
    "prompt_per_second": 68.90597576941629,
    "predicted_n": 50,
    "predicted_ms": 1795.018,
    "predicted_per_token_ms": 35.90036,
    "predicted_per_second": 27.85487387870205
  }
}
```

**Property descriptions:**

| Property                         | Value      | Description                                 |
| -------------------------------- | ---------- | ------------------------------------------- |
| `choices[0].finish_reason`       | `"length"` | Response ended due to reaching `max_tokens` |
| `choices[0].delta`               | `{}`       | Empty delta on final chunk                  |
| `timings.cache_n`                | `0`        | Number of prompt tokens served from cache   |
| `timings.prompt_n`               | `17`       | Total prompt tokens processed               |
| `timings.prompt_ms`              | `246.713`  | Total time spent processing the prompt (ms) |
| `timings.prompt_per_token_ms`    | `14.51`    | Average time per prompt token (ms)          |
| `timings.prompt_per_second`      | `68.91`    | Prompt throughput (tokens/second)           |
| `timings.predicted_n`            | `50`       | Number of tokens generated/predicted        |
| `timings.predicted_ms`           | `1795.018` | Total time spent generating tokens (ms)     |
| `timings.predicted_per_token_ms` | `35.90`    | Average time per generated token (ms)       |
| `timings.predicted_per_second`   | `27.85`    | Generation throughput (tokens/second)       |

### Complete Example: Final Chunk (with OpenAI-style Usage)

Some backends provide usage information in OpenAI format instead of timings:

```json
{
  "choices": [
    {
      "finish_reason": "stop",
      "index": 0,
      "delta": {}
    }
  ],
  "created": 1777047507,
  "id": "chatcmpl-xyz789",
  "model": "unsloth/Qwen3.6-35B-A3B-GGUF:Q8_0",
  "object": "chat.completion.chunk",
  "usage": {
    "prompt_tokens": 17,
    "completion_tokens": 50,
    "total_tokens": 67,
    "prompt_tokens_details": {
      "cached_tokens": 0
    }
  }
}
```

**Property descriptions:**

| Property                                    | Value    | Description                              |
| ------------------------------------------- | -------- | ---------------------------------------- |
| `choices[0].finish_reason`                  | `"stop"` | Response ended naturally (not truncated) |
| `usage.prompt_tokens`                       | `17`     | Total tokens sent to the model           |
| `usage.completion_tokens`                   | `50`     | Tokens generated by the model            |
| `usage.total_tokens`                        | `67`     | Sum of prompt and completion tokens      |
| `usage.prompt_tokens_details.cached_tokens` | `0`      | Tokens served from cache                 |

### Streaming Response Format

Each chunk is sent as a Server-Sent Event with the prefix `data: `:

```
data: {"choices":[...],"created":...,"id":"...","model":"...","object":"chat.completion.chunk",...}
data: {"choices":[...],...}
...
data: [DONE]
```

The stream ends with `data: [DONE]`.

### Example Streaming Response

```
data: {"choices":[{"finish_reason":null,"index":0,"delta":{"role":"assistant","content":null}}],...}

data: {"choices":[{"finish_reason":null,"index":0,"delta":{"content":"Hello"}}],...}

data: {"choices":[{"finish_reason":null,"index":0,"delta":{"content":" world"}}],...}

data: {"choices":[{"finish_reason":"stop","index":0,"delta":{}}],"timings":{"prompt_n":17,"predicted_n":10,...}}

data: [DONE]
```

### curl Example (Streaming)

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama",
    "messages": [
      {"role": "user", "content": "Count from 1 to 5"}
    ],
    "max_tokens": 50,
    "stream": true
  }'
```

Or with pretty printing:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama",
    "messages": [
      {"role": "user", "content": "Count from 1 to 5"}
    ],
    "max_tokens": 50,
    "stream": true
  }' 2>&1 | sed 's/^data: //; s/\r$//' | python3 -c "
import sys, json
for line in sys.stdin:
    if line.strip() == '[DONE]':
        print()
        break
    try:
        chunk = json.loads(line)
        if chunk['choices'][0]['delta'].get('content'):
            print(chunk['choices'][0]['delta']['content'], end='', flush=True)
    except:
        pass
"
```

---

## Key Fields Explained

### Token Counts

**Important:** The API returns accurate token counts that include the FULL context window usage, regardless of caching.

- `prompt_tokens`: Total tokens sent to the model (system prompt + all messages + tools)
- `completion_tokens`: Tokens generated by the model
- `total_tokens`: `prompt_tokens + completion_tokens` (the complete count)
- `cached_tokens`: Number of prompt tokens served from cache (metadata only, NOT deducted)

Example:

```json
{
  "prompt_tokens": 17,
  "completion_tokens": 50,
  "total_tokens": 67,
  "prompt_tokens_details": {
    "cached_tokens": 0
  }
}
```

Here:

- 17 tokens were sent as the prompt (fully accurate)
- 50 tokens were generated
- 67 total tokens processed
- 0 of the 17 prompt tokens were from cache

Even if `cached_tokens` is 10 (meaning 10 of the 17 prompt tokens were cached), `prompt_tokens` is still **17**, not 7. The `cached_tokens` field is purely informational.

### Streaming Token Counts

In streaming mode, token counts only appear in the **final chunk** (when `finish_reason` is set). They are provided in two formats:

1. **OpenAI format** (if available):

   ```json
   {
     "usage": {
       "prompt_tokens": 17,
       "completion_tokens": 50,
       "total_tokens": 67
     }
   }
   ```

2. **llama.cpp timings format** (common for local deployments):
   ```json
   { "timings": { "prompt_n": 17, "predicted_n": 50 } }
   ```

The code falls back to timings if usage is not available:

```typescript
if (chunk.Usage.TotalTokens > 0) {
  // Use OpenAI format
  totalTokens = chunk.Usage.TotalTokens;
} else if (chunk.Timings.PredictedN > 0) {
  // Use llama.cpp timings format
  totalTokens = chunk.Timings.PromptN + chunk.Timings.PredictedN;
}
```

### Getting Total Token Usage from Streaming Responses

In streaming mode, token counts are **not available until the stream completes**. The final chunk (the last chunk before `[DONE]`) contains the token information. Here's how to extract it:

#### Method 1: Using OpenAI-style Usage (Recommended)

Some backends provide usage information in the `usage` field of the final chunk:

```json
{
  "choices": [
    {
      "finish_reason": "stop",
      "index": 0,
      "delta": {}
    }
  ],
  "usage": {
    "prompt_tokens": 17,
    "completion_tokens": 50,
    "total_tokens": 67
  }
}
```

**To get `total_tokens`:**

```typescript
// In your streaming callback, check if usage is present
if (chunk.usage && chunk.usage.total_tokens > 0) {
  const totalTokens = chunk.usage.total_tokens;
  console.log(`Total tokens: ${totalTokens}`);
  // totalTokens = prompt_tokens (17) + completion_tokens (50) = 67
}
```

**To get individual components:**

```typescript
if (chunk.usage) {
  const promptTokens = chunk.usage.prompt_tokens; // 17
  const completionTokens = chunk.usage.completion_tokens; // 50
  const totalTokens = chunk.usage.total_tokens; // 67

  // total_tokens should equal prompt_tokens + completion_tokens
  // This is the authoritative count of everything the API processed
}
```

#### Method 2: Using llama.cpp Timings (Fallback)

Some backends (especially local deployments) provide timing information instead of usage. The timings object includes a `cache_n` field that indicates how many prompt tokens were served from cache. **Important: In llama.cpp streaming mode, `cache_n` is NOT included in `prompt_n`** - they are separate counts:

```json
{
  "choices": [
    {
      "finish_reason": "length",
      "index": 0,
      "delta": {}
    }
  ],
  "timings": {
    "cache_n": 10,
    "prompt_n": 17,
    "predicted_n": 50
  }
}
```

**To calculate `total_tokens` from timings:**

```typescript
// In your streaming callback, check if timings is present
if (chunk.timings) {
  const cacheTokens = chunk.timings.cache_n || 0; // 10 (cached tokens)
  const promptTokens = chunk.timings.prompt_n; // 17 (processed prompt tokens)
  const completionTokens = chunk.timings.predicted_n; // 50 (generated tokens)

  // total_tokens = cached_tokens + prompt_tokens + generated_tokens
  // All three values are ADDITIVE - they represent different parts of the total
  const totalTokens = cacheTokens + promptTokens + completionTokens; // 77

  console.log(`Total tokens: ${totalTokens}`);
  // Note: total_tokens = cache_n (10) + prompt_n (17) + predicted_n (50) = 77
}
```

**Understanding the relationship:**

- `cache_n` = Number of prompt tokens served from cache (NOT processed, just looked up)
- `prompt_n` = Number of prompt tokens actually processed (computed)
- `predicted_n` = Number of tokens generated/predicted
- `total_tokens` = `cache_n + prompt_n + predicted_n`

For example, if `cache_n` is 10, `prompt_n` is 17, and `predicted_n` is 50:

- 10 tokens were served from cache (fast lookup)
- 17 tokens were processed from the prompt (slower computation)
- 50 tokens were generated
- Total tokens = 10 + 17 + 50 = **77**

#### Method 3: Complete Implementation Example

Here's a complete implementation that handles both formats:

```typescript
function handleStreamingResponse(stream: Readable) {
  let totalTokens = 0;

  stream.on("data", (chunk: string) => {
    // Parse SSE data
    const lines = chunk.split("\n");
    for (const line of lines) {
      if (!line.startsWith("data: ")) continue;

      const data = line.slice(6);
      if (data === "[DONE]") {
        console.log(`Stream complete. Total tokens: ${totalTokens}`);
        return;
      }

      try {
        const parsed = JSON.parse(data);

        // Check if this is the final chunk with token counts
        if (parsed.usage && parsed.usage.total_tokens > 0) {
          // OpenAI format - direct access to total_tokens
          totalTokens = parsed.usage.total_tokens;
          console.log(`Total tokens (OpenAI format): ${totalTokens}`);
        } else if (parsed.timings && parsed.timings.predicted_n > 0) {
          // llama.cpp format - calculate from timings
          // Important: cache_n is NOT included in prompt_n, so we add all three
          totalTokens =
            parsed.timings.cache_n +
            parsed.timings.prompt_n +
            parsed.timings.predicted_n;
          console.log(`Total tokens (timings format): ${totalTokens}`);
        }
      } catch (e) {
        // Not valid JSON, skip
      }
    }
  });
}
```

**Python equivalent:**

```python
import json
import requests

def handle_streaming_response(response: requests.Response):
    total_tokens = 0

    for line in response.iter_lines():
        if line.startswith(b'data: '):
            data = line[6:].decode('utf-8')
            if data == '[DONE]':
                print(f"Stream complete. Total tokens: {total_tokens}")
                break

            try:
                parsed = json.loads(data)

                # Check for OpenAI usage format
                if 'usage' in parsed and parsed['usage'].get('total_tokens', 0) > 0:
                    total_tokens = parsed['usage']['total_tokens']
                    print(f"Total tokens: {total_tokens}")

                # Check for llama.cpp timings format
                elif 'timings' in parsed and parsed['timings'].get('predicted_n', 0) > 0:
                    cache_n = parsed['timings'].get('cache_n', 0)
                    prompt_n = parsed['timings']['prompt_n']
                    predicted_n = parsed['timings']['predicted_n']
                    total_tokens = cache_n + prompt_n + predicted_n
                    print(f"Total tokens: {total_tokens}")
            except json.JSONDecodeError:
                pass
```

#### Key Points

1. **Token counts only appear in the final chunk**: You won't receive token counts during the streaming process. They only appear in the last chunk before `[DONE]`.

2. **Two formats exist**:

   - OpenAI format: `usage.total_tokens` (direct access)
   - llama.cpp format: `timings.prompt_n + timings.predicted_n` (calculate)

3. **Check both formats**: Always check for `usage` first, then fall back to `timings`:

   ```typescript
   if (chunk.usage && chunk.usage.total_tokens > 0) {
     totalTokens = chunk.usage.total_tokens;
   } else if (chunk.timings && chunk.timings.predicted_n > 0) {
     totalTokens =
       chunk.timings.cache_n +
       chunk.timings.prompt_n +
       chunk.timings.predicted_n;
   }
   ```

4. **`total_tokens` is authoritative**: This number represents the exact count of everything the API processed (system prompt + all messages + tools + completion). It includes cached tokens in the count.

5. **Timing information**: The `timings` object also provides performance metrics:

   - `prompt_n`: Number of prompt tokens
   - `predicted_n`: Number of generated tokens
   - `prompt_ms`: Time spent processing prompt (ms)
   - `predicted_ms`: Time spent generating tokens (ms)

6. **Example curl command to extract token counts**:
   ```bash
   curl -s http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "llama",
       "messages": [{"role": "user", "content": "What is 2+2?"}],
       "max_tokens": 50,
       "stream": true
     }' | sed 's/^data: //; s/\r$//' | python3 -c "
   import sys, json
   for line in sys.stdin:
    if line.strip() == '[DONE]':
        break
    try:
        chunk = json.loads(line)
        if 'usage' in chunk:
            print(f\"Total tokens: {chunk['usage']['total_tokens']}\")
            break
        elif 'timings' in chunk:
            t = chunk['timings']
            print(f\"Total tokens: {t.get('cache_n', 0) + t['prompt_n'] + t['predicted_n']}\")
            break
    except:
        pass
   "
   ```

### Reasoning Models

For reasoning models (e.g., o1, o3-mini, Qwen with thinking), the response includes reasoning content. **Different backends use different field names for this content:**

| Backend | Field Name | Example |
|---------|------------|---------|
| **llama.cpp** | `reasoning_content` | Used by local llama.cpp server |
| **OpenAI** (o1, o3-mini, etc.) | `reasoning` | OpenAI standard field |

#### Example Response (llama.cpp - uses `reasoning_content`)

```json
{
  "message": {
    "role": "assistant",
    "content": "The answer is 4.",
    "reasoning_content": "Let me think step by step...\n\n1. The user asks for 2+2\n2. 2 + 2 = 4"
  }
}
```

#### Example Response (OpenAI - uses `reasoning`)

```json
{
  "message": {
    "role": "assistant",
    "content": "The answer is 4.",
    "reasoning": "Let me think step by step...\n\n1. The user asks for 2+2\n2. 2 + 2 = 4"
  }
}
```

#### Streaming Mode

In streaming mode, reasoning content is sent as separate chunks. The field name depends on the backend:

- **llama.cpp streaming**: Uses `delta.reasoning_content`
- **OpenAI streaming**: Uses `delta.reasoning`

The inference client automatically detects which field the server uses and handles both transparently.

**Important**: The client tracks which reasoning field type was used (`ReasoningContentType`) to maintain consistency when storing reasoning content in conversation context. When sending messages back to the server, the same field name is used.

---

## Error Responses

### 400 Bad Request

```json
{
  "error": {
    "code": 400,
    "message": "Assistant response prefill is incompatible with enable_thinking.",
    "type": "invalid_request_error"
  }
}
```

### 401 Unauthorized

```json
{
  "error": {
    "code": 401,
    "message": "Invalid API key",
    "type": "authentication_error"
  }
}
```

### 429 Rate Limited

```json
{
  "error": {
    "code": 429,
    "message": "Rate limit exceeded",
    "type": "rate_limit_error"
  }
}
```

Retry after the time specified in the `Retry-After` header.

### 500+ Server Error

```json
{
  "error": {
    "code": 500,
    "message": "Internal server error",
    "type": "server_error"
  }
}
```

---

## Common Use Cases

### Basic Chat

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama",
    "messages": [
      {"role": "user", "content": "Hello, who are you?"}
    ],
    "max_tokens": 100
  }'
```

### Multi-turn Conversation

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama",
    "messages": [
      {"role": "user", "content": "What is 2+2?"},
      {"role": "assistant", "content": "2+2 equals 4."},
      {"role": "user", "content": "What about 3+3?"}
    ],
    "max_tokens": 50
  }'
```

### With Reasoning Content

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama",
    "messages": [
      {"role": "user", "content": "Solve: If x + 5 = 12, what is x?"}
    ],
    "max_tokens": 200,
    "stream": false
  }'
```

### Streaming with Tool Calls (Conceptual)

The API supports tool calling. When the model wants to use a tool, it returns tool calls in the response:

```json
{
  "choices": [
    {
      "finish_reason": "tool_calls",
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "bash",
              "arguments": "{\"command\": \"ls -la\"}"
            }
          }
        ]
      }
    }
  ],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 20,
    "total_tokens": 120
  }
}
```

---

## Image and Vision Support

The inference API supports multi-modal interactions, enabling vision-capable models to **understand images** and provide textual descriptions. This is used by the coding agent's `view_image` tool to analyze screenshots, diagrams, and other visual content.

Images are included as part of the message content using the multi-modal `ContentPart` array format.

---

### Image Understanding (Vision)

Vision-capable models (e.g., LLaVA, Qwen2.5-VL, GLM-4V) can analyze and reason about images. Images are included as part of the message content using the multi-modal `ContentPart` array format.

#### Type Definition

When a message contains images, the `content` field is an array of `ContentPart` objects:

```typescript
interface Message {
  role: "user" | "assistant" | "system" | "tool";
  content: string | ContentPart[]; // Can be a plain string OR an array of content parts
  // ... other fields
}

interface ContentPart {
  type: "text" | "image_url";
}

interface TextContentPart extends ContentPart {
  type: "text";
  text: string;
}

interface ImageContentPart extends ContentPart {
  type: "image_url";
  image_url: {
    url: string; // Public URL or base64 data URI
    detail?: "auto" | "low" | "high"; // Detail level (default: "auto")
  };
}
```

#### Supported Image Formats

- **PNG** (`image/png`)
- **JPEG** (`image/jpeg`)
- **WEBP** (`image/webp`)
- **GIF** (non-animated, `image/gif`)

#### Image Input Methods

**1. Public URL**

Provide a publicly accessible image URL:

```json
{
  "type": "image_url",
  "image_url": {
    "url": "https://example.com/photos/cat.jpg",
    "detail": "auto"
  }
}
```

**2. Base64 Data URI**

Embed the image directly in the request as a base64-encoded data URI:

```json
{
  "type": "image_url",
  "image_url": {
    "url": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQ...",
    "detail": "low"
  }
}
```

The format is: `data:[MIME_TYPE];base64,[BASE64_DATA]`

#### Detail Level

The `detail` parameter controls how the image is processed:

| Value | Description | Token Impact |
|-------|-------------|--------------|
| `"auto"` | Model decides the optimal resolution | Variable |
| `"low"` | Lower resolution (faster, fewer tokens) | Lower cost (~85 tokens) |
| `"high"` | Higher resolution (more detail, more tokens) | Higher cost (varies by image size) |

- `"low"`: Disables the "high res" mode. The model receives a low-res 512×512px version and produces ~85 tokens.
- `"high"`: Enables "high res" mode. The model receives a high-res version scaled to fit 2048×2048 with a 768×768 crop, producing significantly more tokens.
- `"auto"`: The model decides whether to treat the image as `"low"` or `"high"` based on input size.

#### Example: Single Image with Text

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llava",
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "What is in this image?"},
          {
            "type": "image_url",
            "image_url": {
              "url": "https://example.com/photo.jpg",
              "detail": "auto"
            }
          }
        ]
      }
    ],
    "max_tokens": 500
  }'
```

#### Example: Multiple Images

You can include multiple images in a single request:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llava",
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "Compare these two images and tell me the differences."},
          {
            "type": "image_url",
            "image_url": {
              "url": "https://example.com/image1.jpg",
              "detail": "high"
            }
          },
          {
            "type": "image_url",
            "image_url": {
              "url": "https://example.com/image2.jpg",
              "detail": "high"
            }
          }
        ]
      }
    ],
    "max_tokens": 1000
  }'
```

#### Example: Base64-Encoded Image

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"llava\",
    \"messages\": [
      {
        \"role\": \"user\",
        \"content\": [
          {\"type\": \"text\", \"text\": \"Describe this image.\"},
          {
            \"type\": \"image_url\",
            \"image_url\": {
              \"url\": \"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...\",
              \"detail\": \"auto\"
            }
          }
        ]
      }
    ],
    \"max_tokens\": 500
  }"
```

#### Example: Multi-turn Conversation with Images

Images can be referenced across multiple turns:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llava",
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "What is in this image?"},
          {
            "type": "image_url",
            "image_url": {"url": "https://example.com/diagram.png"}
          }
        ]
      },
      {"role": "assistant", "content": "This is a flowchart showing a decision process."},
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "Can you explain the third step in more detail?"}
        ]
      }
    ],
    "max_tokens": 500
  }'
```

#### Vision in Streaming Mode

Vision content works seamlessly with streaming. The content parts are sent in the request, and the response streams back as normal text:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llava",
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "Describe this image in detail."},
          {
            "type": "image_url",
            "image_url": {"url": "https://example.com/photo.jpg"}
          }
        ]
      }
    ],
    "max_tokens": 500,
    "stream": true
  }'
```

---

### Vision Token Usage

When using vision models, token usage includes both text and image tokens:

- **Text tokens**: Counted as usual
- **Image tokens**: Varies based on resolution and `detail` setting
  - `"low"` detail: ~85 tokens per image (fixed)
  - `"high"` detail: Varies by image dimensions (typically 1000-4000+ tokens)
  - `"auto"` detail: Model decides; usage reflects actual tokens consumed

The `usage` object in the response reflects the total:

```json
{
  "usage": {
    "prompt_tokens": 500,      // Text + image tokens combined
    "completion_tokens": 150,
    "total_tokens": 650
  }
}
```

---

### Supported Vision Models

The following vision-capable models support image understanding:

| Model | Description |
|---|---|
| LLaVA | Large Language-and-Vision Assistant |
| Qwen2.5-VL | Qwen Vision-Language model |
| GLM-4V | GLM-4 with Vision support |
| InternVL | InternVL vision-language model |

> **Note**: Not all inference backends support all vision models. Check your backend's model documentation for specific feature availability.

---

## Notes

1. **Token Accuracy**: The `total_tokens` field is the authoritative count of everything the API processed. This includes system prompt, all messages, tools, and the completion. Caching does not reduce this count.

2. **Streaming Behavior**: In streaming mode, token counts only appear in the final chunk. During streaming, only content deltas are sent.

3. **Reasoning Models**: If the model has thinking enabled, you cannot provide assistant response prefills (as shown in the 400 error example).

4. **Content Types**: Streaming supports three content types:

   - Normal content (`delta.content`)
   - Reasoning content via `delta.reasoning` (OpenAI standard field)
   - Reasoning content via `delta.reasoning_content` (llama.cpp field)

   The inference client handles both reasoning fields transparently.

5. **Finish Reasons**:
   - `"stop"`: Natural end of response
   - `"length"`: Reached `max_tokens`
   - `"tool_calls"`: Model requested tool use
   - `null`: Still streaming

6. **Image/Vision Support**: Messages can include images using the multi-modal `ContentPart[]` format. See the [Image and Vision Support](#image-and-vision-support) section for details on:
   - Sending images via URL or base64 data URIs
   - Image detail levels (`auto`, `low`, `high`)
   - Vision token usage
   - Supported vision models
