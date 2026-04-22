# Feature #061: Test Coverage Analysis Tool

## Description
Run project tests with coverage analysis and return structured coverage results including per-file and per-package coverage percentages.

## Acceptance Criteria

- [ ] Tool name: `run_coverage`
- [ ] Auto-detects project type (Go, Python, Node.js)
- [ ] Runs tests with coverage flags appropriate for each language
- [ ] Parses coverage output and returns structured results
- [ ] Reports overall coverage percentage
- [ ] Reports per-file coverage with percentage
- [ ] Reports per-package (Go) or per-module (Python) coverage
- [ ] Highlights files with low coverage (< 50%) and no coverage (0%)
- [ ] Supports custom command override (for non-standard projects)
- [ ] Supports custom coverage output format specification
- [ ] Supports max_lines limit to prevent context overflow
- [ ] Supports timeout configuration
- [ ] Uses only Go stdlib for parsing - zero external dependencies
- [ ] Go: Uses `go test -coverprofile` and `go tool cover`
- [ ] Python: Uses `pytest-cov` or `coverage.py`
- [ ] Node.js: Uses `npx c8` or `npx istanbul`

## Output Format
The tool returns:
- `overall`: Overall coverage percentage
- `files`: Array of file coverage entries with path, covered_lines, total_lines, percentage
- `packages` (Go only): Array of package coverage entries
- `low_coverage_files`: Files with < 50% coverage
- `no_coverage_files`: Files with 0% coverage
- `exit_code`: Test exit code
- `message`: Human-readable summary

## Implementation Notes
- Leverage existing project type detection from `run_tests` tool
- For Go: `go test -coverprofile=cover.out ./...` then parse cover profile
- For Python: `python -m pytest --cov=project --cov-report=term-missing`
- For Node.js: `npx c8 --reporter=lcov --reporter=text --`
- Cover profile parsing is straightforward (Go format is simple CSV-like)
