package tools

import (
	"context"
	"strings"
	"testing"
)

func TestSkillReader(t *testing.T) {
	tool := NewSkillTool(nil, "user", "agent")
	for _, name := range []string{"video-production", "social-publishing", "ffmpeg-video-editing"} {
		result, err := tool.Execute(context.Background(), ToolInput{"name": name})
		if err != nil || result.Error != "" || !strings.HasPrefix(result.Output, "# ") {
			t.Fatalf("%s: %v %+v", name, err, result)
		}
	}
	result, _ := tool.Execute(context.Background(), ToolInput{"name": "ffmpeg-video-editing", "resource": "render.py"})
	if !strings.Contains(result.Output, "def render") {
		t.Fatal("missing helper")
	}
	result, _ = tool.Execute(context.Background(), ToolInput{"name": "video-production", "resource": "../secret"})
	if result.Error == "" {
		t.Fatal("accepted resource escape")
	}
	result, _ = tool.Execute(context.Background(), ToolInput{"action": "list"})
	if !strings.Contains(result.Output, "builtin:capcut-browser-editing") {
		t.Fatal("missing catalog")
	}
}
