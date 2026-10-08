package agent

import (
	"context"
	"fmt"
	"github.com/grok-agent/backend/internal/llm"
	"github.com/grok-agent/backend/internal/tools"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestApprovalClassification(t *testing.T) {
	for _, test := range []struct {
		name, action string
		want         bool
	}{{"file", "read", false}, {"file", "list", false}, {"web_search", "", false}, {"skill", "read", false}, {"skill", "list", false}, {"file", "write", true}, {"file", "delete", true}, {"file", "", true}, {"shell", "", true}, {"python", "", true}, {"browser", "get_text", true}, {"mcp_test", "", true}, {"unknown", "", true}} {
		if got := requiresApproval(test.name, tools.ToolInput{"action": test.action}); got != test.want {
			t.Errorf("%s/%s = %v", test.name, test.action, got)
		}
	}
}
func TestHistoryIsolation(t *testing.T) {
	a := &Agent{}
	history := []llm.Message{{ToolCalls: []llm.ToolCall{{ID: "original"}}}}
	a.SetHistory(history)
	history[0].ToolCalls[0].ID = "mutated"
	got := a.History()
	got[0].ToolCalls[0].ID = "again"
	if a.History()[0].ToolCalls[0].ID != "original" {
		t.Fatal("history aliases caller data")
	}
}
func TestRunDeniesShellWithoutApprovalStore(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if requests.Add(1) == 1 {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call1\",\"function\":{\"name\":\"shell\",\"arguments\":\"{\\\"command\\\":\\\"echo never\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		} else {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Denied\"},\"finish_reason\":\"stop\"}]}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	a := New("", "", "", llm.NewClient(server.URL, "test", "test"), nil, "")
	events := make(chan Event, 32)
	if err := a.Run(context.Background(), "test", "", nil, events); err != nil {
		t.Fatal(err)
	}
	history := a.History()
	if len(history) != 4 || history[2].ToolCallID != "call1" || history[2].Content != "Error: approval unavailable: refusing shell" {
		t.Fatalf("bad execution history: %#v", history)
	}
}
func TestCancelledRunClosesEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &Agent{}
	ch := make(chan Event, 1)
	if a.Run(ctx, "", "", nil, ch) == nil {
		t.Fatal("expected cancellation")
	}
	for range ch {
	}
}
