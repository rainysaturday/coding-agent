# Requirement 030: Patch Tool (DEPRECATED)

**Status: DEPRECATED — Tool removed.**

The `patch` tool is no longer part of the coding agent. It was removed because it proved difficult for LLMs to use reliably. The simpler `replace_text` tool (requirement 013) is the recommended replacement for text modifications.

This file is kept for historical reference only. Do not implement or test this tool.

## Superseded By

- **013-replace-text-tool.md**: Use `replace_text` for find-and-replace operations
- **011-read-lines-tool.md**: Use `read_lines` to inspect content before editing
- **006-write-file-tool.md**: Use `write_file` for full file overwrites
