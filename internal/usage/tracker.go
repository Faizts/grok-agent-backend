package usage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventType string

const (
	EventLLMCall      EventType = "llm_call"
	EventToolCall     EventType = "tool_call"
	EventTaskComplete EventType = "task_complete"
)

type Event struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	AgentID          string    `json:"agent_id"`
	ConversationID   string    `json:"conversation_id"`
	EventType        EventType `json:"event_type"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	ToolName         string    `json:"tool_name,omitempty"`
	DurationMs       int       `json:"duration_ms"`
	CreatedAt        time.Time `json:"created_at"`
}

type Stats struct {
	TotalLLMCalls      int `json:"total_llm_calls"`
	TotalToolCalls     int `json:"total_tool_calls"`
	TotalTasksComplete int `json:"total_tasks_complete"`
	TotalPromptTokens  int `json:"total_prompt_tokens"`
	TotalOutputTokens  int `json:"total_completion_tokens"`
	TotalTokens        int `json:"total_tokens"`
	AvgDurationMs      int `json:"avg_duration_ms"`
}

type ToolStat struct {
	ToolName string `json:"tool_name"`
	Count    int    `json:"count"`
}

type DailyUsage struct {
	Date         string `json:"date"`
	LLMCalls     int    `json:"llm_calls"`
	PromptTokens int    `json:"prompt_tokens"`
	OutputTokens int    `json:"completion_tokens"`
}

type Tracker struct {
	db *pgxpool.Pool
}

func NewTracker(db *pgxpool.Pool) *Tracker {
	return &Tracker{db: db}
}

func (t *Tracker) Record(ctx context.Context, e Event) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	_, err := t.db.Exec(ctx,
		`INSERT INTO usage_events
		 (id, user_id, agent_id, conversation_id, event_type, model, prompt_tokens, completion_tokens, tool_name, duration_ms)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		e.ID, nullableID(e.UserID), nullableID(e.AgentID), nullableID(e.ConversationID),
		string(e.EventType), e.Model, e.PromptTokens, e.CompletionTokens,
		e.ToolName, e.DurationMs,
	)
	return err
}

func (t *Tracker) GetStats(ctx context.Context, userID string) (*Stats, error) {
	var s Stats
	err := t.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE event_type = 'llm_call'),
			COUNT(*) FILTER (WHERE event_type = 'tool_call'),
			COUNT(*) FILTER (WHERE event_type = 'task_complete'),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(prompt_tokens + completion_tokens), 0),
			COALESCE(AVG(duration_ms) FILTER (WHERE event_type = 'llm_call'), 0)::int
		FROM usage_events WHERE user_id = $1`, userID).
		Scan(&s.TotalLLMCalls, &s.TotalToolCalls, &s.TotalTasksComplete,
			&s.TotalPromptTokens, &s.TotalOutputTokens, &s.TotalTokens, &s.AvgDurationMs)
	return &s, err
}

func (t *Tracker) GetTopTools(ctx context.Context, userID string, limit int) ([]ToolStat, error) {
	rows, err := t.db.Query(ctx, `
		SELECT tool_name, COUNT(*) as cnt
		FROM usage_events
		WHERE user_id = $1 AND event_type = 'tool_call' AND tool_name != ''
		GROUP BY tool_name ORDER BY cnt DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []ToolStat
	for rows.Next() {
		var ts ToolStat
		rows.Scan(&ts.ToolName, &ts.Count)
		stats = append(stats, ts)
	}
	if stats == nil {
		stats = []ToolStat{}
	}
	return stats, nil
}

func (t *Tracker) GetDailyUsage(ctx context.Context, userID string, days int) ([]DailyUsage, error) {
	rows, err := t.db.Query(ctx, `
		SELECT
			DATE(created_at)::TEXT as date,
			COUNT(*) FILTER (WHERE event_type = 'llm_call') as llm_calls,
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0)
		FROM usage_events
		WHERE user_id = $1 AND created_at >= NOW() - INTERVAL '1 day' * $2
		GROUP BY DATE(created_at) ORDER BY date ASC`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var usage []DailyUsage
	for rows.Next() {
		var d DailyUsage
		rows.Scan(&d.Date, &d.LLMCalls, &d.PromptTokens, &d.OutputTokens)
		usage = append(usage, d)
	}
	if usage == nil {
		usage = []DailyUsage{}
	}
	return usage, nil
}

func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
