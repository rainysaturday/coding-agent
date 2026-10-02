// Package tools implements the agent tools (bash, file editing, search, git
// inspection, todo, subagents, skills and image viewing) and the registry that
// exposes them to the model as OpenAI function definitions.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/coding-agent/coding-agent/inference"
)

// Tool is a single callable tool.
type Tool interface {
	Name() string
	Description() string
	Schema() map[string]interface{}
	// ReadOnly reports whether the tool only inspects state and never mutates.
	ReadOnly() bool
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// RunContext carries shared state into tool implementations.
type RunContext struct {
	CWD               string
	ReadOnly          bool
	BashTimeoutMS     int
	MaxOutputLines    int
	SubagentTimeoutMS int
	MaxReadBytes      int64
	MaxImageBytes     int64
	AllowFileWrites   bool
}

// SubagentRunner executes a subagent task (implemented by the agent).
type SubagentRunner interface {
	RunSubagent(ctx context.Context, task, model string) (string, error)
}

// SkillActivator exposes the skills system to the activate_skill tool.
type SkillActivator interface {
	Activate(name string) (string, error)
	Catalog() string
	Names() []string
}

// Registry holds the available tools.
type Registry struct {
	mu       sync.RWMutex
	byName   map[string]Tool
	order    []string
	run      *RunContext
	subagent SubagentRunner
	skills   SkillActivator
}

// NewRegistry creates an empty registry.
func NewRegistry(run *RunContext) *Registry {
	if run == nil {
		run = &RunContext{}
	}
	if run.MaxOutputLines <= 0 {
		run.MaxOutputLines = 200
	}
	if run.BashTimeoutMS <= 0 {
		run.BashTimeoutMS = 30000
	}
	if run.MaxReadBytes <= 0 {
		run.MaxReadBytes = 20 * 1024
	}
	if run.MaxImageBytes <= 0 {
		run.MaxImageBytes = 20 * 1024 * 1024
	}
	return &Registry{byName: map[string]Tool{}, run: run}
}

// RunContext returns the shared run context.
func (r *Registry) RunContext() *RunContext { return r.run }

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[t.Name()]; !exists {
		r.order = append(r.order, t.Name())
	}
	r.byName[t.Name()] = t
}

// Unregister removes a tool by name.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byName[name]; !ok {
		return
	}
	delete(r.byName, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

// SetSubagentRunner wires the subagent tool implementation.
func (r *Registry) SetSubagentRunner(s SubagentRunner) {
	r.mu.Lock()
	r.subagent = s
	r.mu.Unlock()
	if s != nil {
		r.Register(&subagentTool{reg: r})
	} else {
		r.Unregister("subagent")
	}
}

// SetSkillActivator wires the activate_skill tool implementation.
func (r *Registry) SetSkillActivator(s SkillActivator) {
	r.mu.Lock()
	r.skills = s
	r.mu.Unlock()
	if s != nil {
		r.Register(&activateSkillTool{reg: r})
	} else {
		r.Unregister("activate_skill")
	}
}

// SubagentRunner returns the configured subagent runner.
func (r *Registry) SubagentRunner() SubagentRunner {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.subagent
}

// SkillActivator returns the configured skill activator.
func (r *Registry) SkillActivator() SkillActivator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skills
}

// SetReadOnly toggles read-only mode for the whole registry.
func (r *Registry) SetReadOnly(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.run.ReadOnly = v
}

// ReadOnly reports the current read-only state.
func (r *Registry) ReadOnly() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.run.ReadOnly
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.byName[name]
	return t, ok
}

// Names returns the registered tool names in registration order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Definitions returns the OpenAI tool definitions.
func (r *Registry) Definitions() []inference.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, len(r.order))
	copy(names, r.order)
	defs := make([]inference.ToolDefinition, 0, len(names))
	for _, n := range names {
		t := r.byName[n]
		// In read-only mode only read-only tools are advertised to the model
		// (requirement 033).
		if r.run.ReadOnly && !t.ReadOnly() {
			continue
		}
		defs = append(defs, inference.ToolDefinition{
			Type: "function",
			Function: inference.ToolFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Schema(),
			},
		})
	}
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].Function.Name < defs[j].Function.Name })
	return defs
}

// Execute runs a tool call and returns its textual result together with a
// failure flag. The raw result is returned unwrapped; the caller adds the
// requirement-016 wrapper ("Tool 'name' executed successfully: ..." /
// "Tool 'name' failed: ...") when storing it in the conversation context.
func (r *Registry) Execute(ctx context.Context, call inference.ToolCall) (string, bool) {
	r.mu.RLock()
	tool, ok := r.byName[call.Function.Name]
	readOnly := r.run.ReadOnly
	r.mu.RUnlock()

	if !ok {
		return fmt.Sprintf("unknown tool %q; available tools: %s", call.Function.Name, strings.Join(r.Names(), ", ")), true
	}
	if readOnly && !tool.ReadOnly() {
		return fmt.Sprintf("tool %q is not available in read-only mode; only read-only tools are permitted", call.Function.Name), true
	}

	args := map[string]interface{}{}
	if strings.TrimSpace(call.Function.Arguments) != "" {
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return fmt.Sprintf("invalid JSON arguments for %s: %v", call.Function.Name, err), true
		}
	}
	out, err := tool.Execute(ctx, args)
	if err != nil {
		return err.Error(), true
	}
	return out, false
}

// ---------------------------------------------------------------------------
// argument helpers
// ---------------------------------------------------------------------------

func getString(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		switch t := v.(type) {
		case string:
			return t
		case nil:
			return ""
		default:
			return fmt.Sprintf("%v", t)
		}
	}
	return ""
}

func getInt(args map[string]interface{}, key string, def int) int {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return def
		}
		return int(n)
	case string:
		var n int
		if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func getBool(args map[string]interface{}, key string, def bool) bool {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(t) {
		case "true", "yes", "1":
			return true
		case "false", "no", "0":
			return false
		}
	case float64:
		return t != 0
	}
	return def
}

func getStringSlice(args map[string]interface{}, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprintf("%v", e))
		}
		return out
	case []string:
		return t
	case string:
		fields := strings.Fields(t)
		if len(fields) == 0 {
			return nil
		}
		return fields
	}
	return nil
}

// objectSchema is a small helper for building JSON schemas.
func objectSchema(props map[string]interface{}, required []string) map[string]interface{} {
	s := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func prop(typ, desc string) map[string]interface{} {
	return map[string]interface{}{"type": typ, "description": desc}
}

func arrayProp(itemType, desc string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"items":       map[string]interface{}{"type": itemType},
		"description": desc,
	}
}
