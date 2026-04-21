# Requirement 007: Inference Backend

## Description
The coding agent harness must connect to an inference backend running llama.cpp server, accessed via OpenAI API compatible REST endpoint.

## Acceptance Criteria
- [x] Connection configured via environment variable or config file
- [x] Supports OpenAI API compatible REST endpoint URL
- [x] Supports API key authentication
- [x] Compatible with llama.cpp server API endpoints
- [x] Handles connection errors gracefully
- [x] Implements retry logic for failed API calls
- [x] Streaming responses supported for better UX
- [x] Token usage information parsed from responses
- [x] Temperature parameter is only sent to the inference backend when explicitly set by the user
- [x] When temperature is not set, the inference engine uses the model-specific default value
