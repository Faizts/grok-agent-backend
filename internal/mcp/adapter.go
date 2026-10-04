package mcp

import (
	"context"
	"encoding/json"
	"fmt"

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

func (a *MCPToolAdapter) Name() string        { return "mcp_" + a.mcpTool.ServerName + "_" + a.mcpTool.Name }
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
