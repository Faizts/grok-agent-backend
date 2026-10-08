package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	pathpkg "path"
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
	resolved := pathpkg.Clean(path)
	if !strings.HasPrefix(resolved, "/") {
		resolved = pathpkg.Join("/workspace", resolved)
	}
	if resolved != "/workspace" && !strings.HasPrefix(resolved, "/workspace/") {
		return &ToolResult{Error: "path must be inside /workspace"}, nil
	}
	path = resolved
	quoted := shellQuote(path)
	guard := "resolved=$(realpath -m -- " + quoted + "); case \"$resolved\" in /workspace|/workspace/*) ;; *) echo 'path escapes workspace' >&2; exit 1;; esac; "

	switch action {
	case "read":
		res, err := t.manager.ExecShell(ctx, t.containerID, guard+"cat -- "+quoted)
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		if res.ExitCode != 0 {
			return &ToolResult{Error: fmt.Sprintf("exit code %d: %s", res.ExitCode, res.Stderr)}, nil
		}
		return &ToolResult{Output: res.Stdout}, nil

	case "write":
		content, _ := input["content"].(string)
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		script := guard + "mkdir -p -- " + shellQuote(pathpkg.Dir(path)) + " && printf %s " + shellQuote(encoded) + " | base64 -d > " + quoted

		res, err := t.manager.ExecShell(ctx, t.containerID, script)
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		if res.ExitCode != 0 {
			return &ToolResult{Error: res.Stderr}, nil
		}
		return &ToolResult{Output: fmt.Sprintf("Written to %s", path)}, nil

	case "list":
		res, err := t.manager.ExecShell(ctx, t.containerID, guard+"ls -la -- "+quoted)
		if err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
		if res.ExitCode != 0 {
			return &ToolResult{Error: fmt.Sprintf("exit code %d: %s", res.ExitCode, res.Stderr)}, nil
		}
		return &ToolResult{Output: res.Stdout}, nil

	case "delete":
		res, err := t.manager.ExecShell(ctx, t.containerID, guard+"rm -rf -- "+quoted)
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

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
