package tui

import "testing"

func TestColorConstants(t *testing.T) {
	if ColorDim == "" || ColorReset == "" {
		t.Fatal("expected ANSI color constants")
	}
}
