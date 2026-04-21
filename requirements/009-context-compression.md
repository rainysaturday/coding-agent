# Requirement 009: Context Compression

## Description
When the context grows beyond the configured context size limit, the harness must automatically compress the context by summarizing the conversation history while preserving the initial prompt.

## Acceptance Criteria
- [x] Context size is monitored during conversation
- [x] Compression triggered when context exceeds configured limit
- [x] Compression requests summarization from inference engine
- [x] Initial system prompt is preserved at the beginning
- [x] Summary replaces the full conversation history
- [x] Prompt prefix is concatenated before the summary
- [x] Compression happens transparently to the user
- [x] Compression success/failure is logged
- [x] Failed compression does not crash the agent
- [x] Compression reduces context size below the limit
- [x] Original context before compression is discarded
