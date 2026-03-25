# Requirement 008: Configurable Context Size

## Description
The coding agent harness must support configurable context window size for the LLM, with a default value.

## Acceptance Criteria
- [ ] Context size is configurable via environment variable
- [ ] Context size is configurable via command-line flag
- [ ] Context size is configurable via config file
- [ ] Default context size is 128000 tokens
- [ ] Context size can be set as integer value
- [ ] Validation ensures context size is positive
- [ ] Context size is enforced in API requests
- [ ] Warning or error on unsupported context sizes from backend
