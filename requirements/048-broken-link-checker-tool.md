# #048: Broken Link Checker Tool

## Description
A tool that scans Markdown and HTML files for broken links - both internal file links (relative paths) and HTTP/HTTPS URLs.

## Acceptance Criteria

- [ ] Tool is named `check_links` and registered in the agent's tool set
- [ ] Accepts optional `paths` parameter to restrict search to specific files/directories (glob patterns). If omitted, searches all files in the current directory recursively.
- [ ] Accepts optional `file_types` parameter to limit which file extensions to scan (default: `.md`, `.html`, `.htm`). Can be a single string or array of strings.
- [ ] Accepts optional `root_dir` parameter as the base directory for resolving relative links (default: current directory)
- [ ] Detects internal links (relative paths) and validates they point to existing files
- [ ] Detects external links (http://, https:// URLs) and performs HEAD requests to verify they return 2xx status
- [ ] Accepts optional `timeout` parameter for external link verification (default: 10 seconds)
- [ ] Returns structured results including:
  - `total_links_found` - total number of links found
  - `total_files_scanned` - number of files scanned
  - `internal_links_ok` - count of valid internal links
  - `internal_links_broken` - count of broken internal links
  - `external_links_ok` - count of valid external links
  - `external_links_broken` - count of broken external links
  - `broken_links` - list of broken links with file, line, type, link text, and link value
  - `status_code` - HTTP status code for broken external links (when available)
- [ ] Output is structured and concise, summarizing findings with details for broken links only
- [ ] Works in non-interactive mode (no TUI required)
- [ ] Is added to the system prompt with proper tool calling format

## Link Detection Patterns

### Internal Links
- Markdown: `[text](path/to/file)`, `[text](path/to/file#anchor)`, `![alt](path/to/image)`
- HTML: `<a href="path/to/file">`, `<img src="path/to/image">`

### External Links
- `[text](https://example.com)`
- `<a href="https://example.com">`

## Notes
- Internal links should be resolved relative to the file's directory
- Anchor fragments in internal links are checked against the file (not validated for content)
- External link verification uses HEAD request; falls back to GET if HEAD fails
- Rate limiting: limit concurrent external link checks to 5 simultaneous requests
