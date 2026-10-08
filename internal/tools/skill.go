package tools

import (
	"context"
	"encoding/json"
	"github.com/grok-agent/backend/internal/skills"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type SkillTool struct {
	db              *pgxpool.Pool
	userID, agentID string
}

func NewSkillTool(db *pgxpool.Pool, userID, agentID string) *SkillTool {
	return &SkillTool{db, userID, agentID}
}
func (t *SkillTool) Name() string { return "skill" }
func (t *SkillTool) Description() string {
	return "Read complete reusable workflows before performing related tasks. Actions: list or read (default). Use a built-in name or saved skill UUID; resource defaults to SKILL.md. ffmpeg-video-editing also has render.py. " + skills.CatalogPrompt()
}
func (t *SkillTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"action":{"type":"string","enum":["list","read"]},"name":{"type":"string"},"resource":{"type":"string"}}}`)
}
func (t *SkillTool) Execute(ctx context.Context, input ToolInput) (*ToolResult, error) {
	action, _ := input["action"].(string)
	if action != "" && action != "read" && action != "list" {
		return &ToolResult{Error: "Unknown skill action"}, nil
	}
	if action == "list" {
		list := skills.Builtins()
		if t.db != nil {
			var err error
			list, err = skills.NewStore(t.db).List(ctx, t.userID, "")
			if err != nil {
				return nil, err
			}
		}
		var out strings.Builder
		for _, s := range list {
			if s.AgentID == "" || s.AgentID == t.agentID {
				out.WriteString(s.ID + " | " + s.Name + " | " + s.Description + "\n")
			}
		}
		return &ToolResult{Output: out.String()}, nil
	}
	name, _ := input["name"].(string)
	resource, _ := input["resource"].(string)
	if content, err := skills.BuiltinResource(name, resource); err == nil {
		return &ToolResult{Output: content}, nil
	}
	if t.db == nil || (resource != "" && resource != "SKILL.md") {
		return &ToolResult{Error: "Skill or resource not found"}, nil
	}
	sk, err := skills.NewStore(t.db).Get(ctx, name)
	if err != nil || sk.UserID != t.userID || (sk.AgentID != "" && sk.AgentID != t.agentID) {
		return &ToolResult{Error: "Skill not found"}, nil
	}
	if len(sk.Content) > 65536 {
		return &ToolResult{Error: "Skill exceeds 64 KB; split it into smaller workflows before using it."}, nil
	}
	skills.NewStore(t.db).IncrementUseCount(ctx, sk.ID)
	return &ToolResult{Output: sk.Content}, nil
}
