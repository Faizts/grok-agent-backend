package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/grok-agent/backend/internal/tools"
)

// MCPToolAdapter wraps a remote MCP tool as a local tools.Tool.
type MCPToolAdapter struct {
	mcpTool MCPTool
	server  MCPServer
	client  *Client
}

func NewToolAdapter(tool MCPTool, server MCPServer, client *Client) *MCPToolAdapter {
	return &MCPToolAdapter{mcpTool: tool, server: server, client: client}
}

func (a *MCPToolAdapter) Name() string {
	raw := a.server.ID + ":" + a.server.URL + ":" + a.mcpTool.Name
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))[:12]
	name := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(a.server.Name+"_"+a.mcpTool.Name, "_")
	if len(name) > 46 {
		name = name[:46]
	}
	return "mcp_" + name + "_" + hash
}
func (a *MCPToolAdapter) Description() string { return a.mcpTool.Description }
func (a *MCPToolAdapter) Schema() json.RawMessage {
	if len(a.mcpTool.InputSchema) > 0 {
		return a.mcpTool.InputSchema
	}
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (a *MCPToolAdapter) Execute(ctx context.Context, input tools.ToolInput) (*tools.ToolResult, error) {
	out, err := a.client.CallTool(ctx, a.server, a.mcpTool.Name, input)
	if err != nil {
		return &tools.ToolResult{Error: fmt.Sprintf("MCP call failed: %v", err)}, nil
	}
	return &tools.ToolResult{Output: out}, nil
}
