package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// TodoItem is a single tracked task.
type TodoItem struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Created     string `json:"created"`
	Completed   string `json:"completed,omitempty"`
}

// TodoStore keeps the agent task list.
type TodoStore struct {
	mu     sync.Mutex
	items  []*TodoItem
	nextID int
}

// NewTodoStore creates an empty store.
func NewTodoStore() *TodoStore { return &TodoStore{nextID: 1} }

// Add appends a new pending item.
func (s *TodoStore) Add(description string) *TodoItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := &TodoItem{ID: s.nextID, Description: description, Status: "pending", Created: time.Now().Format(time.RFC3339)}
	s.items = append(s.items, item)
	s.nextID++
	return item
}

// Complete marks an item as completed.
func (s *TodoStore) Complete(id int) (*TodoItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range s.items {
		if it.ID == id {
			it.Status = "completed"
			it.Completed = time.Now().Format(time.RFC3339)
			return it, nil
		}
	}
	return nil, fmt.Errorf("todo item %d not found", id)
}

// Remove deletes an item.
func (s *TodoStore) Remove(id int) (*TodoItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, it := range s.items {
		if it.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return it, nil
		}
	}
	return nil, fmt.Errorf("todo item %d not found", id)
}

// List returns a copy of the items sorted by ID.
func (s *TodoStore) List() []*TodoItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*TodoItem, len(s.items))
	copy(out, s.items)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Render returns a human readable listing.
func (s *TodoStore) Render() string {
	items := s.List()
	if len(items) == 0 {
		return "(no todo items)"
	}
	var b strings.Builder
	for _, it := range items {
		box := "[ ]"
		if it.Status == "completed" {
			box = "[x]"
		}
		b.WriteString(fmt.Sprintf("%d. %s %s\n", it.ID, box, it.Description))
	}
	return strings.TrimRight(b.String(), "\n")
}

// TodoTool manages the task list.
type TodoTool struct {
	store *TodoStore
	run   *RunContext
}

// NewTodoTool creates the todo tool backed by store.
func NewTodoTool(store *TodoStore, run *RunContext) *TodoTool {
	return &TodoTool{store: store, run: run}
}

func (t *TodoTool) Name() string { return "todo" }

func (t *TodoTool) Description() string {
	return "Manage a personal task list for tracking work-in-progress during development"
}

func (t *TodoTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"action": map[string]interface{}{
			"type":        "string",
			"description": "The action to perform (add, complete, remove, or list)",
			"enum":        []string{"add", "complete", "remove", "list"},
		},
		"id":          prop("integer", "Item ID (required for complete/remove)"),
		"description": prop("string", "Task description (required for add)"),
	}, []string{"action"})
}

// ReadOnly reports the todo tool as read-only: in read-only mode only the
// list and remove actions are permitted (see Execute).
func (t *TodoTool) ReadOnly() bool { return true }

func (t *TodoTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action := strings.ToLower(strings.TrimSpace(getString(args, "action")))
	if action == "" {
		return "", errors.New("missing required parameter: action (add, complete, remove or list)")
	}
	if t.run != nil && t.run.ReadOnly && action != "list" && action != "remove" {
		return "", fmt.Errorf("read-only mode only allows the list and remove todo actions")
	}
	switch action {
	case "list":
		return t.store.Render(), nil
	case "add":
		desc := strings.TrimSpace(getString(args, "description"))
		if desc == "" {
			return "", errors.New("missing required parameter: description (required for add)")
		}
		item := t.store.Add(desc)
		return fmt.Sprintf("Added todo item %d: %s", item.ID, item.Description), nil
	case "complete":
		if _, ok := args["id"]; !ok {
			return "", errors.New("missing required parameter: id (required for complete)")
		}
		item, err := t.store.Complete(getInt(args, "id", 0))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Completed todo item %d: %s", item.ID, item.Description), nil
	case "remove":
		if _, ok := args["id"]; !ok {
			return "", errors.New("missing required parameter: id (required for remove)")
		}
		item, err := t.store.Remove(getInt(args, "id", 0))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Removed todo item %d: %s", item.ID, item.Description), nil
	default:
		return "", fmt.Errorf("unknown todo action %q (expected add, complete, remove or list)", action)
	}
}
