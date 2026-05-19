# Requirement 038: Goal Mode (/goal command)

## Description
When a user sets a goal using the `/goal` command, the agent enters goal mode. In goal mode, whenever the inference naturally ends (i.e., the LLM returns a response without tool calls), the agent automatically injects a goal-check prompt as if it were a user prompt. This prompt goes through the full agentic loop: the LLM can run tools, make more progress, and continue working. Only when the agentic loop naturally ends again (no more tool calls) is the response checked for "goal achieved". This allows the LLM to properly verify its work before confirming completion.

## Acceptance Criteria
- [ ] `/goal <prompt>` command activates goal mode with the given prompt
- [ ] Goal is checked after each inference response with no tool calls (natural end)
- [ ] Goal check is injected as an automatically generated user message
- [ ] The goal check goes through the full agentic loop (LLM can run tools, make progress)
- [ ] After the goal-check agentic loop ends naturally (no tool calls), check for "goal achieved"
- [ ] If the LLM response contains "goal achieved" (case-insensitive), the agent stops
- [ ] If the LLM response does not contain "goal achieved", inject another goal check and continue
- [ ] Goal can be deactivated with `/goal-off` command
- [ ] Goal status is displayed in the TUI when active
- [ ] Goal checking message is displayed in the TUI when a goal check is being performed
- [ ] Goal checking message uses a distinctive color (magenta) to stand out from other messages
- [ ] Goal achieved confirmation message is displayed in the TUI in the special color
- [ ] Goal checking happens transparently without user intervention
- [ ] Goal mode works with both streaming and non-streaming modes

## Goal Checking Algorithm

### How Goal Checking Works
The goal check is an **automatically injected user prompt** that goes through the **full agentic loop**:

1. **Natural End Detected**: The LLM returns a response with no tool calls
2. **Inject Goal Check**: If goal mode is active, inject a user message asking if the goal is achieved
3. **Full Agentic Loop**: The LLM processes this prompt normally:
   - It can run tools to verify its work (e.g., `read_file` to check files, `bash` to test)
   - It can continue working on unfinished tasks
   - It can declare "goal achieved" if satisfied
4. **Check Natural End Again**: When the LLM responds with no tool calls:
   - If response contains "goal achieved" (case-insensitive) → stop and return
   - Otherwise → inject another goal check (loop back to step 2)

### Key Design Principle
The goal check is NOT a separate/special API call. It is treated identically to a user prompt:
- Tools are available and can be used
- The LLM can decide to continue working or verify its work
- Only the "natural end" (no tool calls) is checked for "goal achieved"

### Example Flows

#### Flow 1: Goal Already Achieved
```
User: "Write a Go web server"
Agent: [runs tools, writes files, returns response]

# Natural end - no tool calls
[Goal Check Injected] -> User: "Have you achieved the goal: 'Write a Go web server'?"
LLM: "I have written the server code. Goal achieved."

# Natural end - contains "goal achieved" - agent stops
```

#### Flow 2: LLM Verifies Work Before Confirming
```
User: "Create a REST API"
Agent: [runs tools, writes code, returns response]

# Natural end - no tool calls
[Goal Check Injected] -> User: "Have you achieved the goal: 'Create a REST API'?"
LLM: [Runs tools to verify: read_file, bash to test]
LLM: [After verifying] "I have verified the code compiles and runs. Goal achieved."

# Natural end - contains "goal achieved" - agent stops
```

#### Flow 3: Goal Not Yet Achieved
```
User: "Build a full-stack application"
Agent: [does some work, returns response]

# Natural end - no tool calls
[Goal Check Injected] -> User: "Have you achieved the goal: 'Build a full-stack application'?"
LLM: [Continues working, runs more tools]
LLM: [Returns response about progress]

# Natural end - no "goal achieved"
[Goal Check Injected Again] -> User: "Have you achieved the goal: 'Build a full-stack application'?"
... (continues until goal is achieved or max iterations)
```

## Commands

### `/goal <prompt>`
Activates goal mode with the given prompt.

```
/goal Create a REST API with user authentication
```

### `/goal-off`
Deactivates goal mode.

## Implementation Details

### Agent State
The agent needs to track:
- `goal string`: The current goal prompt (empty string means goal mode is off)
- `goalActive bool`: Whether goal mode is currently active

### Goal Check Message Format
When checking the goal, inject a user message like:
```
User: "Please review the current state of your work. Have you achieved the following goal?

Goal: <goal_prompt>

If you have achieved the goal, respond with 'goal achieved'.
If you have not achieved the goal, explain what remains to be done and continue working."
```

### Integration with Main Loop
The goal check integrates into the main agentic loop as follows:

```
for each iteration:
    1. Get inference response (LLM can return tool calls OR text)
    2. Add assistant response to context
    3. If response has tool calls:
        - Execute tools
        - Add tool results to context
        - Continue to next iteration
    4. If response has NO tool calls (natural end):
        - If goal mode is active:
            - If "goal achieved" in response → STOP and return
            - Otherwise → inject goal check message, continue to next iteration
        - If goal mode is NOT active → STOP and return
```

### Case-Insensitive Matching
The string "goal achieved" should be matched case-insensitively:
- "goal achieved" ✓
- "Goal Achieved" ✓
- "GOAL ACHIEVED" ✓
- "I have achieved the goal" ✗ (not matched - must contain "goal achieved")

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Goal mode active but inference fails | Return error, do not silently continue |
| Empty goal prompt | Show error message, do not activate goal mode |
| Goal check causes max iterations | Return error indicating iteration limit reached |

## TUI Feedback

### Goal Check Message
When the goal check is being performed (i.e., when the goal check prompt is being sent to the LLM), display a message in the TUI:

```
[Goal Check] Checking if goal is achieved: "<goal_prompt>"
```

This message should be displayed in **magenta** (`\033[35m`) to distinguish it from other message types.

### Goal Achieved Confirmation
When the goal has been achieved (i.e., the LLM response contains "goal achieved"), display a confirmation message in the TUI:

```
[Goal Achieved] ✓ Goal has been achieved!
```

This message should also be displayed in **magenta** (`\033[35m`) for consistency.

### Color Scheme
- Goal messages use magenta color (`\033[35m`) to stand out from:
  - Normal content (white/default)
  - Reasoning content (dim/bright black)
  - Tool calls (cyan)
  - Success messages (green)
  - Error messages (red)

### Implementation
The goal messages should be streamed through the existing stream callback mechanism using a new content type `StreamingContentTypeGoal`. The TUI should handle this content type by applying the magenta color.

## Testing Requirements

- [ ] Setting a goal activates goal mode
- [ ] Goal check is injected as a user message (not a separate API call)
- [ ] LLM can run tools during goal check agentic loop
- [ ] "goal achieved" at natural end stops the agent
- [ ] Non-"goal achieved" at natural end injects another goal check
- [ ] Goal mode works in streaming mode
- [ ] Goal mode works in non-streaming mode
- [ ] Goal can be deactivated with /goal-off
- [ ] Case-insensitive matching works correctly
