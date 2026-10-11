# Requirement 048: Schedule Ping Tool

## Description
The harness must support a `schedule` tool that lets the agent (and the user)
register **scheduled messages** ("pings"). A scheduled message is a block of
text that is injected into the conversation as a `user` message at a future
point in time, so the LLM sees it exactly as if the user had typed it.

Schedules can be one-shot ("in 10 minutes", "at 2026-05-01 09:00") or recurring
(cron expression or fixed interval). Registering a schedule must return
immediately: the tool call never blocks until the fire time.

Both the LLM and the user can list, remove, pause, resume and clear schedules.
Every delivered message is visible to the user (TUI and web UI) and is logged in
the conversation context like any other message. The run it triggers is reported
like any other run: the model's answer is streamed when streaming is enabled and
printed when it is not, and the prompt bar is restored afterwards. The number of
active schedules is shown as a counter on the prompt bar.

## Acceptance Criteria
- [x] Tool named `schedule` is available in normal and read-only modes
- [x] `add` action registers a scheduled message and **returns immediately**
- [x] One-shot scheduling by relative delay (`in 10m`) and absolute time (`at ...`)
- [x] Recurring scheduling by cron expression (`cron "*/5 * * * *"`) and fixed interval (`every 30m`)
- [x] Cron parser supports 5-field expressions, lists, ranges, steps and `@hourly`/`@daily`/`@weekly`/`@monthly`/`@yearly` macros, implemented with the standard library only
- [x] `list` action shows every schedule with ID, state, spec, next fire time, delivery count and message
- [x] `remove` action deletes a schedule by ID
- [x] `pause` / `resume` actions control a schedule without deleting it
- [x] `clear` action removes all schedules
- [x] Invalid parameters return clear error messages and are counted as failed tool calls
- [x] When a schedule fires, its message is appended to the agent context as a `user` message, labelled as scheduled
- [x] A fired message is visible to the user (TUI/web UI) with a distinct notification line
- [x] A fired message is written to the debug log as a user message
- [x] The reply to a fired message is shown to the user in both streaming and
      non-streaming modes, without being duplicated, and the prompt bar is
      restored when the scheduled run finishes
- [x] A fired message triggers an agent run when the agent is idle, and is picked up by the current run when the agent is busy (no concurrent runs)
- [x] Recurring schedules re-arm automatically; one-shot schedules are removed after delivery
- [x] `limit` optionally bounds the number of deliveries of a recurring schedule
- [x] The user can manage schedules with the `/schedule` slash command (same actions as the tool)
- [x] The number of active schedules is displayed on the prompt bar (TUI) and in the web UI status bar
- [x] The schedule count is included in `/stats` output
- [x] In read-only mode, `list`, `remove` and `clear` are allowed; `add`, `pause` and `resume` are blocked
- [x] Schedules are session-local in-memory state (no persistence, no external processes/crontab)
- [x] No external dependencies are introduced (requirement 024)

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "schedule",
    "description": "Schedule a message to be sent to the LLM later (once at a future time, or repeatedly on a cron/interval schedule). The call returns immediately. Use list/remove/pause/resume/clear to manage existing schedules.",
    "parameters": {
      "type": "object",
      "properties": {
        "action": {
          "type": "string",
          "description": "The action to perform: add, list, remove, pause, resume, or clear"
        },
        "message": {
          "type": "string",
          "description": "The message to send to the LLM when the schedule fires (required for add)"
        },
        "in": {
          "type": "string",
          "description": "One-shot delay from now, e.g. 10m, 90s, 2h30m (add only)"
        },
        "at": {
          "type": "string",
          "description": "One-shot absolute local time, e.g. 2026-05-01T09:00:00 or \"2026-05-01 09:00\" (add only)"
        },
        "cron": {
          "type": "string",
          "description": "Recurring 5-field cron expression, e.g. \"*/5 * * * *\" (add only)"
        },
        "every": {
          "type": "string",
          "description": "Recurring fixed interval, e.g. 30m (add only)"
        },
        "id": {
          "type": "integer",
          "description": "Schedule ID (required for remove, pause, resume)"
        },
        "name": {
          "type": "string",
          "description": "Optional human-readable label for the schedule"
        },
        "limit": {
          "type": "integer",
          "description": "Optional maximum number of deliveries for a recurring schedule (0 = unlimited)"
        }
      },
      "required": ["action"]
    }
  }
}
```

## Parameters
- `action` (string, required): `add`, `list`, `remove`, `pause`, `resume`, `clear`.
- `message` (string, required for `add`): text delivered to the LLM on each firing.
- `in` (string): relative delay for a one-shot schedule (`10m`, `90s`, `2h30m`, `10 minutes`).
- `at` (string): absolute local time for a one-shot schedule (RFC3339, `2006-01-02 15:04:05`, `2006-01-02 15:04`, or `15:04`).
- `cron` (string): 5-field cron expression for a recurring schedule.
- `every` (string): fixed interval for a recurring schedule.
- `id` (integer): schedule ID, required for `remove`, `pause`, `resume`.
- `name` (string, optional): label shown in listings and delivery notifications.
- `limit` (integer, optional): stop after this many deliveries (recurring only).

Exactly one of `in`, `at`, `cron`, `every` must be supplied for `add`.

## Schedule Entry Structure
- **id**: auto-incrementing integer starting at 1
- **name**: optional label
- **message**: text injected into the conversation
- **kind**: `once`, `cron`, or `every`
- **spec**: the spec as provided (`10m`, `2026-05-01T09:00:00`, `*/5 * * * *`)
- **next**: next fire time
- **created / lastFired / fireCount**: bookkeeping for listings
- **limit**: maximum deliveries (0 = unlimited; one-shot entries always 1)
- **paused / done**: control and lifecycle flags

## Return Values

### Add Success
- `success: true`
- `output`: `Scheduled #3 (cron "*/5 * * * *") next 2026-05-01 09:05:00: "Run the test suite"`
- Extra: `id`, `kind`, `next`

### List Success
- `success: true`
- `output`: one line per schedule
  ```
  #1 [active]  once at 2026-05-01 09:10:00 (in 9m58s)          :: Run the test suite
  #2 [active]  cron "*/5 * * * *" (next 09:05:00, fired 3)     :: Check the build
  #3 [paused]  every 30m                                       :: Stand-up reminder
  ```
- Extra: `totalSchedules`, `activeSchedules`, `pausedSchedules`

### Remove / Pause / Resume / Clear Success
- `success: true`
- `output`: confirmation naming the affected schedule(s)

### Error Cases
- Missing `action` → `missing required parameter: action`
- Unknown action → `invalid action: X (must be one of: add, list, remove, pause, resume, clear)`
- Missing `message` → `missing required parameter: message`
- No/ambiguous timing spec → `exactly one of in, at, cron, or every is required`
- Bad duration → `invalid duration "X" (use e.g. 10m, 90s, 2h30m, "10 minutes")`
- Bad cron → `invalid cron expression: ...`
- Past one-shot time → `scheduled time ... is in the past`
- Unknown ID → `schedule #X not found`
- Missing `id` → `missing required parameter: id`

## Delivery Semantics
When the scheduler fires an entry:
1. The entry is re-armed (cron/interval) or retired (one-shot / limit reached)
   atomically inside the store, so a message can never be delivered twice.
2. A `user` message is appended to the agent context:
   ```
   [Scheduled message #2 (check-build) at 2026-05-01 09:05:00]
   Check the build
   ```
   The label is generated by the harness, never by the model, so the model cannot
   mistake a scheduled ping for real user input.
3. The message is counted in the context-size estimate and written to the debug
   log as a user message.
4. A notification line is streamed to the UI with the dedicated
   `schedule` streaming content type, so the user sees it immediately.
5. If the agent is idle, a run is started for the injected message; if a run is
   already in progress, the message is picked up by that run's next iteration.
   Runs are serialized by a run gate — a scheduled run never overlaps a
   user-initiated run.
6. The prompt-bar schedule counter is refreshed.

## Read-Only Mode Behavior
Schedules are session-local state, mirroring the `todo` tool:
- `list`, `remove`, `clear` — allowed
- `add`, `pause`, `resume` — **blocked** with
  `Tool 'schedule' action 'X' is not available in read-only mode`

## User Interface
- **TUI**: the prompt bar shows `[Schedules: N]` when N > 0, e.g.
  `[Context: 12000 / 128000 (9.4%) ✓] [Schedules: 2] > `
- **TUI**: `/schedule` slash command mirrors the tool actions (flags first, then
  the timing spec, then the message; quotes are honored):
  - `/schedule` or `/schedule list`
  - `/schedule add in 10m Check the build`
  - `/schedule add at "2026-05-01 09:00" Stand up`
  - `/schedule add cron "*/5 * * * *" Check the build`
  - `/schedule add every 30m Stand up`
  - `/schedule add --name standup --limit 3 every 1h Review the backlog`
  - `/schedule remove 2`, `/schedule pause 2`, `/schedule resume 2`, `/schedule clear`
- **TUI**: delivered messages are printed in a distinct color; if the user is
  mid-typing, the prompt line (including the counter) is redrawn.
- **Web UI**: the status bar shows `Schedules: N`, delivered messages render as a
  "Scheduled" block, and `/schedule ...` works as a slash command.
- **`/stats`**: shows the active schedule count.

## Scheduling Notes
- Schedules live in memory for the lifetime of the session. They are not
  persisted by `/dump` / `--load` and do not survive process exit; one-shot mode
  (`-p`) does not run the scheduler because the process exits after the run.
- The scheduler wakes at the earliest pending fire time, polling at most every
  30 seconds, and re-evaluates immediately when schedules are added or removed.
- Cron expressions are evaluated in the agent's local time zone.

## Implementation Notes
- `tools/cron.go`: stdlib-only cron parser and `NextAfter` calculator
  (5 fields, lists/ranges/steps, month/weekday names, macros, standard
  day-of-month/day-of-week union semantics).
- `tools/schedule.go`: `ScheduleStore` (mutex-protected, change notifier) and
  the `schedule` tool handler; the store is owned by the `ToolExecutor` and
  shared with the agent's scheduler.
- `agent/schedule.go`: scheduler goroutine, delivery into context, run gate,
  UI hooks (`ScheduleHooks`), `ScheduleCount`.
- `tui/tui.go`: prompt-bar counter, schedule chunk coloring, prompt redraw.
- `main.go` / `webui`: slash command, scheduler lifecycle, counter display.
