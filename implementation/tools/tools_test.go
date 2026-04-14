package tools

import (
	"testing"
)

func TestParseToolCall(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ToolCall
		wantErr bool
	}{
		{
			name:  "valid bash command",
			input: `{"id":"call_abc123","type":"function","function":{"name":"bash","arguments":"{\"command\":\"ls -la\"}"}}`,
			want: &ToolCall{
				ID:   "call_abc123",
				Name: "bash",
				Parameters: map[string]interface{}{
					"command": "ls -la",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid read_file",
			input: `{"id":"call_def456","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"/test/file.txt\"}"}}`,
			want: &ToolCall{
				ID:   "call_def456",
				Name: "read_file",
				Parameters: map[string]interface{}{
					"path": "/test/file.txt",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid write_file",
			input: `{"id":"call_ghi789","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"/test/file.txt\",\"content\":\"hello\"}"}}`,
			want: &ToolCall{
				ID:   "call_ghi789",
				Name: "write_file",
				Parameters: map[string]interface{}{
					"path":    "/test/file.txt",
					"content": "hello",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid read_lines",
			input: `{"id":"call_jkl012","type":"function","function":{"name":"read_lines","arguments":"{\"path\":\"file.txt\",\"start\":1,\"end\":10}"}}`,
			want: &ToolCall{
				ID:   "call_jkl012",
				Name: "read_lines",
				Parameters: map[string]interface{}{
					"path":  "file.txt",
					"start": 1.0,
					"end":   10.0,
				},
			},
			wantErr: false,
		},
		{
			name:  "valid insert_lines",
			input: `{"id":"call_mno345","type":"function","function":{"name":"insert_lines","arguments":"{\"path\":\"file.txt\",\"line\":5,\"lines\":\"new line\"}"}}`,
			want: &ToolCall{
				ID:   "call_mno345",
				Name: "insert_lines",
				Parameters: map[string]interface{}{
					"path":  "file.txt",
					"line":  5.0,
					"lines": "new line",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid replace_text",
			input: `{"id":"call_pqr678","type":"function","function":{"name":"replace_text","arguments":"{\"path\":\"file.txt\",\"search\":\"old\",\"replace\":\"new\"}"}}`,
			want: &ToolCall{
				ID:   "call_pqr678",
				Name: "replace_text",
				Parameters: map[string]interface{}{
					"path":    "file.txt",
					"search":  "old",
					"replace": "new",
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "missing name",
			input:   `{"id":"call_xyz","type":"function","function":{"arguments":"{}"}}`,
			wantErr: true,
		},
		{
			name:    "empty tool call",
			input:   `{}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseToolCall(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseToolCall() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.want != nil {
				if result.Name != tt.want.Name {
					t.Errorf("ParseToolCall() name = %v, want %v", result.Name, tt.want.Name)
				}
				if result.ID != tt.want.ID {
					t.Errorf("ParseToolCall() ID = %v, want %v", result.ID, tt.want.ID)
				}
				for k, v := range tt.want.Parameters {
					if result.Parameters[k] != v {
						t.Errorf("ParseToolCall() parameter %v = %v, want %v", k, result.Parameters[k], v)
					}
				}
			}
		})
	}
}
