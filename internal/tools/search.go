package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SearchTool performs web searches via a local SearXNG instance.
type SearchTool struct {
	searxngURL string
}

func NewSearchTool(searxngURL string) *SearchTool {
	return &SearchTool{searxngURL: searxngURL}
}

func (t *SearchTool) Name() string { return "web_search" }
func (t *SearchTool) Description() string {
	return "Search the web for current information. Returns titles, URLs, and snippets of top results."
}
func (t *SearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "The search query"},
			"num_results": {"type": "integer", "description": "Number of results to return (default 5)", "default": 5}
		},
		"required": ["query"]
	}`)
}
func (t *SearchTool) Execute(ctx context.Context, input ToolInput) (*ToolResult, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return &ToolResult{Error: "query is required"}, nil
	}

	searchURL := fmt.Sprintf("%s/search?q=%s&format=json&engines=google,bing,duckduckgo",
		t.searxngURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &ToolResult{Error: fmt.Sprintf("search request failed: %v", err)}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}

	// Parse and format results
	var result struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return &ToolResult{Output: string(body)}, nil
	}

	limit := 5
	if n, ok := input["num_results"].(float64); ok && int(n) > 0 {
		limit = int(n)
	}

	out := ""
	for i, r := range result.Results {
		if i >= limit {
			break
		}
		out += fmt.Sprintf("[%d] %s\n    URL: %s\n    %s\n\n", i+1, r.Title, r.URL, r.Content)
	}
	if out == "" {
		out = "No results found."
	}
	return &ToolResult{Output: out}, nil
}
