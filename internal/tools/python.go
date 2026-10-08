package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grok-agent/backend/internal/sandbox"
)

// PythonTool executes Python code in the sandbox.
type PythonTool struct {
	manager     *sandbox.Manager
	containerID string
}

func NewPythonTool(m *sandbox.Manager, containerID string) *PythonTool {
	return &PythonTool{manager: m, containerID: containerID}
}

func (t *PythonTool) Name() string { return "python" }
func (t *PythonTool) Description() string {
	return "Execute Python 3 code in the sandbox. Requests, BeautifulSoup and Pillow are installed. Install other packages with pip when needed (shell approval required)."
}
func (t *PythonTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"code": {"type": "string", "description": "Python code to execute"}
		},
		"required": ["code"]
	}`)
}
func (t *PythonTool) Execute(ctx context.Context, input ToolInput) (*ToolResult, error) {
	code, _ := input["code"].(string)
	if code == "" {
		return &ToolResult{Error: "code is required"}, nil
	}
	res, err := t.manager.ExecPython(ctx, t.containerID, code)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	out := res.Stdout
	if res.Stderr != "" {
		out += "\n[stderr]: " + res.Stderr
	}
	if res.ExitCode != 0 {
		return &ToolResult{Output: out, Error: fmt.Sprintf("exit code %d: %s", res.ExitCode, out)}, nil
	}
	return &ToolResult{Output: out}, nil
}
