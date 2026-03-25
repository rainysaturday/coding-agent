# Requirement 011: Read Lines Tool

## Description
The harness must support a `read_lines` tool that allows reading a specific line range from a file.

## Acceptance Criteria
- [ ] Tool named `read_lines` is available
- [ ] Accepts file path as input parameter
- [ ] Accepts start line number as input parameter (1-indexed)
- [ ] Accepts end line number as input parameter (1-indexed)
- [ ] Returns only the specified line range
- [ ] Handles start > end by returning empty result or error
- [ ] Handles start line beyond file end gracefully
- [ ] Handles end line beyond file end by reading to end of file
- [ ] Handles file not found errors gracefully
- [ ] Handles permission errors gracefully
- [ ] Returns line numbers with content for reference
- [ ] Tool call failures are tracked in statistics
