# Requirement 007: Inference Backend

## Description
The coding agent harness must connect to an inference backend running llama.cpp server (or compatible OpenAI API endpoint), accessed via the API defined in [Inference API Specification](../../specifications/inference-api.md).

## Reasoning Content Support

Different inference backends use different field names for reasoning/thinking content:

| Backend | Field Name | Description |
|---------|------------|-------------|
| llama.cpp | `reasoning_content` | llama.cpp uses `reasoning_content` for thinking/reasoning content |
| OpenAI (o1, o3-mini, etc.) | `reasoning` | OpenAI uses `reasoning` as the standard field |

The inference client automatically detects which field the server uses and normalizes it internally. It also tracks the field type via `ReasoningContentType` (either `"reasoning"` or `"reasoning_content"`) to preserve consistency when storing reasoning content in conversation context.

## Acceptance Criteria
- [ ] Connection configured via environment variable or config file
- [ ] Supports API endpoint URL per the [Inference API Specification](../../specifications/inference-api.md)
- [ ] Supports API key authentication
- [ ] Compatible with the [Inference API Specification](../../specifications/inference-api.md)
- [ ] Handles connection errors gracefully
- [ ] Implements retry logic for failed API calls
- [ ] Streaming responses supported for better UX
- [ ] Token usage information parsed from responses
- [ ] Temperature parameter is only sent to the inference backend when explicitly set by the user
- [ ] When temperature is not set, the inference engine uses the model-specific default value
- [ ] Supports reasoning content from both `reasoning` (OpenAI) and `reasoning_content` (llama.cpp) fields
- [ ] Detects and tracks which reasoning field type the server uses
