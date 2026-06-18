// Package tools implements the tool execution system for the coding agent.
// This file contains the todo store and todo tool executor.
package tools

import (
	"fmt"
	"strings"
)

// TodoItem represents a single todo item in the task list.
type TodoItem struct {
	ID          int
	Description string
	Completed   bool
}

// TodoStore manages a collection of todo items.
type TodoStore struct {
	items  []*TodoItem
	nextID int
}

// NewTodoStore creates a new empty todo store.
func NewTodoStore() *TodoStore {
	return &TodoStore{
		items:  make([]*TodoItem, 0),
		nextID: 1,
	}
}

// Add creates a new todo item with the given description.
// Returns the ID of the newly created item.
func (ts *TodoStore) Add(description string) int {
	item := &TodoItem{
		ID:          ts.nextID,
		Description: description,
		Completed:   false,
	}
	ts.items = append(ts.items, item)
	id := ts.nextID
	ts.nextID++
	return id
}

// Complete marks a todo item as done by ID.
// Returns the completed item, or nil if not found.
func (ts *TodoStore) Complete(id int) *TodoItem {
	for _, item := range ts.items {
		if item.ID == id {
			item.Completed = true
			return item
		}
	}
	return nil
}

// Remove deletes a todo item by ID.
// Returns the removed item, or nil if not found.
func (ts *TodoStore) Remove(id int) *TodoItem {
	for i, item := range ts.items {
		if item.ID == id {
			ts.items = append(ts.items[:i], ts.items[i+1:]...)
			return item
		}
	}
	return nil
}

// List returns all todo items.
func (ts *TodoStore) List() []*TodoItem {
	return ts.items
}

// CountPending returns the number of non-completed items.
func (ts *TodoStore) CountPending() int {
	count := 0
	for _, item := range ts.items {
		if !item.Completed {
			count++
		}
	}
	return count
}

// CountCompleted returns the number of completed items.
func (ts *TodoStore) CountCompleted() int {
	count := 0
	for _, item := range ts.items {
		if item.Completed {
			count++
		}
	}
	return count
}

// FormatList formats all todo items as a readable string.
func FormatList(items []*TodoItem) string {
	if len(items) == 0 {
		return "(no todo items)"
	}

	var lines []string
	for _, item := range items {
		status := "[ ]"
		if item.Completed {
			status = "[x]"
		}
		lines = append(lines, fmt.Sprintf("#%d %s %s", item.ID, status, item.Description))
	}

	return strings.Join(lines, "\n")
}

// executeTodo manages a personal task list for tracking work-in-progress.
func (te *ToolExecutor) executeTodo(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action",
		}
	}

	action = strings.ToLower(strings.TrimSpace(action))

	switch action {
	case "add":
		return te.executeTodoAdd(params)
	case "complete":
		return te.executeTodoComplete(params)
	case "remove":
		return te.executeTodoRemove(params)
	case "list":
		return te.executeTodoList()
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid action: %s (must be one of: add, complete, remove, list)", action),
		}
	}
}

// executeTodoAdd creates a new todo item.
func (te *ToolExecutor) executeTodoAdd(params map[string]interface{}) *ToolResult {
	description, ok := params["description"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: description",
		}
	}

	description = strings.TrimSpace(description)
	if description == "" {
		return &ToolResult{
			Success: false,
			Error:   "description cannot be empty",
		}
	}

	id := te.todoStore.Add(description)

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Added todo item #%d: %s", id, description),
		Extra: map[string]interface{}{
			"id": id,
		},
	}
}

// executeTodoComplete marks a todo item as done.
func (te *ToolExecutor) executeTodoComplete(params map[string]interface{}) *ToolResult {
	idVal, ok := params["id"]
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: id",
		}
	}

	var id int
	switch v := idVal.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	default:
		return &ToolResult{
			Success: false,
			Error:   "id must be an integer",
		}
	}

	item := te.todoStore.Complete(id)
	if item == nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("todo item #%d not found", id),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Completed todo item #%d: %s", item.ID, item.Description),
	}
}

// executeTodoRemove deletes a todo item.
func (te *ToolExecutor) executeTodoRemove(params map[string]interface{}) *ToolResult {
	idVal, ok := params["id"]
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: id",
		}
	}

	var id int
	switch v := idVal.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	default:
		return &ToolResult{
			Success: false,
			Error:   "id must be an integer",
		}
	}

	item := te.todoStore.Remove(id)
	if item == nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("todo item #%d not found", id),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Removed todo item #%d: %s", item.ID, item.Description),
	}
}

// executeTodoList returns all todo items.
func (te *ToolExecutor) executeTodoList() *ToolResult {
	items := te.todoStore.List()
	output := FormatList(items)

	total := len(items)
	completed := te.todoStore.CountCompleted()
	pending := te.todoStore.CountPending()

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"totalItems":     total,
			"completedItems": completed,
			"pendingItems":   pending,
		},
	}
}
