# Feature #055: Process Management Tool

## Description
A tool for managing running processes and checking system resource usage.

## Features
- `process_list` - List running processes with optional filtering by name/regex, user, or PID
- `process_kill` - Kill a process by PID or name
- `port_check` - Check if a specific TCP/UDP port is in use
- `system_info` - Show system resource usage (CPU, memory, disk)

## Parameters

### process_list
- `filter` (optional): Regex pattern to filter process names
- `user` (optional): Filter by username
- `limit` (optional): Max number of results (default: 50)
- `sort` (optional): Sort by "cpu", "memory", or "pid" (default: "pid")

### process_kill
- `pid` (optional): Process ID to kill
- `name` (optional): Process name to kill (kills first match)
- `force` (optional): Use SIGKILL instead of SIGTERM (default: false)

### port_check
- `port` (required): Port number to check
- `protocol` (optional): "tcp" or "udp" (default: "tcp")

### system_info
- `format` (optional): "short" (default) or "detailed"

## Implementation Notes
- Use only Go stdlib (no external dependencies)
- On Linux: read from /proc filesystem
- On macOS: use sysctl or vm_stat
- On Windows: use wmi (if available) or fall back to command-line tools
