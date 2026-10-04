package skills

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Skill represents a saved reusable workflow.
type Skill struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	UseCount    int       `json:"use_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Save stores a new skill.
func (s *Store) Save(ctx context.Context, agentID, userID, name, description, content string, tags []string) (*Skill, error) {
	id := uuid.New().String()
	_, err := s.db.Exec(ctx,
		`INSERT INTO skills (id, agent_id, user_id, name, description, content, tags)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, agentID, userID, name, description, content, tags,
	)
	if err != nil {
		return nil, err
	}
	return &Skill{
		ID: id, AgentID: agentID, UserID: userID,
		Name: name, Description: description, Content: content, Tags: tags,
	}, nil
}

// List returns all skills for a user/agent.
func (s *Store) List(ctx context.Context, userID, agentID string) ([]Skill, error) {
	var rows pgx_rows
	var err error

	if agentID != "" {
		rows, err = s.db.Query(ctx,
			`SELECT id, agent_id, user_id, name, description, content, COALESCE(tags, '{}'), use_count, created_at, updated_at
			 FROM skills WHERE agent_id = $1 ORDER BY use_count DESC, created_at DESC`, agentID)
	} else {
		rows, err = s.db.Query(ctx,
			`SELECT id, agent_id, user_id, name, description, content, COALESCE(tags, '{}'), use_count, created_at, updated_at
			 FROM skills WHERE user_id = $1 ORDER BY use_count DESC, created_at DESC`, userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var sk Skill
		rows.Scan(&sk.ID, &sk.AgentID, &sk.UserID, &sk.Name, &sk.Description, &sk.Content, &sk.Tags, &sk.UseCount, &sk.CreatedAt, &sk.UpdatedAt)
		skills = append(skills, sk)
	}
	if skills == nil {
		skills = []Skill{}
	}
	return skills, nil
}

// Get fetches a single skill.
func (s *Store) Get(ctx context.Context, id string) (*Skill, error) {
	var sk Skill
	err := s.db.QueryRow(ctx,
		`SELECT id, agent_id, user_id, name, description, content, COALESCE(tags, '{}'), use_count, created_at, updated_at
		 FROM skills WHERE id = $1`, id).
		Scan(&sk.ID, &sk.AgentID, &sk.UserID, &sk.Name, &sk.Description, &sk.Content, &sk.Tags, &sk.UseCount, &sk.CreatedAt, &sk.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sk, nil
}

// Delete removes a skill.
func (s *Store) Delete(ctx context.Context, id, userID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM skills WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

// IncrementUseCount bumps the usage counter.
func (s *Store) IncrementUseCount(ctx context.Context, id string) {
	s.db.Exec(ctx, `UPDATE skills SET use_count = use_count + 1 WHERE id = $1`, id)
}

// AutoSave generates a skill name/description via summarization and saves it.
// Called after a successful task to auto-capture the workflow.
func (s *Store) AutoSave(ctx context.Context, agentID, userID string, messages []ConversationMessage) (*Skill, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to save as skill")
	}

	// Build markdown content from the conversation
	var sb strings.Builder
	sb.WriteString("# Workflow\n\n")
	for _, m := range messages {
		switch m.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("**User:** %s\n\n", m.Content))
		case "assistant":
			if m.Content != "" {
				sb.WriteString(fmt.Sprintf("**Assistant:** %s\n\n", m.Content))
			}
		case "tool":
			sb.WriteString(fmt.Sprintf("**Tool `%s`:**\n```\n%s\n```\n\n", m.ToolName, truncate(m.Content, 500)))
		}
	}

	// Derive name from first user message
	name := "Saved workflow"
	for _, m := range messages {
		if m.Role == "user" && m.Content != "" {
			name = truncate(m.Content, 60)
			break
		}
	}

	return s.Save(ctx, agentID, userID, name, "Auto-saved workflow", sb.String(), []string{"auto-saved"})
}

// ConversationMessage is a lightweight message type for skill auto-save.
type ConversationMessage struct {
	Role     string
	Content  string
	ToolName string
}

type pgx_rows interface {
	Next() bool
	Scan(...any) error
	Close()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
