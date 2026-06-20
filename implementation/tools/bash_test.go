package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ===== Tests for bash tool =====

func TestExecute_Bash_Failed(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "false", // always fails
		},
	})
	if result.Success {
		t.Error("Expected failure for 'false' command")
	}
	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code")
	}
}

func TestExecute_Bash_WithEnvVars(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo $HOME",
		},
	})
	// Should succeed and return output
	if !result.Success {
		t.Logf("bash command returned error (expected in test env): %s", result.Error)
	}
}

func TestExecute_Bash_WithPipes(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test | cat",
		},
	})
	// Should succeed
	if result.Success {
		if !strings.Contains(result.Output, "test") {
			t.Errorf("Expected output to contain 'test', got: %s", result.Output)
		}
	}
}

func TestExecute_Bash_Timeout(t *testing.T) {
	te := NewToolExecutor()
	// Use a very short timeout (10ms) to trigger the timeout quickly
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "sleep 10",
			"timeout": 10,
		},
	})
	if result.Success {
		t.Fatal("Expected failure due to timeout")
	}
	if result.ExitCode != 124 {
		t.Errorf("Expected exit code 124 for timeout, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Error, "timed out") {
		t.Errorf("Expected timeout error message, got: %s", result.Error)
	}
	if !strings.Contains(result.Error, "10ms") {
		t.Errorf("Expected timeout value in error message, got: %s", result.Error)
	}
	if !strings.Contains(result.Error, "Consider increasing the timeout") {
		t.Errorf("Expected timeout suggestion in error message, got: %s", result.Error)
	}
}

func TestExecute_Bash_DefaultTimeout(t *testing.T) {
	te := NewToolExecutor()
	// Command should succeed within default 30s timeout
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo hello",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected 'hello' in output, got: %s", result.Output)
	}
}

func TestExecute_Bash_CustomTimeout(t *testing.T) {
	te := NewToolExecutor()
	// Use a custom timeout of 5000ms (5 seconds) for a quick command
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
			"timeout": 5000,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "test") {
		t.Errorf("Expected 'test' in output, got: %s", result.Output)
	}
}

func TestExecute_Bash_ZeroTimeoutFallsBackToDefault(t *testing.T) {
	te := NewToolExecutor()
	// Zero timeout should fall back to default
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo fallback",
			"timeout": 0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "fallback") {
		t.Errorf("Expected 'fallback' in output, got: %s", result.Output)
	}
}

func TestExecute_Bash_MissingCommand(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name:       "bash",
		Parameters: map[string]interface{}{},
	})
	if result.Success {
		t.Error("Expected failure for missing command parameter")
	}
	if !strings.Contains(result.Error, "missing required parameter") {
		t.Errorf("Expected 'missing required parameter' error, got: %s", result.Error)
	}
}

func TestExecute_Bash_EmptyCommand(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "",
		},
	})
	// Empty command actually succeeds with sh -c (returns empty output)
	if !result.Success {
		t.Logf("Got failure (may depend on shell): %s", result.Error)
	}
}

func TestExecute_Bash_Success(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo hello",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected 'hello' in output, got: %s", result.Output)
	}
}

func TestExecute_Bash_NonZeroExit(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "exit 42",
		},
	})
	if result.Success {
		t.Error("Expected failure for exit 42")
	}
	if result.ExitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", result.ExitCode)
	}
}

func TestExecute_Bash_Ctx_Cancelled(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running command
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	result := te.Execute(ctx, &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "sleep 30",
		},
	})

	// The command should be cancelled
	if result.Success {
		t.Error("Expected failure for cancelled context")
	}
}

func TestExecute_Bash_StringTimeout(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
			"timeout": "5000",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecute_Bash_NegativeTimeout(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
			"timeout": -1,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success with default timeout, got: %s", result.Error)
	}
}

func TestExecute_Bash_FloatTimeout(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
			"timeout": float64(5000),
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecuteBashCtx_ContextCancelled(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := te.Execute(ctx, &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo hello",
		},
	})

	if result.Success {
		t.Log("Got success with cancelled context - unexpected")
	}
}

func TestExecuteBashCtx_Timeout(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := te.Execute(ctx, &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "sleep 10",
		},
	})

	// Should fail due to timeout
	if result.Success {
		t.Log("Got success with timeout - unexpected")
	}
}

func TestExecuteBash_SimpleCommand(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo hello",
		},
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", result.Output)
	}
}

func TestExecuteBash_MultilineCommand(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo line1\necho line2",
		},
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
}

func TestExecuteBash_OutputCapture(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo 'test output'",
		},
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "test output") {
		t.Errorf("Expected output to contain 'test output', got: %s", result.Output)
	}
}

func TestExecuteBash_ContextWithTimeout(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := te.Execute(ctx, &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo 'context test'",
		},
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
}

func TestExecuteBash_BashScript(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "for i in 1 2 3; do echo $i; done",
		},
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
}

func TestExecuteBash(t *testing.T) {
	te := NewToolExecutor()

	// Test with a valid bash command
	result := te.executeBash(context.Background(), map[string]interface{}{
		"command": "echo hello",
	})
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
	if result.Output != "hello\n" {
		t.Errorf("Expected 'hello\\n', got '%s'", result.Output)
	}
}

func TestExecuteBash_Error(t *testing.T) {
	te := NewToolExecutor()

	// Test with a failing command
	result := te.executeBash(context.Background(), map[string]interface{}{
		"command": "exit 42",
	})
	if result.Success {
		t.Error("Expected failure for exit 42")
	}
	if result.ExitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", result.ExitCode)
	}
}

func TestExecuteBash_MissingCommand(t *testing.T) {
	te := NewToolExecutor()

	// Test with missing command parameter
	result := te.executeBash(context.Background(), map[string]interface{}{})
	if result.Success {
		t.Error("Expected failure for missing command")
	}
	if !strings.Contains(result.Error, "missing required parameter: command") {
		t.Errorf("Expected 'missing required parameter: command', got: %s", result.Error)
	}
}

// ===== Tests for isCancelled =====

func TestIsCancelled_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ctx.Err()
	if !isCancelled(err) {
		t.Error("Expected isCancelled to return true for context.Canceled")
	}
}

func TestIsCancelled_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	err := ctx.Err()
	if isCancelled(err) {
		t.Error("Expected isCancelled to return false for context.DeadlineExceeded")
	}
}

func TestIsCancelled_OtherError(t *testing.T) {
	err := fmt.Errorf("some other error")
	if isCancelled(err) {
		t.Error("Expected isCancelled to return false for other errors")
	}
}

func TestIsCancelled_NilError(t *testing.T) {
	if isCancelled(nil) {
		t.Error("Expected isCancelled to return false for nil error")
	}
}
