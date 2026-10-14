package debug

import (
	"regexp"
	"strconv"
	"strings"
)

// This file provides redaction helpers for ad-hoc debug output (stderr dumps,
// console traces). They are deliberately narrower than redactSensitiveData,
// which is applied to the whole debug.log file: these helpers must not mangle
// ordinary content such as git SHAs, file paths or base64 blobs.

// sensitiveHeaderNames lists HTTP headers whose values must never be written to
// a log or terminal verbatim. Keys are lower-cased for case-insensitive lookup.
var sensitiveHeaderNames = map[string]bool{
	"authorization":         true,
	"proxy-authorization":   true,
	"x-api-key":             true,
	"api-key":               true,
	"apikey":                true,
	"openai-api-key":        true,
	"anthropic-api-key":     true,
	"x-goog-api-key":        true,
	"x-auth-token":          true,
	"cookie":                true,
	"set-cookie":            true,
	"x-copilot-token":       true,
	"github-authentication": true,
}

// RedactHeader returns a logging-safe representation of an HTTP header value.
//
// Non-sensitive headers are returned unchanged. For sensitive headers the
// credential is dropped entirely; only the authentication scheme (e.g.
// "Bearer") and the value length are kept, which is enough to confirm that the
// right credential was sent without exposing it.
func RedactHeader(name, value string) string {
	if !sensitiveHeaderNames[strings.ToLower(strings.TrimSpace(name))] {
		return value
	}
	if value == "" {
		return ""
	}
	if scheme := authScheme(value); scheme != "" {
		return scheme + " [REDACTED len=" + strconv.Itoa(len(value)) + "]"
	}
	return "[REDACTED len=" + strconv.Itoa(len(value)) + "]"
}

// authScheme returns the leading auth scheme of a header value ("Bearer",
// "Basic", "Token", ...) when the value is of the form "<scheme> <credential>".
func authScheme(value string) string {
	scheme, rest, ok := strings.Cut(strings.TrimSpace(value), " ")
	if !ok || scheme == "" || rest == "" {
		return ""
	}
	// Only treat it as a scheme when the remainder looks like a credential.
	if len(rest) < 8 {
		return ""
	}
	return scheme
}

// secretPatterns matches credential-shaped substrings. Patterns are anchored on
// well-known key prefixes or on JSON/text key names, so ordinary identifiers,
// commit hashes and long hex strings are left intact.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=\-]{12,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{8,}`),
	regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_\-]{8,}`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{16,}`),
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9\-]{8,}`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{20,}`),
	regexp.MustCompile(`\byg[a-z]_[0-9A-Za-z_\-]{20,}`),
	regexp.MustCompile(`(?i)"(api[_-]?key|apikey|token|auth[_-]?token|access[_-]?token|secret|password|private[_-]?key)"\s*:\s*"[^"]{8,}"`),
	regexp.MustCompile(`(?i)\b(api[_-]?key|apikey|access[_-]?token|auth[_-]?token)\s*[:=]\s*[A-Za-z0-9_\-\.]{12,}`),
}

// secretReplacements maps each entry in secretPatterns to its replacement.
// Capture-group patterns use $1 to keep the key name visible.
var secretReplacements = []string{
	"bearer [REDACTED]",
	"[REDACTED_API_KEY]",
	"[REDACTED_API_KEY]",
	"[REDACTED_TOKEN]",
	"[REDACTED_TOKEN]",
	"[REDACTED_API_KEY]",
	"[REDACTED_TOKEN]",
	`"$1": "[REDACTED]"`,
	"$1=[REDACTED]",
}

// RedactSecrets replaces credential-shaped substrings with placeholders.
// Unlike redactSensitiveData it does not redact every long alphanumeric run,
// so it is safe to apply to request bodies and other content that legitimately
// contains hashes, SHAs and encoded blobs.
func RedactSecrets(content string) string {
	for i, re := range secretPatterns {
		content = re.ReplaceAllString(content, secretReplacements[i])
	}
	return content
}
