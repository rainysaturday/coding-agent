package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// toolAccum accumulates streamed tool call fragments.
type toolAccum struct {
	id   string
	name string
	args string
}

// FlattenArgs converts a JSON arguments object into a flat single line
// "key: value, key: value" representation (requirement 017). Partial JSON is
// rendered as compacted raw text.
func FlattenArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil && len(obj) > 0 {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %s", k, formatValue(obj[k])))
		}
		return strings.Join(parts, ", ")
	}
	// Partial JSON: compact it into one line.
	compact := strings.Join(strings.Fields(raw), " ")
	return compact
}

// formatValue renders a JSON value compactly for display.
func formatValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		if len(t) > 120 {
			return fmt.Sprintf("%q (%d chars)", truncateOneLine(t, 120), len(t))
		}
		return fmt.Sprintf("%q", truncateOneLine(t, 120))
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		return fmt.Sprintf("%t", t)
	default:
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		s := string(data)
		if len(s) > 120 {
			return fmt.Sprintf("%s (%d chars)", s[:120], len(s))
		}
		return s
	}
}

// truncateOneLine collapses whitespace and truncates long strings.
func truncateOneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
