package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/grok-agent/backend/internal/tools"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func requiresApproval(name string, input tools.ToolInput) bool {
	if name == "web_search" || name == "skill" {
		return false
	}
	if name == "file" {
		action, _ := input["action"].(string)
		return action != "read" && action != "list"
	}
	return true
}

func (a *Agent) approve(ctx context.Context, db *pgxpool.Pool, name, callID string, input tools.ToolInput, events chan<- Event) error {
	if !requiresApproval(name, input) {
		return nil
	}
	if db == nil {
		return fmt.Errorf("approval unavailable: refusing %s", name)
	}
	var rule string
	err := db.QueryRow(ctx, `SELECT rule FROM tool_rules WHERE agent_id=$1 AND tool_name=$2`, a.ID, name).Scan(&rule)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if rule == "always_allow" {
		return nil
	}
	if rule == "always_deny" {
		return fmt.Errorf("tool denied by rule")
	}
	id := uuid.NewString()
	action, _ := json.Marshal(input)
	if _, err = db.Exec(ctx, `INSERT INTO approvals (id,agent_id,tool_name,action) VALUES ($1,$2,$3,$4)`, id, a.ID, name, string(action)); err != nil {
		return err
	}
	if !emit(ctx, events, Event{Type: EventApproval, Tool: name, Input: input, ToolCallID: callID, ApprovalID: id}) {
		return ctx.Err()
	}
	wait, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	defer func() {
		cleanup, c := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer c()
		db.Exec(cleanup, `UPDATE approvals SET status='denied' WHERE id=$1 AND status='pending'`, id)
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-wait.Done():
			return fmt.Errorf("approval cancelled or expired: %w", wait.Err())
		case <-ticker.C:
			var status string
			if err = db.QueryRow(wait, `SELECT status FROM approvals WHERE id=$1 AND agent_id=$2`, id, a.ID).Scan(&status); err != nil {
				return err
			}
			switch status {
			case "approved":
				return nil
			case "pending":
			default:
				return fmt.Errorf("tool approval denied")
			}
		}
	}
}
