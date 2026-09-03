package tools

import "fmt"

// executeActivateSkill handles the activate_skill tool call. It loads a skill's
// instructions into the agent's context by delegating to the callback registered
// via SetSkillActivator (wired by the agent package). The tool is read-only: it
// only adds instructions to context, never modifies the file system.
func (te *ToolExecutor) executeActivateSkill(params map[string]interface{}) *ToolResult {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return &ToolResult{
			Success: false,
			Error:   "activate_skill requires a 'name' string parameter",
		}
	}

	if te.activateSkill == nil {
		return &ToolResult{
			Success: false,
			Error:   "activate_skill is not available: no skill activator registered",
		}
	}

	output, err := te.activateSkill(name)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to activate skill %q: %v", name, err),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  output,
	}
}
