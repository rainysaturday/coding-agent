# #068: Test Generator Tool

## Description
A tool that analyzes source code files and generates unit tests with language auto-detection.

## Acceptance Criteria

- [ ] Tool is named `testgen` and registered in the agent's tool set
- [ ] Accepts `path` parameter (source file to generate tests for)
- [ ] Accepts optional `language` parameter (auto-detect from file extension if not provided)
- [ ] Accepts optional `output` parameter (test file output path, defaults to test file next to source)
- [ ] Accepts optional `test_framework` parameter (e.g., "testing" for Go, "pytest" for Python, "jest" for JS/TS)
- [ ] Auto-detects project type from file extension:
  - `.go` → Go `testing` package (table-driven tests)
  - `.py` → Python `pytest` (parametrized tests)
  - `.js` / `.ts` → JavaScript/TypeScript Jest test suites
- [ ] Extracts function/method signatures from source code
- [ ] Generates comprehensive test cases covering:
  - Normal/expected inputs
  - Edge cases (empty input, zero, nil/None)
  - Error cases (invalid input, failure paths)
- [ ] Uses only Go stdlib for analysis - zero external dependencies
- [ ] Test output is written as a new file
- [ ] Returns a clear error if source file is not found or is not a supported language
- [ ] Is added to the system prompt with proper tool calling format

## Tool Parameters
- `path` (required): Source file path to analyze
- `language` (optional): Language hint - "go", "python", "javascript", "typescript"
- `output` (optional): Output path for generated test file
- `test_framework` (optional): Test framework name

## Notes
- This tool is a companion to #045 (Run Tests Tool) - first generate, then run tests
- Go tests follow table-driven test patterns
- Python tests use pytest with @pytest.mark.parametrize
- JS/TS tests use Jest describe/it pattern
