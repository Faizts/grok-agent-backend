package tools

import (
	"context"
	"encoding/json"

	"github.com/grok-agent/backend/internal/sandbox"
)

// ShellTool executes bash commands in the sandbox.
type ShellTool struct {
	manager     *sandbox.Manager
	containerID string
}

func NewShellTool(m *sandbox.Manager, containerID string) *ShellTool {
	return &ShellTool{manager: m, containerID: containerID}
}

func (t *ShellTool) Name() string { return "shell" }
func (t *ShellTool) Description() string {
	return "Execute a bash command in the sandbox Linux environment. Use for file operations, installing packages, running scripts, and system tasks."
}
func (t *ShellTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "The bash command to run"}
		},
		"required": ["command"]
	}`)
}
func (t *ShellTool) Execute(ctx context.Context, input ToolInput) (*ToolResult, error) {
	cmd, _ := input["command"].(string)
	if cmd == "" {
		return &ToolResult{Error: "command is required"}, nil
	}
	res, err := t.manager.ExecShell(ctx, t.containerID, cmd)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	out := res.Stdout
	if res.Stderr != "" {
		out += "\n[stderr]: " + res.Stderr
	}
	if res.ExitCode != 0 {
		out += "\n[exit code]: " + string(rune('0'+res.ExitCode))
	}
	return &ToolResult{Output: out}, nil
}
