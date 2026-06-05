// Package tools implements the tool execution system for the coding agent.
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
