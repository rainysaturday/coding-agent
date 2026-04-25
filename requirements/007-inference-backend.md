# Requirement 007: Inference Backend

## Description
The coding agent harness must connect to an inference backend running llama.cpp server, accessed via the API defined in [Inference API Specification](../../specifications/inference-api.md).

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
