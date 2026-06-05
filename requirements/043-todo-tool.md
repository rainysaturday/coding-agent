# Requirement 043: TODO Tool

## Description
The harness must support a `todo` tool that allows the agent to manage a personal task list for tracking work-in-progress during development. The tool supports adding, completing, removing, and listing todo items. Each todo item has a short numeric ID and a description.

## Acceptance Criteria
- [ ] Tool named `todo` is available
- [ ] Supports `add` action to create new todo items
- [ ] Supports `complete` action to mark items as done
- [ ] Supports `remove` action to delete items
- [ ] Supports `list` action to display all items
- [ ] Each item has a short numeric ID and a description
- [ ] Tool works in both normal and read-only modes
- [ ] In read-only mode, only `list` and `remove` actions are allowed (write actions blocked)
- [ ] Failed operations return proper error messages
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "todo",
    "description": "Manage a personal task list for tracking work-in-progress during development",
    "parameters": {
      "type": "object",
      "properties": {
        "action": {
          "type": "string",
          "description": "The action to perform: add, complete, remove, or list"
        },
        "id": {
          "type": "integer",
          "description": "The ID of the todo item (required for complete, remove; not for add or list)"
        },
        "description": {
          "type": "string",
          "description": "The description of the todo item (required for add; not for complete, remove, or list)"
        }
      },
      "required": ["action"]
    }
  }
}
```

## Tool Call Format

### Add a new todo item
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "todo",
    "arguments": "{\"action\":\"add\",\"description\":\"Implement authentication middleware\"}"
  }
}
```

### Complete a todo item
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "todo",
    "arguments": "{\"action\":\"complete\",\"id\":1}"
  }
}
```

### Remove a todo item
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "todo",
    "arguments": "{\"action\":\"remove\",\"id\":2}"
  }
}
```

### List all todo items
```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "todo",
    "arguments": "{\"action\":\"list\"}"
  }
}
```

## Parameters
- `action` (string, required): The action to perform. Must be one of: `add`, `complete`, `remove`, `list`.
- `id` (integer, optional): The ID of a todo item. Required for `complete` and `remove` actions. Not used for `add` or `list`.
- `description` (string, optional): The description of the todo item. Required for `add` action. Not used for `complete`, `remove`, or `list`.

## Todo Item Structure
Each todo item consists of:
- **id**: A short numeric ID (auto-incrementing, starting from 1)
- **description**: A text description of the task
- **completed**: A boolean indicating whether the task is done

## Return Values

### Add Success
On successful add:
- `success`: `true`
- `output`: "Added todo item #N: description"
- Extra: `id` - the assigned ID

### Complete Success
On successful complete:
- `success`: `true`
- `output`: "Completed todo item #N: description"

### Remove Success
On successful remove:
- `success`: `true`
- `output`: "Removed todo item #N: description"

### List Success
On successful list:
- `success`: `true`
- `output`: Formatted list showing all items with their IDs, descriptions, and completion status
- Format:
  ```
  #1 [ ] Description of item 1
  #2 [x] Description of item 2
  #3 [ ] Description of item 3
  ```
- Extra: `totalItems`, `completedItems`, `pendingItems`

### Error Cases
- Missing required parameter: Returns error with message about which parameter is missing
- Invalid action: Returns error with message "invalid action: X (must be one of: add, complete, remove, list)"
- Item not found: Returns error "todo item #X not found"
- Empty description: Returns error "description cannot be empty"

## Read-Only Mode Behavior
In read-only mode, the `todo` tool behaves as follows:
- `list` - Allowed (read operation)
- `remove` - Allowed (this is safe in read-only as it's session-local state)
- `add` - **Blocked** with error: "Tool 'todo' action 'add' is not available in read-only mode"
- `complete` - **Blocked** with error: "Tool 'todo' action 'complete' is not available in read-only mode"

Note: `remove` is allowed in read-only mode because it only affects the in-memory session state, not any external files or system state.

## TUI Feedback
When displaying todo tool results in the TUI:
- The output should show the full list when using the `list` action
- Single-item operations (add, complete, remove) should show a brief confirmation message

## System Prompt Description
```
13. todo
     Description: Manage a personal task list for tracking work-in-progress during development. Supports add, complete, remove, and list actions.
     Parameters:
       - action (string, required): Action to perform (add, complete, remove, or list)
       - id (integer, optional): Item ID (required for complete/remove)
       - description (string, optional): Task description (required for add)
     How to call: Use the todo tool to break down complex tasks into tracked sub-items. This helps you remember what to do between turns.
     Example use case: Creating a checklist for a multi-step refactoring task
```

## Implementation Notes
- Todo items are stored in-memory (in the ToolExecutor struct)
- IDs are auto-incrementing starting from 1
- Items are maintained as a slice, preserving insertion order
- The completed field is a boolean (false by default)
- No persistence is required - todos are session-only
