package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// parseINI reads a simple INI file and returns the global (pre-section) keys
// plus a map of section name -> key/value pairs.
//
// Supported syntax:
//
//	# comment
//	; comment
//	key=value
//	[section.name]
//	key=value
//
// Values may be quoted with double quotes. Keys are case-insensitive and are
// lower-cased by the parser.
func parseINI(path string) (sections map[string]map[string]string, global map[string]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]string{}, map[string]string{}, nil
		}
		return nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}
	defer f.Close()

	sections = map[string]map[string]string{}
	global = map[string]string{}
	section := ""

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return nil, nil, fmt.Errorf("config file %s:%d: malformed section header %q", path, lineNo, line)
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			if _, ok := sections[section]; !ok {
				sections[section] = map[string]string{}
			}
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		value := strings.TrimSpace(line[idx+1:])
		value = unquote(value)
		if key == "" {
			continue
		}
		if section == "" {
			global[key] = value
		} else {
			sections[section][key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return sections, global, nil
}

func unquote(v string) string {
	if len(v) >= 2 && strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		return strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\"`, "\"").Replace(v[1 : len(v)-1])
	}
	return v
}
