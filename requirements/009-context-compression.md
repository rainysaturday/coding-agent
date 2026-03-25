# Requirement 009: Context Compression

## Description
When the context grows beyond the configured context size limit, the harness must automatically compress the context by summarizing the conversation history while preserving the initial prompt.

## Acceptance Criteria
- [ ] Context size is monitored during conversation
- [ ] Compression triggered when context exceeds configured limit
- [ ] Compression requests summarization from inference engine
- [ ] Initial system prompt is preserved at the beginning
- [ ] Summary replaces the full conversation history
- [ ] Prompt prefix is concatenated before the summary
- [ ] Compression happens transparently to the user
- [ ] Compression success/failure is logged
- [ ] Failed compression does not crash the agent
- [ ] Compression reduces context size below the limit
- [ ] Original context before compression is discarded
