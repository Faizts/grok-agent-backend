package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grok-agent/backend/internal/sandbox"
)

// FileTool reads and writes files in the sandbox workspace.
type FileTool struct {
	manager     *sandbox.Manager
	containerID string
}

func NewFileTool(m *sandbox.Manager, containerID string) *FileTool {
	return &FileTool{manager: m, containerID: containerID}
}

func (t *FileTool) Name() string { return "file" }
func (t *FileTool) Description() string {
	return "Read or write files in the sandbox workspace (/workspace). Actions: read, write, list, delete."
}
func (t *FileTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["read", "write", "list", "delete"], "description": "Action to perform"},
			"path":   {"type": "string", "description": "File or directory path (relative to /workspace)"},
			"content": {"type": "string", "description": "Content to write (required for write action)"}
		},
		"required": ["action", "path"]
	}`)
}
func (t *FileTool) Execute(ctx context.Context, input ToolInput) (*ToolResult, error) {
	action, _ := input["action"].(string)
	path, _ := input["path"].(string)
	if path == "" {
		return &ToolResult{Error: "path is required"}, nil
	}
	// Always anchor to /workspace
	if !strings.HasPrefix(path, "/") {
		path = "/workspace/" + path
	}

	switch action {
	case "read":
		res, err := t.manager.ExecShell(ctx, t.containerID, fmt.Sprintf("cat %q", path))
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		return &ToolResult{Output: res.Stdout}, nil

	case "write":
		content, _ := input["content"].(string)
		// Write via printf to preserve newlines
		script := fmt.Sprintf("mkdir -p $(dirname %q) && cat > %q << 'GROKEOF'\n%s\nGROKEOF", path, path, content)
		res, err := t.manager.ExecShell(ctx, t.containerID, script)
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		if res.ExitCode != 0 {
			return &ToolResult{Error: res.Stderr}, nil
		}
		return &ToolResult{Output: fmt.Sprintf("Written to %s", path)}, nil

	case "list":
		res, err := t.manager.ExecShell(ctx, t.containerID, fmt.Sprintf("ls -la %q", path))
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		return &ToolResult{Output: res.Stdout}, nil

	case "delete":
		res, err := t.manager.ExecShell(ctx, t.containerID, fmt.Sprintf("rm -rf %q", path))
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		if res.ExitCode != 0 {
			return &ToolResult{Error: res.Stderr}, nil
		}
		return &ToolResult{Output: fmt.Sprintf("Deleted %s", path)}, nil

	default:
		return &ToolResult{Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}
