# Requirement 010: Streaming Inference

## Description
The inference engine must operate in streaming mode to display tokens as they are generated, improving user experience. The timeout for receiving the initial token must be configurable to support slower hardware.

## Acceptance Criteria
- [ ] Inference operates in streaming mode by default
- [ ] Tokens are displayed in real-time as they are generated
- [ ] Initial token timeout is configurable via environment variable
- [ ] Initial token timeout is configurable via command-line flag
- [ ] Initial token timeout is configurable via config file
- [ ] Default initial token timeout is 2 hours (7200 seconds)
- [ ] Timeout value must be a positive integer
- [ ] Timeout validation ensures reasonable minimum (e.g., 10 seconds)
- [ ] Timeout exceeded triggers appropriate error message
- [ ] Streaming can be disabled if explicitly configured
- [ ] Partial tokens are buffered and displayed atomically
- [ ] Connection timeout and read timeout are configurable separately
