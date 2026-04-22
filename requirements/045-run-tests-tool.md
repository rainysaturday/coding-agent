# #045: Run Tests Tool

## Description
A tool that allows the agent to execute tests for the current project and report structured results back to the conversation.

## Acceptance Criteria

- [ ] Tool is named `run_tests` and registered in the agent's tool set
- [ ] Auto-detects project type from common manifest files:
  - `go.mod` → Go test suite (`go test`)
  - `package.json` → npm test (`npm test`)
  - `requirements.txt` or `pyproject.toml` → Python test (`pytest`)
  - `Makefile` with `test` target → `make test`
- [ ] Accepts optional `command` parameter to override auto-detected command
- [ ] Accepts optional `args` parameter for additional test arguments (e.g., `-v`, `-run TestFoo`, `./pkg/...`)
- [ ] Accepts optional `timeout` parameter (default: 60 seconds)
- [ ] Returns structured results including:
  - `exit_code` - process exit code
  - `stdout` - captured standard output
  - `stderr` - captured standard error
  - `passed` - boolean indicating whether tests passed
  - `summary` - human-readable summary of results
- [ ] Truncates output if it exceeds 10000 characters
- [ ] Returns a clear error message if no project type is detected
- [ ] Works in non-interactive mode (no TUI required)
- [ ] Is added to the system prompt with proper tool calling format

## Notes
- This tool is essential for the edit-run-verify workflow
- Should handle both passing and failing test suites gracefully
- Timeout prevents hanging on tests that don't complete
