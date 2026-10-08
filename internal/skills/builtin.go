package skills

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed bundles/*/*
var bundled embed.FS

var catalog = []struct {
	name, description string
	tags              []string
}{
	{"video-production", "Plan and produce complete videos with your preferred services.", []string{"video", "storyboard", "automation"}},
	{"browser-media-generation", "Generate and download images or video from Gemini, ChatGPT, or a chosen website.", []string{"gemini", "chatgpt", "images", "generation"}},
	{"ffmpeg-video-editing", "Combine stills, clips and narration with lightweight local rendering.", []string{"ffmpeg", "combine", "render", "video"}},
	{"transcription-subtitles", "Create scripts, faithful transcripts and verified captions.", []string{"transcript", "subtitles", "captions", "narration"}},
	{"capcut-browser-editing", "Import, edit and export through the CapCut browser editor.", []string{"capcut", "editing"}},
	{"social-publishing", "Prepare and publish reviewed videos to selected social accounts.", []string{"youtube", "facebook", "instagram", "upload", "publish"}},
}

func Builtins() []Skill {
	out := make([]Skill, 0, len(catalog))
	for _, item := range catalog {
		content, err := bundled.ReadFile("bundles/" + item.name + "/SKILL.md")
		if err != nil {
			panic(err)
		}
		out = append(out, Skill{ID: "builtin:" + item.name, Name: item.name, Description: item.description, Content: string(content), Tags: item.tags, Builtin: true})
	}
	return out
}

func BuiltinResource(name, resource string) (string, error) {
	name = strings.TrimPrefix(name, "builtin:")
	if resource == "" {
		resource = "SKILL.md"
	}
	for _, item := range catalog {
		if item.name == name && (resource == "SKILL.md" || (name == "ffmpeg-video-editing" && resource == "render.py")) {
			data, err := bundled.ReadFile("bundles/" + name + "/" + resource)
			return string(data), err
		}
	}
	return "", fmt.Errorf("unknown skill or resource")
}

func CatalogPrompt() string {
	var b strings.Builder
	b.WriteString("Available reusable workflows (read the full matching workflow with the skill tool before executing it; follow user preferences and permissions):\n")
	for _, s := range Builtins() {
		fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
	}
	return b.String()
}
