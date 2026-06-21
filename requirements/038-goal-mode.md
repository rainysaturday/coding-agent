# Requirement 038: Goal Mode (/goal command)

## Description
When a user sets a goal using the `/goal` command, the agent enters goal mode and **immediately begins working** on the goal. The goal prompt itself serves as the first user message, so the agent starts executing right away without requiring additional user input.

In goal mode, whenever the inference naturally ends (i.e., the LLM returns a response without tool calls), the agent automatically injects a goal-check prompt as if it were a user prompt. This prompt goes through the full agentic loop: the LLM can run tools, make more progress, and continue working. Only when the agentic loop naturally ends again (no more tool calls) is the response checked for "goal achieved". This allows the LLM to properly verify its work before confirming completion.

When the goal is achieved, **the goal is automatically reset and removed** so that the agent returns to normal mode and can accept new input without the goal-checking behavior interfering.

## Acceptance Criteria
- [x] `/goal <prompt>` command activates goal mode with the given prompt
- [x] **The goal prompt is also sent as the first user message**, so the agent starts working immediately without requiring additional input
- [x] Goal is checked after each inference response with no tool calls (natural end)
- [x] Goal check is injected as an automatically generated user message
- [x] The goal check goes through the full agentic loop (LLM can run tools, make progress)
- [x] After the goal-check agentic loop ends naturally (no tool calls), check for "goal achieved"
- [x] If the LLM response contains "goal achieved" (case-insensitive), the agent stops
- [x] **When the goal is achieved, the goal is automatically reset and removed** (goal mode is deactivated)
- [x] If the LLM response does not contain "goal achieved", inject another goal check and continue
- [x] Goal can be manually deactivated with `/goal-off` command
- [x] Goal status is displayed in the TUI when active
- [x] Goal checking message is displayed in the TUI when a goal check is being performed
- [x] Goal checking message uses a distinctive color (magenta) to stand out from other messages
- [x] Goal achieved confirmation message is displayed in the TUI in the special color
- [x] Goal checking happens transparently without user intervention
- [x] Goal mode works with both streaming and non-streaming modes
- [x] When the goal is achieved, the elapsed time since the goal was started is displayed
- [x] Elapsed time is shown in raw seconds as well as formatted in hours, minutes, and seconds
- [x] When a new goal is set, the timer resets to zero so each goal is measured individually

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
   - If response contains "goal achieved" (case-insensitive) → **automatically clear the goal**, stop and return
   - Otherwise → inject another goal check (loop back to step 2)

### Key Design Principle
The goal check is NOT a separate/special API call. It is treated identically to a user prompt:
- Tools are available and can be used
- The LLM can decide to continue working or verify its work
- Only the "natural end" (no tool calls) is checked for "goal achieved"

### Example Flows

#### Flow 1: Goal Set and Immediate Execution
```
User: /goal Write a Go web server

[Goal mode activated: "Write a Go web server"]
[Agent starts working immediately with the goal as the first prompt]

Agent: [runs tools, writes files, returns response]

# Natural end - no tool calls
[Goal Check] Checking if goal is achieved: "Write a Go web server"
LLM: "I have written the server code. Goal achieved."

# Natural end - contains "goal achieved" - goal is auto-cleared, agent stops
[Goal Achieved] ✓ Goal has been achieved! Time: 125s (2m 5s)
```

#### Flow 2: LLM Verifies Work Before Confirming
```
User: /goal Create a REST API

[Goal mode activated: "Create a REST API"]
[Agent starts working immediately]

Agent: [runs tools, writes code, returns response]

# Natural end - no tool calls
[Goal Check] Checking if goal is achieved: "Create a REST API"
LLM: [Runs tools to verify: read_file, bash to test]
LLM: [After verifying] "I have verified the code compiles and runs. Goal achieved."

# Natural end - contains "goal achieved" - goal is auto-cleared, agent stops
[Goal Achieved] ✓ Goal has been achieved! Time: 45s
```

#### Flow 3: Goal Not Yet Achieved
```
User: /goal Build a full-stack application

[Goal mode activated: "Build a full-stack application"]
[Agent starts working immediately]

Agent: [does some work, returns response]

# Natural end - no tool calls
[Goal Check] Checking if goal is achieved: "Build a full-stack application"
LLM: [Continues working, runs more tools]
LLM: [Returns response about progress]

# Natural end - no "goal achieved"
[Goal Check] Checking if goal is achieved: "Build a full-stack application"
... (continues until goal is achieved or max iterations)
```

## Commands

### `/goal <prompt>`
Activates goal mode with the given prompt. **The agent immediately starts working on the goal** — the goal prompt serves as the first user message.

```
/goal Create a REST API with user authentication
```

When the goal is achieved, the goal is **automatically cleared** and the agent returns to normal interactive mode.

### `/goal-off`
Manually deactivates goal mode at any time (before or after the goal is achieved).

## CLI Flag

### `--goal <prompt>`
Activates goal mode with the given prompt in one-shot (non-interactive) mode. Works similarly to `--prompt`:
- The agent immediately starts working on the goal
- The goal prompt serves as the first user message
- Goal checking behavior is identical to `/goal` command

```
coding-agent --goal "Create a REST API with user authentication"
```

When combined with other one-shot flags like `--prompt-file` or `--stdin`, `--goal` takes precedence as the prompt text.

## Implementation Details

### Agent State
The agent needs to track:
- `goal string`: The current goal prompt (empty string means goal mode is off)
- `goalActive bool`: Whether goal mode is currently active
- `goalStartTime time.Time`: When the current goal was started (used to measure elapsed time)

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
            - If "goal achieved" in response → auto-clear goal, STOP and return
            - Otherwise → inject goal check message, continue to next iteration
        - If goal mode is NOT active → STOP and return
```

### Auto-Clear on Goal Achievement
When "goal achieved" is detected at a natural end:
1. Display the goal achieved confirmation message
2. **Call `ClearGoal()`** to reset `goalActive = false` and `goal = ""`
3. Return the result

This ensures that subsequent interactions are not affected by the previous goal.

### Case-Insensitive Matching
The string "goal achieved" should be matched case-insensitively:
- "goal achieved" ✓
- "Goal Achieved" ✓
- "GOAL ACHIEVED" ✓
- "I have achieved the goal" ✗ (not matched - must contain "goal achieved")

### Goal Timing
When a goal is set, the agent records the start time. When the goal is achieved, the elapsed time is calculated and displayed in the goal achieved confirmation message.

The elapsed time is formatted to show:
- Raw seconds (e.g., `125s`)
- Hours, minutes, and seconds when applicable (e.g., `1h 5m 25s`)

Example output:
```
[Goal Achieved] ✓ Goal has been achieved! Time: 125s (2m 5s)
[Goal Achieved] ✓ Goal has been achieved! Time: 3665s (1h 1m 5s)
[Goal Achieved] ✓ Goal has been achieved! Time: 45s
```

**Timer Reset**: When a new goal is set (via `/goal`), the timer resets to zero. This ensures that each goal is measured individually, even within the same session. Clearing the goal (via `/goal-off` or automatic clear on achievement) also resets the timer.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Goal mode active but inference fails | Return error, do not silently continue |
| Empty goal prompt | Show error message, do not activate goal mode |
| Goal check causes max iterations | Return error indicating iteration limit reached |

## TUI Feedback

### Goal Activation Message
When the goal is set, display:
```
[Goal mode activated: "Create a REST API"]
```

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

After this message, the goal is automatically cleared and the TUI returns to the normal input prompt.

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

- [x] Setting a goal activates goal mode
- [x] Goal prompt is sent as the first user message (agent starts immediately)
- [x] Goal check is injected as a user message (not a separate API call)
- [x] LLM can run tools during goal check agentic loop
- [x] "goal achieved" at natural end stops the agent
- [x] **Goal is automatically cleared when "goal achieved" is detected**
- [x] Non-"goal achieved" at natural end injects another goal check
- [x] Goal mode works in streaming mode
- [x] Goal mode works in non-streaming mode
- [x] Goal can be deactivated with /goal-off
- [x] Case-insensitive matching works correctly
- [x] Setting a new goal resets the elapsed time timer
- [x] Elapsed time is displayed when goal is achieved
- [x] Elapsed time format correctly shows seconds, minutes, and hours
