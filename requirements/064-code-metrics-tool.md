# Feature #064: Code Metrics Analysis Tool

## Description
A tool that analyzes source code files and generates quantitative metrics including lines of code breakdown, cyclomatic complexity, function/method counts, and language-specific statistics.

## Acceptance Criteria

- [ ] Tool named `code_metrics` accessible via tool execution
- [ ] Auto-detects programming language from file extension
- [ ] Supports languages: Go (.go), Python (.py), JavaScript (.js), TypeScript (.ts, .tsx), Java (.java), Rust (.rs), C/C++ (.c, .h, .cpp, .hpp)
- [ ] Counts lines of code: total, blank, comment, code (non-blank, non-comment)
- [ ] Computes approximate cyclomatic complexity per function
- [ ] Detects functions/methods per language with line numbers
- [ ] Returns structured JSON output with per-file and aggregate metrics
- [ ] Supports `path` parameter (file or directory)
- [ ] Supports `language` parameter to force a specific language
- [ ] Supports `max_depth` parameter for recursive directory scanning
- [ ] Supports `glob` parameter for file pattern filtering
- [ ] Uses only Go stdlib - zero external dependencies
- [ ] Returns empty metrics gracefully for non-source files
- [ ] Handles missing files/directories with appropriate error messages
- [ ] Tool definition registered in system prompt

## Tool Definition
```json
{
  "name": "code_metrics",
  "description": "Analyze source code files for metrics: lines of code (total/blank/comment/code), cyclomatic complexity, function/method counts, and language breakdown. Supports Go, Python, JavaScript/TypeScript, Java, Rust, and C/C++.",
  "parameters": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "File or directory path to analyze"
      },
      "language": {
        "type": "string",
        "description": "Force language detection (e.g., 'go', 'python', 'javascript'). Auto-detected from extension if not specified."
      },
      "max_depth": {
        "type": "integer",
        "description": "Maximum directory recursion depth (default: 5, use 0 for current file only)"
      },
      "glob": {
        "type": "string",
        "description": "Glob pattern to filter files (e.g., '*.go', 'src/**/*.py')"
      }
    },
    "required": ["path"]
  }
}
```

## Expected Output Format
```json
{
  "summary": {
    "total_files": 10,
    "total_lines": 5000,
    "total_code_lines": 3500,
    "total_blank_lines": 800,
    "total_comment_lines": 700,
    "total_functions": 45,
    "avg_complexity": 3.2,
    "max_complexity": 12,
    "languages": {
      "Go": 6,
      "Python": 4
    }
  },
  "files": [
    {
      "path": "main.go",
      "language": "Go",
      "lines": {
        "total": 500,
        "code": 350,
        "blank": 80,
        "comment": 70
      },
      "functions": [
        {
          "name": "main",
          "line": 1,
          "complexity": 2
        }
      ],
      "complexity": 2
    }
  ]
}
```

## Complexity Calculation
- Base complexity: 1
- Add 1 for each: `if`, `else if`, `for`, `while`, `switch`, `case`, `&&`, `||`, `?:`, `catch`, `except`
- This is an approximation; full control flow analysis is not required

## Implementation Notes
- Use regex for function detection per language
- Use simple comment detection (line starts with `//`, `#`, `/*`, `*`, `//`, etc.)
- Binary files should be skipped
- Empty files should report 0 for all metrics
- Non-source files should be silently skipped or reported with a warning
