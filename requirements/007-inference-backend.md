# Requirement 007: Inference Backend

## Description
The coding agent harness must connect to an inference backend running llama.cpp server, accessed via OpenAI API compatible REST endpoint.

## Acceptance Criteria
- [ ] Connection configured via environment variable or config file
- [ ] Supports OpenAI API compatible REST endpoint URL
- [ ] Supports API key authentication
- [ ] Compatible with llama.cpp server API endpoints
- [ ] Handles connection errors gracefully
- [ ] Implements retry logic for failed API calls
- [ ] Streaming responses supported for better UX
- [ ] Token usage information parsed from responses
