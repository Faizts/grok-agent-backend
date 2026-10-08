package handlers

import (
	"context"
	"fmt"
	"github.com/grok-agent/backend/internal/config"
	"github.com/grok-agent/backend/internal/sandbox"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strconv"
	"time"
)

// Transaction-scoped locks coordinate computer turns across backend replicas.
func lockComputer(ctx context.Context, db *pgxpool.Pool, userID string) (func(), error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	release := func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}
	var locked bool
	if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1, 9173))`, userID).Scan(&locked); err != nil {
		release()
		return nil, err
	}
	if !locked {
		release()
		return nil, fmt.Errorf("your computer is busy with another agent; retry when its turn finishes")
	}
	return release, nil
}

// Idle computers stop without deleting workspace volumes or browser profiles.
func ReapIdleComputers(ctx context.Context, db *pgxpool.Pool, cfg *config.Config) {
	minutes, err := strconv.Atoi(os.Getenv("SANDBOX_IDLE_MINUTES"))
	if err != nil {
		minutes = 10
	}
	if minutes <= 0 {
		return
	}
	mgr, err := sandbox.NewManager(cfg.SandboxImage, cfg.WorkspacePath)
	if err != nil {
		return
	}
	defer mgr.Close()
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			rows, err := db.Query(ctx, `SELECT id::text FROM users WHERE computer_last_active < NOW() - make_interval(mins => $1)`, minutes)
			if err != nil {
				continue
			}
			var users []string
			for rows.Next() {
				var id string
				if rows.Scan(&id) == nil {
					users = append(users, id)
				}
			}
			rows.Close()
			for _, userID := range users {
				release, err := lockComputer(ctx, db, userID)
				if err != nil {
					continue
				}
				func() {
					defer release()
					var idle bool
					if db.QueryRow(ctx, `SELECT computer_last_active < NOW() - make_interval(mins => $2) FROM users WHERE id=$1`, userID, minutes).Scan(&idle) != nil || !idle {
						return
					}
					agentRows, err := db.Query(ctx, `SELECT DISTINCT sandbox_id FROM agents WHERE user_id=$1 AND status='running' AND sandbox_id IS NOT NULL`, userID)
					if err != nil {
						return
					}
					var containers []string
					for agentRows.Next() {
						var id string
						if agentRows.Scan(&id) == nil {
							containers = append(containers, id)
						}
					}
					agentRows.Close()
					for _, id := range containers {
						connected, err := mgr.DesktopConnected(ctx, id)
						if err != nil {
							continue // Leave the computer running if activity cannot be checked.
						}
						if connected {
							_, _ = db.Exec(ctx, `UPDATE users SET computer_last_active=NOW() WHERE id=$1`, userID)
							continue
						}
						if mgr.Stop(ctx, id) == nil {
							_, _ = db.Exec(ctx, `UPDATE agents SET status='idle',novnc_port='',vnc_port='' WHERE user_id=$1 AND sandbox_id=$2`, userID, id)
						}
					}
				}()
			}
		}
	}
}
