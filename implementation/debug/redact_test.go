package debug

import (
	"strings"
	"testing"
)

func TestRedactHeader(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantHidden string // substring that must NOT appear
		wantPart   string // substring that must appear
	}{
		{name: "bearer", value: "Bearer sk-proj-abcdefghijklmnopqrstuvwxyz0123456789", wantHidden: "sk-proj-abcdefghijklmnopqrstuvwxyz0123456789", wantPart: "Bearer [REDACTED"},
		{name: "plain", value: "ghp_abcdefghijklmnopqrstuvwxyz0123", wantHidden: "ghp_abcdefghijklmnopqrstuvwxyz0123", wantPart: "[REDACTED len=34]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactHeader("Authorization", tc.value)
			if strings.Contains(got, tc.wantHidden) {
				t.Errorf("RedactHeader leaked credential: %q", got)
			}
			if !strings.Contains(got, tc.wantPart) {
				t.Errorf("RedactHeader(%q) = %q, want it to contain %q", tc.value, got, tc.wantPart)
			}
		})
	}

	// Case-insensitive header matching.
	for _, name := range []string{"authorization", "Authorization", "AUTHORIZATION", " X-Api-Key "} {
		got := RedactHeader(name, "sk-abcdefghijklmnopqrstuvwxyz")
		if strings.Contains(got, "sk-abcdefghijklmnopqrstuvwxyz") {
			t.Errorf("RedactHeader(%q) leaked credential: %q", name, got)
		}
	}

	// Non-sensitive headers must pass through untouched, otherwise verbose
	// debugging loses the information it exists to provide.
	for _, name := range []string{"Content-Type", "User-Agent", "Accept", "Copilot-Integration-Id"} {
		value := "application/json"
		if got := RedactHeader(name, value); got != value {
			t.Errorf("RedactHeader(%q, %q) = %q, want unchanged", name, value, got)
		}
	}

	// Empty values stay empty.
	if got := RedactHeader("Authorization", ""); got != "" {
		t.Errorf("RedactHeader empty = %q, want empty", got)
	}
}

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantHidden string
	}{
		{name: "openai key", in: `{"key":"sk-abcdefghijklmnopqrstuvwx"}`, wantHidden: "sk-abcdefghijklmnopqrstuvwx"},
		{name: "anthropic key", in: `key sk-ant-api03-abcdefghijklmnopqrstuvwx end`, wantHidden: "sk-ant-api03-abcdefghijklmnopqrstuvwx"},
		{name: "bearer", in: "Authorization: Bearer ya29.a0abcdef.ghijkl-mnop", wantHidden: "ya29.a0abcdef.ghijkl-mnop"},
		{name: "github token", in: "token ghp_abcdefghijklmnopqrstuvwxyz0123", wantHidden: "ghp_abcdefghijklmnopqrstuvwxyz0123"},
		{name: "json api_key", in: `{"api_key": "abcdefghij0123456789"}`, wantHidden: "abcdefghij0123456789"},
		{name: "inline api_key=", in: "api_key=abcdefghij0123456789", wantHidden: "abcdefghij0123456789"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactSecrets(tc.in)
			if strings.Contains(got, tc.wantHidden) {
				t.Errorf("RedactSecrets(%q) = %q, want %q redacted", tc.in, got, tc.wantHidden)
			}
			if !strings.Contains(got, "REDACTED") {
				t.Errorf("RedactSecrets(%q) = %q, want a REDACTED placeholder", tc.in, got)
			}
		})
	}

	// Ordinary content must survive: git SHAs, hex digests and file paths are
	// the reason this helper is narrower than redactSensitiveData.
	keep := []string{
		"commit 1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b",
		"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"/home/user/projects/coding-agent/implementation/main.go",
		"func TestNewAgent_SkillsDiscovered(t *testing.T) {",
	}
	for _, in := range keep {
		if got := RedactSecrets(in); got != in {
			t.Errorf("RedactSecrets changed ordinary content:\n in: %s\nout: %s", in, got)
		}
	}
}
