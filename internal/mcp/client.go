package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MCPTool represents a tool discovered from an MCP server.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	ServerName  string          `json:"server_name"`
}

// MCPServer is a configured MCP server entry.
type MCPServer struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Transport string   `json:"transport"` // http | stdio
	URL       string   `json:"url,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Enabled   bool     `json:"enabled"`
}

// Client discovers and calls tools on MCP servers.
type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// DiscoverTools fetches the tools list from an HTTP MCP server.
func (c *Client) DiscoverTools(ctx context.Context, server MCPServer) ([]MCPTool, error) {
	if server.Transport != "http" || server.URL == "" {
		return nil, fmt.Errorf("only HTTP transport supported for discovery")
	}

	url := strings.TrimRight(server.URL, "/") + "/tools/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp discovery failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	var result struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid mcp response: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("mcp error: %s", result.Error.Message)
	}

	var tools []MCPTool
	for _, t := range result.Result.Tools {
		tools = append(tools, MCPTool{
			Name:        t.Name,
			Description: fmt.Sprintf("[MCP:%s] %s", server.Name, t.Description),
			InputSchema: t.InputSchema,
			ServerName:  server.Name,
		})
	}
	return tools, nil
}

// CallTool invokes a tool on an HTTP MCP server.
func (c *Client) CallTool(ctx context.Context, server MCPServer, toolName string, input map[string]interface{}) (string, error) {
	if server.Transport != "http" || server.URL == "" {
		return "", fmt.Errorf("only HTTP transport supported")
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": input,
		},
	})

	url := strings.TrimRight(server.URL, "/") + "/tools/call"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mcp call failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))

	var result struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}
	if result.Error != nil {
		return "", fmt.Errorf("mcp tool error: %s", result.Error.Message)
	}

	var out strings.Builder
	for _, c := range result.Result.Content {
		if c.Type == "text" {
			out.WriteString(c.Text)
		}
	}
	return out.String(), nil
}
