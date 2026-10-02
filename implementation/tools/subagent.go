package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SubagentTool spawns a nested agent to work on an independent task.
type subagentTool struct{ reg *Registry }

func (t *subagentTool) Name() string { return "subagent" }

func (t *subagentTool) Description() string {
	return "Spawn a sub-agent to work on a task independently. The sub-agent runs as a separate process and returns only its conclusion/summary."
}

func (t *subagentTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"prompt":  prop("string", "The task description for the sub-agent. Be specific and clear about what you want the sub-agent to accomplish."),
		"persona": prop("string", "A persona to give the sub-agent. For example: 'Expert Go developer', 'Code reviewer focused on security', 'Documentation writer'."),
	}, []string{"prompt"})
}

func (t *subagentTool) ReadOnly() bool { return true }

func (t *subagentTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	prompt := strings.TrimSpace(getString(args, "prompt"))
	if prompt == "" {
		return "", errors.New("missing required parameter: prompt")
	}
	runner := t.reg.SubagentRunner()
	if runner == nil {
		return "", errors.New("subagents are not available")
	}
	timeoutMS := t.reg.RunContext().SubagentTimeoutMS
	if timeoutMS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
		defer cancel()
	}
	out, err := runner.RunSubagent(ctx, prompt, getString(args, "persona"))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "(subagent produced no output)", nil
	}
	return out, nil
}

// activateSkillTool loads a skill's instructions into the conversation.
type activateSkillTool struct{ reg *Registry }

func (t *activateSkillTool) Name() string { return "activate_skill" }

func (t *activateSkillTool) Description() string {
	return "Load a skill's instructions and bundled resources into context. Skills are discovered from the AVAILABLE SKILLS catalog."
}

func (t *activateSkillTool) Schema() map[string]interface{} {
	nameProp := prop("string", "The name of the skill to activate (must be one of the available skills).")
	// Constrain the name to the discovered skill set so the model cannot
	// hallucinate a skill name (skills spec 6.2).
	if act := t.reg.SkillActivator(); act != nil {
		if names := act.Names(); len(names) > 0 {
			nameProp["enum"] = names
		}
	}
	return objectSchema(map[string]interface{}{
		"name": nameProp,
	}, []string{"name"})
}

func (t *activateSkillTool) ReadOnly() bool { return true }

func (t *activateSkillTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	name := strings.TrimSpace(getString(args, "name"))
	if name == "" {
		return "", errors.New("missing required parameter: name")
	}
	activator := t.reg.SkillActivator()
	if activator == nil {
		return "", errors.New("no skills are available (skills are disabled or none were discovered)")
	}
	body, err := activator.Activate(name)
	if err != nil {
		names := activator.Names()
		if len(names) > 0 {
			return "", fmt.Errorf("%w (available skills: %s)", err, strings.Join(names, ", "))
		}
		return "", err
	}
	return body, nil
}
