package tools

import (
	"context"
	"strings"
	"testing"
)

// ===== Tests for todo tool =====

func TestTodoStore_Add(t *testing.T) {
	store := NewTodoStore()
	id := store.Add("Test item")
	if id != 1 {
		t.Errorf("Expected ID 1, got %d", id)
	}
	if store.CountPending() != 1 {
		t.Errorf("Expected 1 pending item, got %d", store.CountPending())
	}
}

func TestTodoStore_AddMultiple(t *testing.T) {
	store := NewTodoStore()
	id1 := store.Add("First item")
	id2 := store.Add("Second item")
	id3 := store.Add("Third item")

	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Errorf("Expected IDs 1, 2, 3, got %d, %d, %d", id1, id2, id3)
	}
	if store.CountPending() != 3 {
		t.Errorf("Expected 3 pending items, got %d", store.CountPending())
	}
}

func TestTodoStore_Complete(t *testing.T) {
	store := NewTodoStore()
	store.Add("Item 1")
	store.Add("Item 2")

	item := store.Complete(1)
	if item == nil {
		t.Fatal("Expected completed item")
	}
	if !item.Completed {
		t.Error("Expected item to be completed")
	}
	if store.CountCompleted() != 1 {
		t.Errorf("Expected 1 completed item, got %d", store.CountCompleted())
	}
	if store.CountPending() != 1 {
		t.Errorf("Expected 1 pending item, got %d", store.CountPending())
	}
}

func TestTodoStore_CompleteNotFound(t *testing.T) {
	store := NewTodoStore()
	item := store.Complete(999)
	if item != nil {
		t.Error("Expected nil for non-existent item")
	}
}

func TestTodoStore_Remove(t *testing.T) {
	store := NewTodoStore()
	store.Add("Item 1")
	store.Add("Item 2")

	item := store.Remove(1)
	if item == nil {
		t.Fatal("Expected removed item")
	}
	if item.ID != 1 {
		t.Errorf("Expected ID 1, got %d", item.ID)
	}
	if store.CountPending() != 1 {
		t.Errorf("Expected 1 pending item after remove, got %d", store.CountPending())
	}
	if len(store.List()) != 1 {
		t.Errorf("Expected 1 item in list, got %d", len(store.List()))
	}
}

func TestTodoStore_RemoveNotFound(t *testing.T) {
	store := NewTodoStore()
	item := store.Remove(999)
	if item != nil {
		t.Error("Expected nil for non-existent item")
	}
}

func TestTodoStore_List(t *testing.T) {
	store := NewTodoStore()
	store.Add("First")
	store.Add("Second")

	items := store.List()
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
	if items[0].Description != "First" {
		t.Errorf("Expected 'First', got %s", items[0].Description)
	}
}

func TestFormatList_Empty(t *testing.T) {
	result := FormatList([]*TodoItem{})
	if result != "(no todo items)" {
		t.Errorf("Expected '(no todo items)', got %s", result)
	}
}

func TestFormatList_WithItems(t *testing.T) {
	items := []*TodoItem{
		{ID: 1, Description: "First", Completed: false},
		{ID: 2, Description: "Second", Completed: true},
	}
	result := FormatList(items)
	if !strings.Contains(result, "#1 [ ] First") {
		t.Errorf("Expected '#1 [ ] First' in output, got: %s", result)
	}
	if !strings.Contains(result, "#2 [x] Second") {
		t.Errorf("Expected '#2 [x] Second' in output, got: %s", result)
	}
}

func TestExecute_Todo_MissingAction(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name:       "todo",
		Parameters: map[string]interface{}{},
	})
	if result.Success {
		t.Error("Expected failure for missing action")
	}
	if !strings.Contains(result.Error, "missing required parameter") {
		t.Errorf("Expected 'missing required parameter' error, got: %s", result.Error)
	}
}

func TestExecute_Todo_InvalidAction(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "invalid",
		},
	})
	if result.Success {
		t.Error("Expected failure for invalid action")
	}
	if !strings.Contains(result.Error, "invalid action") {
		t.Errorf("Expected 'invalid action' error, got: %s", result.Error)
	}
}

func TestExecute_Todo_Add(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action":      "add",
			"description": "Implement authentication",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "#1") {
		t.Errorf("Expected '#1' in output, got: %s", result.Output)
	}
	if result.Extra == nil || result.Extra["id"] != 1 {
		t.Errorf("Expected id 1 in Extra, got %v", result.Extra)
	}
}

func TestExecute_Todo_AddEmptyDescription(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action":      "add",
			"description": "",
		},
	})
	if result.Success {
		t.Error("Expected failure for empty description")
	}
	if !strings.Contains(result.Error, "description cannot be empty") {
		t.Errorf("Expected 'description cannot be empty' error, got: %s", result.Error)
	}
}

func TestExecute_Todo_AddMissingDescription(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "add",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing description")
	}
	if !strings.Contains(result.Error, "missing required parameter") {
		t.Errorf("Expected 'missing required parameter' error, got: %s", result.Error)
	}
}

func TestExecute_Todo_Complete(t *testing.T) {
	te := NewToolExecutor()
	// First add an item
	te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action":      "add",
			"description": "Test item",
		},
	})

	// Then complete it
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "complete",
			"id":     1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "#1") {
		t.Errorf("Expected '#1' in output, got: %s", result.Output)
	}
}

func TestExecute_Todo_CompleteNotFound(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "complete",
			"id":     999.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for non-existent item")
	}
	if !strings.Contains(result.Error, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", result.Error)
	}
}

func TestExecute_Todo_Remove(t *testing.T) {
	te := NewToolExecutor()
	// First add an item
	te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action":      "add",
			"description": "Item to remove",
		},
	})

	// Then remove it
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "remove",
			"id":     1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "#1") {
		t.Errorf("Expected '#1' in output, got: %s", result.Output)
	}
}

func TestExecute_Todo_RemoveNotFound(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "remove",
			"id":     999.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for non-existent item")
	}
	if !strings.Contains(result.Error, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", result.Error)
	}
}

func TestExecute_Todo_List(t *testing.T) {
	te := NewToolExecutor()
	// Add a couple items
	te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action":      "add",
			"description": "Item 1",
		},
	})
	te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action":      "add",
			"description": "Item 2",
		},
	})

	// List items
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "list",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "#1") {
		t.Errorf("Expected '#1' in output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "#2") {
		t.Errorf("Expected '#2' in output, got: %s", result.Output)
	}
	if result.Extra == nil {
		t.Fatal("Expected Extra map")
	}
	if total, ok := result.Extra["totalItems"].(int); !ok || total != 2 {
		t.Errorf("Expected totalItems 2, got %v", result.Extra["totalItems"])
	}
	if completed, ok := result.Extra["completedItems"].(int); !ok || completed != 0 {
		t.Errorf("Expected completedItems 0, got %v", result.Extra["completedItems"])
	}
	if pending, ok := result.Extra["pendingItems"].(int); !ok || pending != 2 {
		t.Errorf("Expected pendingItems 2, got %v", result.Extra["pendingItems"])
	}
}

func TestExecute_Todo_ListEmpty(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "list",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "(no todo items)") {
		t.Errorf("Expected '(no todo items)' in output, got: %s", result.Output)
	}
}

func TestExecute_Todo_ActionCaseInsensitive(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "LIST",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success for uppercase LIST, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadOnly_TodoListAllowed(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "list",
		},
	})
	if !result.Success {
		t.Fatalf("Expected todo list to succeed in read-only mode, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadOnly_TodoRemoveAllowed(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "remove",
			"id":     1.0,
		},
	})
	// Remove is allowed in read-only mode
	if result.Error != "" && strings.Contains(result.Error, "not available in read-only mode") {
		t.Error("Expected todo remove to be allowed in read-only mode")
	}
}

func TestToolExecutor_ReadOnly_TodoAddBlocked(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action":      "add",
			"description": "Test item",
		},
	})
	if result.Success {
		t.Error("Expected todo add to fail in read-only mode")
	}
	if !strings.Contains(result.Error, "not available in read-only mode") {
		t.Errorf("Expected 'not available in read-only mode' error, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadOnly_TodoCompleteBlocked(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "todo",
		Parameters: map[string]interface{}{
			"action": "complete",
			"id":     1.0,
		},
	})
	if result.Success {
		t.Error("Expected todo complete to fail in read-only mode")
	}
	if !strings.Contains(result.Error, "not available in read-only mode") {
		t.Errorf("Expected 'not available in read-only mode' error, got: %s", result.Error)
	}
}
