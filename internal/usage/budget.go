package usage

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Cost(prompt, completion int, inputRate, outputRate float64) float64 {
	return (float64(prompt)*inputRate + float64(completion)*outputRate) / 1000
}

// CheckBudget durably rolls over the current UTC billing month before checking limits.
func CheckBudget(ctx context.Context, db *pgxpool.Pool, userID, model string) error {
	_, err := db.Exec(ctx, `UPDATE users SET spent_this_month_usd=0, budget_period=date_trunc('month', NOW() AT TIME ZONE 'UTC')::date WHERE id=$1 AND budget_period < date_trunc('month', NOW() AT TIME ZONE 'UTC')::date`, userID)
	if err != nil {
		return err
	}
	var status string
	var budget, spent float64
	if err = db.QueryRow(ctx, `SELECT status,monthly_budget_usd,spent_this_month_usd FROM users WHERE id=$1`, userID).Scan(&status, &budget, &spent); err != nil {
		return err
	}
	if status != "active" {
		return fmt.Errorf("account is not active")
	}
	if spent >= budget {
		return fmt.Errorf("monthly budget of $%.2f reached", budget)
	}
	var rate float64
	if err = db.QueryRow(ctx, `SELECT input_cost_per_1k FROM model_rates WHERE model=$1`, model).Scan(&rate); err != nil {
		return fmt.Errorf("model pricing unavailable for %s: %w", model, err)
	}
	return nil
}

// RecordLLM atomically records metering and applies the model's configured token rates.
func (t *Tracker) RecordLLM(ctx context.Context, e Event) error {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var input, output float64
	if err = tx.QueryRow(ctx, `SELECT input_cost_per_1k, output_cost_per_1k FROM model_rates WHERE model=$1`, e.Model).Scan(&input, &output); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE users SET spent_this_month_usd=CASE WHEN budget_period < date_trunc('month',NOW() AT TIME ZONE 'UTC')::date THEN $2 ELSE spent_this_month_usd+$2 END, budget_period=date_trunc('month',NOW() AT TIME ZONE 'UTC')::date WHERE id=$1`, e.UserID, Cost(e.PromptTokens, e.CompletionTokens, input, output))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO usage_events (user_id,agent_id,conversation_id,event_type,model,prompt_tokens,completion_tokens,duration_ms) VALUES ($1,$2,$3,'llm_call',$4,$5,$6,$7)`, e.UserID, nullableID(e.AgentID), nullableID(e.ConversationID), e.Model, e.PromptTokens, e.CompletionTokens, e.DurationMs)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
