package agent

import "github.com/coding-agent/harness/inference"

// getToolNames returns the list of tool names to use based on config.
// If toolsList is non-empty, it overrides the defaults for the given mode.
func getToolNames(readOnly bool, experimental bool, toolsList []string) []string {
	if len(toolsList) > 0 {
		// User-specified tool list overrides everything
		return toolsList
	}

	if readOnly {
		return []string{
			"read_file",
			"read_lines",
			"list_files",
			"grep",
			"git_log",
			"git_show",
			"git_diff",
			"view_image",
			"todo",
		}
	}

	// Normal mode defaults
	names := []string{
		"bash",
		"read_file",
		"read_lines",
		"write_file",
		"insert_lines",
		"replace_text",
		"move_text",
		"list_files",
		"grep",
		"git_log",
		"git_show",
		"git_diff",
		"view_image",
		"todo",
	}

	if experimental {
		names = append(names, "subagent")
	}

	return names
}

// buildTools builds the tool definitions for the OpenAI API.
// When toolsList is non-empty, it overrides the defaults for the given mode.
func buildTools(readOnly bool, experimental bool, toolsList []string) []inference.ToolDefinition {
	names := getToolNames(readOnly, experimental, toolsList)
	allDefs := AllToolDefinitions()

	var tools []inference.ToolDefinition
	for _, name := range names {
		if info, ok := allDefs[name]; ok {
			tools = append(tools, info.Definition)
		}
	}
	return tools
}
