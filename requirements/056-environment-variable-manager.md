# Feature #056: Environment Variable Manager Tool

## Description
A tool for reading and managing environment variables.

## Actions
- `get` - Read a specific environment variable by name
- `set` - Set an environment variable for the current process
- `unset` - Unset an environment variable
- `list` - List environment variables with optional prefix filtering
- `source` - Source a .env file and load its variables

## Parameters

### get
- `name` (required): Environment variable name

### set
- `name` (required): Environment variable name
- `value` (required): Value to set

### unset
- `name` (required): Environment variable name to remove

### list
- `prefix` (optional): Filter variables by prefix
- `show_all` (optional): Include unset/empty variables (default: false, skip empty)

### source
- `path` (required): Path to .env file
- `overwrite` (optional): Overwrite existing variables (default: false)

## Acceptance Criteria
- [ ] Tool registered in agent.go with correct tool definition
- [ ] All five actions work correctly (get, set, unset, list, source)
- [ ] `get` returns value or "not set" message
- [ ] `set` persists for the current process
- [ ] `unset` removes the variable
- [ ] `list` with no prefix shows all non-empty vars
- [ ] `list` with prefix filters correctly
- [ ] `source` parses .env format (KEY=VALUE, comments #, quoted values)
- [ ] `source` with overwrite=false skips existing vars
- [ ] `source` with overwrite=true overwrites existing vars
- [ ] Uses only Go stdlib (zero external dependencies)
- [ ] Tool appears in system prompt with proper documentation
