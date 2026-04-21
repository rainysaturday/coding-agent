# Requirement 008: Configurable Context Size

## Description
The coding agent harness must support configurable context window size for the LLM, with a default value.

## Acceptance Criteria
- [x] Context size is configurable via environment variable
- [x] Context size is configurable via command-line flag
- [x] Context size is configurable via config file
- [x] Default context size is 128000 tokens
- [x] Context size can be set as integer value
- [x] Validation ensures context size is positive
- [x] Context size is enforced in API requests
- [x] Warning or error on unsupported context sizes from backend
