package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolInput is a generic map of tool arguments.
type ToolInput map[string]interface{}

// ToolResult holds the output of a tool execution.
type ToolResult struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// Tool is the interface every tool must implement.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Execute(ctx context.Context, input ToolInput) (*ToolResult, error)
}

// Registry holds all registered tools.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

func (r *Registry) Dispatch(ctx context.Context, name string, input ToolInput) (*ToolResult, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return t.Execute(ctx, input)
}

// LLMFunctions returns tool definitions in OpenAI function-calling format.
func (r *Registry) LLMFunctions() []map[string]interface{} {
	var funcs []map[string]interface{}
	for _, t := range r.tools {
		funcs = append(funcs, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name(),
				"description": t.Description(),
				"parameters":  t.Schema(),
			},
		})
	}
	return funcs
}
