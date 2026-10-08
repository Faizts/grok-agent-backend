package skills

import (
	"strings"
	"testing"
)

func TestBuiltinResources(t *testing.T) {
	list := Builtins()
	if len(list) != 6 {
		t.Fatalf("got %d workflows", len(list))
	}
	for _, s := range list {
		content, err := BuiltinResource(s.ID, "SKILL.md")
		if err != nil || content != s.Content || !s.Builtin || s.AgentID != "" {
			t.Fatalf("invalid builtin %s: %v", s.Name, err)
		}
	}
	for _, resource := range []string{"../store.go", "../../bundles/video-production/SKILL.md", "missing.py"} {
		if _, err := BuiltinResource("ffmpeg-video-editing", resource); err == nil {
			t.Fatalf("accepted %s", resource)
		}
	}
	if _, err := BuiltinResource("unknown", "SKILL.md"); err == nil {
		t.Fatal("accepted unknown")
	}
	script, err := BuiltinResource("ffmpeg-video-editing", "render.py")
	if err != nil || !strings.Contains(script, "subprocess.run") {
		t.Fatal("renderer missing")
	}
	publishing, _ := BuiltinResource("social-publishing", "")
	if !strings.Contains(publishing, "before Publish/Share/Schedule") || !strings.Contains(publishing, "avoid duplicate posts") {
		t.Fatal("publishing review/recovery missing")
	}
}
