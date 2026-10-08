package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamableHTTPHandshakeAndCall(t *testing.T) {
	initialized := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			t.Error("wrong RPC endpoint")
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", "session")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-03-26"}}`, req.ID)
			return
		}
		if r.Header.Get("Mcp-Session-Id") != "session" || r.Header.Get("MCP-Protocol-Version") != "2025-03-26" {
			t.Error("missing session headers")
		}
		switch req.Method {
		case "notifications/initialized":
			initialized = true
			w.WriteHeader(202)
		case "tools/list":
			if !initialized {
				t.Error("not initialized")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"tools\":[{\"name\":\"test\",\"inputSchema\":{\"type\":\"object\"}}]}}\n\n", req.ID)
		case "tools/call":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"isError":true,"content":[{"type":"text","text":"denied"}]}}`, req.ID)
		}
	}))
	defer server.Close()
	c := NewClient()
	s := MCPServer{ID: "id", Name: "name", Transport: "http", URL: server.URL + "/mcp"}
	tools, err := c.DiscoverTools(context.Background(), s)
	if err != nil || len(tools) != 1 {
		t.Fatalf("discovery %v %v", tools, err)
	}
	if _, err = c.CallTool(context.Background(), s, "test", nil); err == nil {
		t.Fatal("isError ignored")
	}
}
func TestStdioRejected(t *testing.T) {
	_, err := NewClient().DiscoverTools(context.Background(), MCPServer{Transport: "stdio"})
	if err == nil {
		t.Fatal("stdio accepted")
	}
}
