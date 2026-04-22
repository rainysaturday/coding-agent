package tools

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestParseJSONPath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{".foo", false},
		{".foo.bar", false},
		{".foo.bar[0]", false},
		{".foo.bar[1].baz", false},
		{".items[0].name", false},
		{"", false},
		{".", false},
	}

	for _, tt := range tests {
		result, err := parseJSONPath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseJSONPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
		}
		if result != nil && len(result) == 0 && tt.path != "" && tt.path != "." {
			t.Errorf("parseJSONPath(%q) returned 0 segments", tt.path)
		}
	}
}

func TestResolvePath(t *testing.T) {
	data := map[string]interface{}{
		"name":    "test",
		"version": "1.0.0",
		"database": map[string]interface{}{
			"host": "localhost",
			"port": float64(5432),
		},
		"features": []interface{}{"auth", "api", "docs"},
	}

	tests := []struct {
		path    string
		want    interface{}
		wantErr bool
	}{
		{".name", "test", false},
		{".database.host", "localhost", false},
		{".database.port", float64(5432), false},
		{".features", []interface{}{"auth", "api", "docs"}, false},
		{".nonexistent", nil, true},
	}

	for _, tt := range tests {
		segments, _ := parseJSONPath(tt.path)
		result, err := resolvePath(data, segments)
		if (err != nil) != tt.wantErr {
			t.Errorf("resolvePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			// Use strings for comparison of simple values
			if wantStr, ok := tt.want.(string); ok {
				if gotStr, ok := result.(string); !ok || gotStr != wantStr {
					t.Errorf("resolvePath(%q) = %v (%T), want %v", tt.path, result, result, tt.want)
				}
			} else if wantFloat, ok := tt.want.(float64); ok {
				if gotFloat, ok := result.(float64); !ok || gotFloat != wantFloat {
					t.Errorf("resolvePath(%q) = %v (%T), want %v", tt.path, result, result, tt.want)
				}
			}
		}
	}
}

func TestJsonExtract(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-*.json")
	defer os.Remove(tmpFile.Name())

	jsonData := `{"name": "test", "version": "1.0.0", "database": {"host": "localhost", "port": 5432}}`
	tmpFile.WriteString(jsonData)
	tmpFile.Close()

	executor := NewToolExecutor()

	tests := []struct {
		name        string
		params      map[string]interface{}
		wantSuccess bool
	}{
		{
			name: "extract simple field",
			params: map[string]interface{}{
				"command":   "extract",
				"file_path": tmpFile.Name(),
				"path":      ".name",
			},
			wantSuccess: true,
		},
		{
			name: "extract nested field",
			params: map[string]interface{}{
				"command":   "extract",
				"file_path": tmpFile.Name(),
				"path":      ".database.host",
			},
			wantSuccess: true,
		},
		{
			name: "extract invalid path",
			params: map[string]interface{}{
				"command":   "extract",
				"file_path": tmpFile.Name(),
				"path":      ".nonexistent",
			},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.executeJsonTransformer(tt.params)
			if result.Success != tt.wantSuccess {
				t.Errorf("extract: success = %v, want %v, error: %s", result.Success, tt.wantSuccess, result.Error)
			}
		})
	}
}

func TestJsonSet(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-*.json")
	defer os.Remove(tmpFile.Name())

	jsonData := `{"name": "test", "database": {"host": "localhost"}}`
	tmpFile.WriteString(jsonData)
	tmpFile.Close()

	executor := NewToolExecutor()
	result := executor.executeJsonTransformer(map[string]interface{}{
		"command":   "set",
		"file_path": tmpFile.Name(),
		"path":      ".database.port",
		"value":     "5432",
	})

	if !result.Success {
		t.Errorf("set: unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, `"database"`) {
		t.Error("set: output doesn't contain 'database' key")
	}
}

func TestJsonFormat(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-*.json")
	defer os.Remove(tmpFile.Name())

	jsonData := `{"name":"test","version":"1.0.0"}`
	tmpFile.WriteString(jsonData)
	tmpFile.Close()

	executor := NewToolExecutor()
	result := executor.executeJsonTransformer(map[string]interface{}{
		"command":   "format",
		"file_path": tmpFile.Name(),
		"indent":    4,
	})

	if !result.Success {
		t.Errorf("format: unexpected error: %s", result.Error)
	}
	if len(result.Output) == 0 {
		t.Error("format: empty output")
	}
	// Check that indentation is 4 spaces
	if !strings.Contains(result.Output, "    \"name\"") && !strings.Contains(result.Output, `    "name"`) {
		t.Error("format: output doesn't appear to use 4-space indentation")
	}
}

func TestJsonConvertToYAML(t *testing.T) {
	executor := NewToolExecutor()
	result := executor.executeJsonTransformer(map[string]interface{}{
		"command":     "convert_to_yaml",
		"json_string": `{"name": "test", "version": "1.0.0"}`,
	})

	if !result.Success {
		t.Errorf("convert_to_yaml: unexpected error: %s", result.Error)
	}
	if len(result.Output) == 0 {
		t.Error("convert_to_yaml: empty output")
	}
	// Check for YAML-like content
	if !strings.Contains(result.Output, "name:") {
		t.Error("convert_to_yaml: output doesn't look like YAML")
	}
}

func TestJsonConvertToEnv(t *testing.T) {
	executor := NewToolExecutor()
	result := executor.executeJsonTransformer(map[string]interface{}{
		"command":     "convert_to_env",
		"json_string": `{"name": "test", "port": 8080, "debug": true}`,
	})

	if !result.Success {
		t.Errorf("convert_to_env: unexpected error: %s", result.Error)
	}
	if len(result.Output) == 0 {
		t.Error("convert_to_env: empty output")
	}
	// Check for env-like content
	if !strings.Contains(result.Output, "NAME=") {
		t.Error("convert_to_env: output doesn't look like env vars")
	}
}

func TestJsonValidate(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-*.json")
	defer os.Remove(tmpFile.Name())

	jsonData := `{"name": "test", "version": "1.0.0", "database": {"host": "localhost"}}`
	tmpFile.WriteString(jsonData)
	tmpFile.Close()

	executor := NewToolExecutor()

	result := executor.executeJsonTransformer(map[string]interface{}{
		"command":        "validate",
		"file_path":      tmpFile.Name(),
		"required_fields": []interface{}{".name", ".version", ".database.host", ".missing"},
	})

	if !result.Success {
		t.Errorf("validate: unexpected error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "FAILED") {
		t.Error("validate: expected FAILED since .missing field doesn't exist")
	}
}

func TestMergeJSON(t *testing.T) {
	file1, _ := os.CreateTemp("", "merge1-*.json")
	defer os.Remove(file1.Name())
	file1.WriteString(`{"name": "app", "version": "1.0.0"}`)
	file1.Close()

	file2, _ := os.CreateTemp("", "merge2-*.json")
	defer os.Remove(file2.Name())
	file2.WriteString(`{"database": {"host": "localhost"}, "debug": true}`)
	file2.Close()

	executor := NewToolExecutor()
	result := executor.executeJsonTransformer(map[string]interface{}{
		"command":   "merge",
		"file_path": file1.Name(),
		"files":     []interface{}{file2.Name()},
	})

	if !result.Success {
		t.Errorf("merge: unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("merge: empty output")
	}

	// Verify the merged output contains all keys
	var merged map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &merged); err == nil {
		if _, ok := merged["name"]; !ok {
			t.Error("merge: missing 'name' key")
		}
		if _, ok := merged["database"]; !ok {
			t.Error("merge: missing 'database' key")
		}
		if _, ok := merged["debug"]; !ok {
			t.Error("merge: missing 'debug' key")
		}
	}
}

func TestJsonExtractJSONString(t *testing.T) {
	executor := NewToolExecutor()
	result := executor.executeJsonTransformer(map[string]interface{}{
		"command":     "extract",
		"json_string": `{"name": "test", "items": [{"id": 1}, {"id": 2}]}`,
		"path":        ".name",
	})

	if !result.Success {
		t.Errorf("extract with json_string: unexpected error: %s", result.Error)
	}
	if result.Output != "test" {
		t.Errorf("extract: output = %q, want %q", result.Output, "test")
	}
}

func TestJsonMergeWithJSONStrings(t *testing.T) {
	executor := NewToolExecutor()
	result := executor.executeJsonTransformer(map[string]interface{}{
		"command":      "merge",
		"json_string":  `{"name": "app"}`,
		"json_strings": []interface{}{`{"version": "1.0.0"}`},
	})

	if !result.Success {
		t.Errorf("merge with json_strings: unexpected error: %s", result.Error)
	}

	var merged map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &merged); err == nil {
		if _, ok := merged["name"]; !ok {
			t.Error("merge: missing 'name' key")
		}
		if _, ok := merged["version"]; !ok {
			t.Error("merge: missing 'version' key")
		}
	}
}
