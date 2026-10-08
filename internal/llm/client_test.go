package llm

import (
	"context"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccumulatorSeparateSparseCalls(t *testing.T) {
	a := toolAccumulator{}
	one, three := 1, 3
	a.add(openai.ToolCall{Index: &three, ID: "b", Function: openai.FunctionCall{Name: "file", Arguments: `{"action":`}})
	a.add(openai.ToolCall{Index: &one, ID: "a", Function: openai.FunctionCall{Name: "shell", Arguments: `{"command":"pwd"}`}})
	a.add(openai.ToolCall{Index: &three, Function: openai.FunctionCall{Arguments: `"read"}`}})
	calls := a.calls()
	if len(calls) != 2 || calls[0].ID != "a" || calls[1].Function.Arguments != `{"action":"read"}` {
		t.Fatalf("bad calls: %#v", calls)
	}
}
func TestStreamUsageAndFinalTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"a\",\"function\":{\"name\":\"file\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	ch := make(chan StreamChunk, 8)
	err := NewClient(server.URL, "test", "test").StreamComplete(context.Background(), nil, nil, ch)
	if err != nil {
		t.Fatal(err)
	}
	chunks := []StreamChunk{}
	for c := range ch {
		chunks = append(chunks, c)
	}
	if len(chunks) != 1 || !chunks[0].Done || chunks[0].Usage.PromptTokens != 10 || chunks[0].ToolCalls[0].Function.Arguments != "{}" {
		t.Fatalf("bad chunks %#v", chunks)
	}
}
func TestIncompleteStreamFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
	}))
	defer server.Close()
	ch := make(chan StreamChunk, 8)
	if err := NewClient(server.URL, "test", "test").StreamComplete(context.Background(), nil, nil, ch); err == nil {
		t.Fatal("accepted incomplete stream")
	}
}
