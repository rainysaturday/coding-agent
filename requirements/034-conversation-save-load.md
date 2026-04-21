# Feature #034: Conversation Save/Load

## Overview
Allow the agent to save and load conversation sessions so users can continue from where they left off across separate invocations.

## Requirements

### Core Functionality
- [ ] `/save [filename]` command to save current conversation session
- [ ] `/load [filename]` command to load a previous conversation session
- [ ] Default filename is `session.json` when no filename specified
- [ ] Session files saved in the current working directory
- [ ] Session files are in JSON format (human-readable)

### Session Data Structure
- [ ] System prompt is saved and restored
- [ ] Message history (role, content, tool_calls, tool_call_id) is preserved
- [ ] Timestamp and metadata included in saved session
- [ ] Token usage stats preserved

### Error Handling
- [ ] Clear error messages for file not found on load
- [ ] Validation of session file format
- [ ] Graceful handling of corrupted session files

### User Feedback
- [ ] Confirmation message on successful save
- [ ] Confirmation message on successful load
- [ ] Display filename and step count in confirmation

## Acceptance Criteria
1. User can run `/save` during a conversation and a session.json file is created
2. User can exit and restart the agent, then run `/load session.json` to continue
3. Saved sessions include all messages, tool calls, and results
4. After loading a session, the conversation context is fully restored
5. Error messages are clear when loading a non-existent or invalid file
