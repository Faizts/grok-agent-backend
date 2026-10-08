package tools

import (
	"context"
	"testing"
)

func TestFileRejectsWorkspaceEscape(t *testing.T) {
	tool := NewFileTool(nil, "")
	for _, path := range []string{"../../etc/passwd", "/etc/passwd", "/workspace/../secret"} {
		result, err := tool.Execute(context.Background(), ToolInput{"action": "read", "path": path})
		if err != nil || result.Error == "" {
			t.Errorf("escape accepted: %s", path)
		}
	}
}
func TestShellQuotePreservesMetacharacters(t *testing.T) {
	if got := shellQuote("a'$(command)"); got != "'a'\"'\"'$(command)'" {
		t.Fatalf("unsafe quoting %q", got)
	}
}
