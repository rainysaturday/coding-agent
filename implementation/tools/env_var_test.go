package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func getInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	default:
		return -1
	}
}

func TestEnvVarGet(t *testing.T) {
	te := NewToolExecutor()

	// Test setting and getting a variable
	os.Setenv("TEST_AGENT_VAR_056", "hello_world")
	result := te.executeEnvVar(map[string]interface{}{
		"action": "get",
		"name":   "TEST_AGENT_VAR_056",
	})

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Error)
	}
	if result.Path != "TEST_AGENT_VAR_056" {
		t.Errorf("expected path 'TEST_AGENT_VAR_056', got: %s", result.Path)
	}

	// Test getting unset variable
	result = te.executeEnvVar(map[string]interface{}{
		"action": "get",
		"name":   "NONEXISTENT_VAR_056",
	})

	if !result.Success {
		t.Errorf("expected success for unset var, got: %s", result.Error)
	}

	// Test missing name
	result = te.executeEnvVar(map[string]interface{}{
		"action": "get",
	})

	if result.Success {
		t.Error("expected failure for missing name")
	}

	// Cleanup
	os.Unsetenv("TEST_AGENT_VAR_056")
}

func TestEnvVarSet(t *testing.T) {
	te := NewToolExecutor()

	// Test setting a variable
	result := te.executeEnvVar(map[string]interface{}{
		"action":  "set",
		"name":    "TEST_SET_VAR_056",
		"value":   "test_value",
	})

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Error)
	}

	// Verify it was actually set
	if got := os.Getenv("TEST_SET_VAR_056"); got != "test_value" {
		t.Errorf("expected 'test_value', got: '%s'", got)
	}

	// Test missing parameters
	result = te.executeEnvVar(map[string]interface{}{
		"action": "set",
		"name":   "TEST_SET_VAR_056",
	})

	if result.Success {
		t.Error("expected failure for missing value")
	}

	// Cleanup
	os.Unsetenv("TEST_SET_VAR_056")
}

func TestEnvVarUnset(t *testing.T) {
	te := NewToolExecutor()

	// Set a variable first
	os.Setenv("TEST_UNSET_VAR_056", "to_remove")

	// Unset it
	result := te.executeEnvVar(map[string]interface{}{
		"action": "unset",
		"name":   "TEST_UNSET_VAR_056",
	})

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Error)
	}

	// Verify it's gone
	if got := os.Getenv("TEST_UNSET_VAR_056"); got != "" {
		t.Errorf("expected empty, got: '%s'", got)
	}

	// Test missing name
	result = te.executeEnvVar(map[string]interface{}{
		"action": "unset",
	})

	if result.Success {
		t.Error("expected failure for missing name")
	}
}

func TestEnvVarList(t *testing.T) {
	te := NewToolExecutor()

	// Set some test variables
	os.Setenv("TEST_LIST_PREFIX_056", "value1")
	os.Setenv("TEST_LIST_ANOTHER_056", "value2")
	os.Setenv("OTHER_VAR_056", "different")

	// Test listing with prefix
	result := te.executeEnvVar(map[string]interface{}{
		"action": "list",
		"prefix": "TEST_LIST_PREFIX_056",
	})

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Error)
	}
	if getInt(result.Extra["count"]) != 1 {
		t.Errorf("expected 1 result with prefix, got: %v", result.Extra["count"])
	}

	// Test listing without prefix (should include our test vars)
	result = te.executeEnvVar(map[string]interface{}{
		"action": "list",
	})

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Error)
	}

	// Cleanup
	os.Unsetenv("TEST_LIST_PREFIX_056")
	os.Unsetenv("TEST_LIST_ANOTHER_056")
	os.Unsetenv("OTHER_VAR_056")
}

func TestEnvVarSource(t *testing.T) {
	te := NewToolExecutor()

	// Ensure clean state for this test
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "EMPTY_VAR", "API_KEY"} {
		os.Unsetenv(key)
	}

	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	envContent := `# This is a comment
DB_HOST=localhost
DB_PORT=5432
DB_USER="admin"
DB_PASS='secret123'
EMPTY_VAR=
# Another comment
API_KEY=abc123`

	err := os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test env file: %v", err)
	}

	// Test sourcing with overwrite=false (default)
	// First set DB_HOST to something else
	os.Setenv("DB_HOST", "production.example.com")

	result := te.executeEnvVar(map[string]interface{}{
		"action":    "source",
		"path":      envFile,
		"overwrite": false,
	})

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Error)
	}
	if getInt(result.Extra["loaded"]) != 5 { // DB_HOST should be skipped, EMPTY_VAR loads with empty value
		t.Errorf("expected 5 loaded (DB_HOST skipped, EMPTY_VAR loaded), got: %v", result.Extra["loaded"])
	}

	// Verify DB_HOST wasn't overwritten
	if got := os.Getenv("DB_HOST"); got != "production.example.com" {
		t.Errorf("expected 'production.example.com' (not overwritten), got: '%s'", got)
	}

	// Verify other vars were set
	if got := os.Getenv("DB_PORT"); got != "5432" {
		t.Errorf("expected '5432', got: '%s'", got)
	}

	// Cleanup
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASS")
	os.Unsetenv("EMPTY_VAR")
	os.Unsetenv("API_KEY")

	// Test sourcing with overwrite=true
	os.Setenv("DB_HOST", "production.example.com")

	result = te.executeEnvVar(map[string]interface{}{
		"action":    "source",
		"path":      envFile,
		"overwrite": true,
	})

	if !result.Success {
		t.Errorf("expected success, got: %s", result.Error)
	}
	if getInt(result.Extra["loaded"]) != 6 { // All should be loaded (including DB_HOST overwritten)
		t.Errorf("expected 6 loaded (all overwritten including DB_HOST), got: %v", result.Extra["loaded"])
	}

	// Verify DB_HOST was overwritten
	if got := os.Getenv("DB_HOST"); got != "localhost" {
		t.Errorf("expected 'localhost' (was overwritten), got: '%s'", got)
	}

	// Cleanup
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASS")
	os.Unsetenv("EMPTY_VAR")
	os.Unsetenv("API_KEY")
}

func TestEnvVarSourceMissingFile(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeEnvVar(map[string]interface{}{
		"action": "source",
		"path":   "/nonexistent/path/.env",
	})

	if result.Success {
		t.Error("expected failure for missing file")
	}
}

func TestEnvVarActionValidation(t *testing.T) {
	te := NewToolExecutor()

	// Test missing action
	result := te.executeEnvVar(map[string]interface{}{})
	if result.Success {
		t.Error("expected failure for missing action")
	}

	// Test invalid action
	result = te.executeEnvVar(map[string]interface{}{
		"action": "invalid",
	})
	if result.Success {
		t.Error("expected failure for invalid action")
	}
}
