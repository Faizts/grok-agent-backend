package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	ServerName  string          `json:"server_name"`
}
type MCPServer struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	URL       string   `json:"url,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Enabled   bool     `json:"enabled"`
}
type session struct{ id, version string }
type Client struct {
	httpClient *http.Client
	mu         sync.Mutex
	sessions   map[string]session
	nextID     int
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 30 * time.Second}, sessions: map[string]session{}}
}

type rpcEnvelope struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// rpc implements the Streamable HTTP endpoint (not REST-style per-method URLs).
func (c *Client) rpc(ctx context.Context, s MCPServer, method string, params any, state session, notification bool) (json.RawMessage, string, error) {
	if s.Transport != "http" || s.URL == "" {
		return nil, "", fmt.Errorf("only Streamable HTTP MCP is supported; stdio must not run on the host")
	}
	c.nextID++
	id := c.nextID
	payload := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if !notification {
		payload["id"] = id
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if state.id != "" {
		req.Header.Set("Mcp-Session-Id", state.id)
	}
	if state.version != "" {
		req.Header.Set("MCP-Protocol-Version", state.version)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("MCP HTTP status %d", resp.StatusCode)
	}
	sid := resp.Header.Get("Mcp-Session-Id")
	if notification {
		return nil, sid, nil
	}
	decode := func(data []byte) (json.RawMessage, bool, error) {
		var envelope rpcEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, false, err
		}
		if string(envelope.ID) != fmt.Sprint(id) {
			return nil, false, nil
		}
		if envelope.Error != nil {
			return nil, true, fmt.Errorf("MCP error %d: %s", envelope.Error.Code, envelope.Error.Message)
		}
		if envelope.Result == nil {
			return nil, true, fmt.Errorf("MCP response missing result")
		}
		return envelope.Result, true, nil
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, 4<<20))
		scanner.Buffer(make([]byte, 4096), 2<<20)
		var data strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if data.Len() > 0 {
					result, matched, err := decode([]byte(strings.TrimSuffix(data.String(), "\n")))
					if err != nil || matched {
						return result, sid, err
					}
					data.Reset()
				}
				continue
			}
			if strings.HasPrefix(line, "data:") {
				data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
				data.WriteByte('\n')
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, sid, err
		}
		return nil, sid, fmt.Errorf("MCP stream ended without matching response")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, sid, err
	}
	result, matched, err := decode(data)
	if err != nil {
		return nil, sid, err
	}
	if !matched {
		return nil, sid, fmt.Errorf("MCP response ID mismatch")
	}
	return result, sid, nil
}
func (c *Client) initialize(ctx context.Context, s MCPServer) (session, error) {
	key := s.ID + "|" + s.URL
	if state, ok := c.sessions[key]; ok {
		return state, nil
	}
	raw, id, err := c.rpc(ctx, s, "initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "grok-agent", "version": "1.0"}}, session{}, false)
	if err != nil {
		return session{}, err
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return session{}, err
	}
	switch result.ProtocolVersion {
	case "2024-11-05", "2025-03-26", "2025-06-18":
	default:
		return session{}, fmt.Errorf("unsupported MCP protocol %q", result.ProtocolVersion)
	}
	state := session{id: id, version: result.ProtocolVersion}
	if _, _, err = c.rpc(ctx, s, "notifications/initialized", map[string]any{}, state, true); err != nil {
		return session{}, err
	}
	c.sessions[key] = state
	return state, nil
}
func (c *Client) DiscoverTools(ctx context.Context, s MCPServer) ([]MCPTool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, err := c.initialize(ctx, s)
	if err != nil {
		return nil, err
	}
	var tools []MCPTool
	cursor := ""
	seen := map[string]bool{}
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, _, err := c.rpc(ctx, s, "tools/list", params, state, false)
		if err != nil {
			return nil, err
		}
		var result struct {
			Tools      []MCPTool `json:"tools"`
			NextCursor string    `json:"nextCursor"`
		}
		if err = json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		for _, t := range result.Tools {
			t.ServerName = s.Name
			tools = append(tools, t)
		}
		if result.NextCursor == "" {
			return tools, nil
		}
		if seen[result.NextCursor] || len(tools) > 10000 {
			return nil, fmt.Errorf("invalid MCP pagination")
		}
		cursor = result.NextCursor
		seen[cursor] = true
	}
}
func (c *Client) CallTool(ctx context.Context, s MCPServer, name string, input map[string]interface{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, err := c.initialize(ctx, s)
	if err != nil {
		return "", err
	}
	raw, _, err := c.rpc(ctx, s, "tools/call", map[string]any{"name": name, "arguments": input}, state, false)
	if err != nil {
		return "", err
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError           bool            `json:"isError"`
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	var out []string
	for _, part := range result.Content {
		if part.Type == "text" {
			out = append(out, part.Text)
		}
	}
	if len(result.StructuredContent) > 0 {
		out = append(out, string(result.StructuredContent))
	}
	text := strings.Join(out, "\n")
	if result.IsError {
		return "", fmt.Errorf("MCP tool failed: %s", text)
	}
	return text, nil
}
