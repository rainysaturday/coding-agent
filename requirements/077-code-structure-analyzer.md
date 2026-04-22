# #077: Code Structure Analyzer Tool

## Description
Generate a structured summary of source code files including functions, types, interfaces, variables, constants, and exports. Helps agents quickly understand unfamiliar codebases.

## Acceptance Criteria

### Core Features
- [x] Analyze source files and extract code structure
- [x] Support multiple languages with zero external dependencies
- [x] Return structured JSON output with all found elements
- [x] Support file-level and directory-level analysis

### Supported Languages
- [x] Go (using `ast` package for accurate parsing)
- [x] Python (regex-based parsing)
- [x] JavaScript/TypeScript (regex-based parsing)
- [x] Rust (regex-based parsing)

### Output Format
- [x] Functions/methods: name, signature, parameters, return type, line numbers
- [x] Types/Structs/Classes: name, fields, line numbers
- [x] Interfaces: name, methods, line numbers
- [x] Variables/Constants: name, type, line numbers
- [x] Package/module declarations
- [x] Import statements
- [x] Summary statistics per file

### Tool Parameters
- [x] `path` (string, required): File or directory path to analyze
- [x] `language` (string, optional): Force specific language detection
- [x] `max_depth` (integer, optional): Max directory depth for recursive analysis (default: 5)
- [x] `glob` (string, optional): Glob pattern to filter files
- [x] `include_tests` (boolean, optional): Include test files (default: false)
- [x] `include_private` (boolean, optional): Include private/underscore-prefixed items (default: true)

### Error Handling
- [x] Graceful handling of non-existent paths
- [x] Skip files that can't be read
- [x] Report unsupported file types
- [x] Proper error messages for invalid parameters

## Implementation Notes
- Use Go's `ast` package for Go files (most accurate, no external deps beyond stdlib)
- Use regex-based parsing for Python, JS/TS, Rust
- Output structured JSON for machine readability by the agent
- Directory traversal follows existing `globRecursive` pattern from tools.go
