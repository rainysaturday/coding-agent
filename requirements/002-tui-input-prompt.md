# Requirement 002: TUI with Input Prompt

## Description
The harness must provide a minimal terminal user interface (TUI) that includes an input prompt for user interaction. Commands to the agent should be prefixed with '/' to distinguish them from natural language prompts sent to the LLM.

## Command Prefix Convention
- All agent commands must start with '/' (forward slash)
- This clearly separates user commands from natural language prompts
- Examples: `/stats`, `/clear`, `/clear-history`
- Natural language prompts (without '/') are sent directly to the LLM

## Acceptance Criteria
- [ ] TUI displays in terminal with clear output area
- [ ] Input prompt accepts user text input
- [ ] Enter key submits input
- [ ] Commands starting with '/' are interpreted as agent commands (not sent to LLM)
- [ ] Natural language input (without '/') is sent to the LLM as a prompt
- [ ] Output displays previous messages/requests and tool feedback
- [ ] Basic styling or formatting for readability

## Supported Commands
- `/stats` - Display runtime statistics
- `/clear` - Clear the output display
- `/clear-history` - Clear input history

## Examples
```
> /stats
==================================================
Runtime Statistics
==================================================
Input Tokens:      1500
Output Tokens:     800
...

> Create a Go hello world program
[Agent processes this as a natural language prompt]

> /clear
[Output display is cleared]
```
