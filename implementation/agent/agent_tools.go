package agent

import (
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

// getToolNames returns the list of tool names to use based on config.
// If toolsList is non-empty, it overrides the defaults for the given mode.
// The default lists are the canonical ones defined in the tools package.
func getToolNames(readOnly bool, experimental bool, toolsList []string) []string {
	if len(toolsList) > 0 {
		// User-specified tool list overrides everything
		return toolsList
	}

	if readOnly {
		return tools.DefaultReadOnlyTools()
	}

	// Normal mode defaults
	names := tools.DefaultNormalTools()
	if experimental {
		// Copy so we don't mutate the package's default slice.
		names = append(append([]string{}, names...), "subagent")
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
