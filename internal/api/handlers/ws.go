package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/grok-agent/backend/internal/agent"
	"github.com/grok-agent/backend/internal/config"
	"github.com/grok-agent/backend/internal/llm"
	"github.com/grok-agent/backend/internal/memory"
	"github.com/grok-agent/backend/internal/sandbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

var activeConversations sync.Map

type WSHandler struct {
	db  *pgxpool.Pool
	cfg *config.Config
}

func NewWSHandler(db *pgxpool.Pool, cfg *config.Config) *WSHandler {
	return &WSHandler{db: db, cfg: cfg}
}

type wsIncoming struct {
	Message string `json:"message"`
}

func (h *WSHandler) Handle(c *gin.Context) {
	convID, userID := c.Param("conversation_id"), c.GetString("user_id")
	if !owns(c, h.db, "conversations", convID) {
		return
	}
	if _, busy := activeConversations.LoadOrStore(convID, true); busy {
		c.JSON(http.StatusConflict, gin.H{"error": "conversation already connected"})
		return
	}
	defer activeConversations.Delete(convID)
	upgrader := websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 4096, CheckOrigin: websocketOriginPolicy(h.cfg.WSAllowedOrigins)}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	incoming := make(chan wsIncoming)
	go func() {
		defer cancel()
		defer close(incoming)
		conn.SetReadLimit(1024 * 1024)
		for {
			var msg wsIncoming
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			if msg.Message == "" {
				continue
			}
			select {
			case incoming <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()
	write := func(e agent.Event) bool {
		conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
		if conn.WriteJSON(e) != nil {
			cancel()
			return false
		}
		return true
	}
	mgr, err := sandbox.NewManager(h.cfg.SandboxImage, h.cfg.WorkspacePath)
	if err != nil {
		write(agent.Event{Type: agent.EventError, Content: err.Error()})
		return
	}
	defer mgr.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-incoming:
			if !ok {
				return
			}
			release, err := lockComputer(ctx, h.db, userID)
			if err != nil {
				write(agent.Event{Type: agent.EventError, Content: err.Error()})
				continue
			}
			a, err := h.loadAgent(ctx, mgr, convID, userID)
			if err != nil {
				release()
				write(agent.Event{Type: agent.EventError, Content: err.Error()})
				continue
			}
			// Persist the user turn before execution so a refresh or failed provider call cannot erase it.
			var turnID string
			if err := h.db.QueryRow(ctx, `INSERT INTO messages(conversation_id,role,content) VALUES($1,'user',$2) RETURNING id`, convID, msg.Message).Scan(&turnID); err != nil {
				release()
				write(agent.Event{Type: agent.EventError, Content: "Failed to save message; please retry"})
				continue
			}
			write(agent.Event{Type: "turn_started", TurnID: turnID})
			baseline, baselineErr := mgr.WorkingFiles(ctx, a.ContainerID, a.ID)
			before := len(a.History())
			events := make(chan agent.Event, 256)
			finished := make(chan error, 1)
			go func() { finished <- a.Run(ctx, msg.Message, userID, h.db, events) }()
			for ev := range events {
				if ev.Type != agent.EventDone {
					write(ev)
				}
			}
			runErr := <-finished
			saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
			newHistory := a.History()[before:]
			if len(newHistory) > 0 && newHistory[0].Role == "user" {
				newHistory = newHistory[1:]
			}
			err = h.saveHistory(saveCtx, convID, newHistory)
			if err == nil && baselineErr == nil {
				filesCtx, filesCancel := context.WithTimeout(context.Background(), 45*time.Second)
				files, filesErr := mgr.SaveResponseFiles(filesCtx, a.ContainerID, a.ID, turnID, baseline)
				if filesErr == nil {
					encoded, _ := json.Marshal(files)
					_, filesErr = h.db.Exec(filesCtx, `UPDATE messages SET attachments=$1 WHERE id=$2`, encoded, turnID)
					if filesErr == nil {
						write(agent.Event{Type: "files", TurnID: turnID, Attachments: files})
					}
				}
				if filesErr != nil {
					write(agent.Event{Type: agent.EventThinking, Content: "Files could not be attached. Open the agent's Files view."})
				}
				filesCancel()
			}
			saveCancel()
			activityCtx, activityCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = h.db.Exec(activityCtx, `UPDATE users SET computer_last_active=NOW() WHERE id=$1`, userID)
			activityCancel()
			release()
			if err != nil {
				write(agent.Event{Type: agent.EventError, Content: "Failed to persist conversation"})
			} else if runErr == nil {
				write(agent.Event{Type: agent.EventDone})
			}
		}
	}
}

func (h *WSHandler) loadAgent(ctx context.Context, mgr *sandbox.Manager, convID, userID string) (*agent.Agent, error) {
	var id, name, prompt, model string
	err := h.db.QueryRow(ctx, `SELECT a.id,a.name,COALESCE(a.system_prompt,''),COALESCE(u.assigned_model,$3) FROM conversations c JOIN agents a ON a.id=c.agent_id JOIN users u ON u.id=c.user_id WHERE c.id=$1 AND c.user_id=$2 AND a.user_id=$2 AND u.status='active'`, convID, userID, h.cfg.LLMModel).Scan(&id, &name, &prompt, &model)
	if err != nil {
		return nil, fmt.Errorf("conversation unavailable")
	}
	computerID, err := mgr.ComputerKey(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if _, err = h.db.Exec(ctx, `UPDATE users SET computer_last_active=NOW() WHERE id=$1`, userID); err != nil {
		return nil, err
	}
	info, err := mgr.GetOrCreate(ctx, computerID)
	if err != nil {
		return nil, fmt.Errorf("sandbox unavailable: %w", err)
	}
	if _, err = h.db.Exec(ctx, `UPDATE agents SET sandbox_id=$1,novnc_port=$2,vnc_port=$3,status='running' WHERE user_id=$4 AND (id=$5 OR ($6 AND (sandbox_id IS NULL OR sandbox_id='' OR sandbox_id=$1)))`, info.ContainerID, info.NoVNCPort, info.VNCPort, userID, id, computerID != id); err != nil {
		return nil, err
	}
	ready, readyErr := mgr.ExecShell(ctx, info.ContainerID, "for attempt in $(seq 1 30); do curl -fsS http://localhost:9222/json/version >/dev/null && exit 0; sleep 1; done; exit 1")
	if readyErr != nil || ready.ExitCode != 0 {
		return nil, fmt.Errorf("computer browser is not ready; please retry")
	}
	a := agent.New(id, name, prompt, llm.NewClient(h.cfg.LLMBaseURL, h.cfg.LLMAPIKey, model), mgr, info.ContainerID)
	a.ConversationID = convID
	embedKey := h.cfg.EmbedAPIKey
	if embedKey == "" {
		embedKey = h.cfg.LLMAPIKey
	}
	a.Configure(h.cfg.SearxngURL, memory.NewEmbedder(h.cfg.EmbedBaseURL, embedKey))
	rows, err := h.db.Query(ctx, `SELECT role,COALESCE(content,''),COALESCE(tool_name,''),COALESCE(tool_call_id,''),tool_calls FROM messages WHERE conversation_id=$1 ORDER BY sequence`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := []llm.Message{}
	for rows.Next() {
		var m llm.Message
		var calls []byte
		if err = rows.Scan(&m.Role, &m.Content, &m.Name, &m.ToolCallID, &calls); err != nil {
			return nil, err
		}
		if len(calls) > 0 {
			if err = json.Unmarshal(calls, &m.ToolCalls); err != nil {
				return nil, err
			}
		}
		history = append(history, m)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(history) > 0 && history[0].Role != "system" && prompt != "" {
		history = append([]llm.Message{{Role: "system", Content: prompt}}, history...)
	}
	a.SetHistory(history)
	return a, nil
}
func (h *WSHandler) saveHistory(ctx context.Context, convID string, messages []llm.Message) error {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		calls, err := json.Marshal(m.ToolCalls)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO messages(conversation_id,role,content,tool_name,tool_call_id,tool_calls) VALUES($1,$2,$3,$4,$5,$6)`, convID, m.Role, m.Content, m.Name, m.ToolCallID, calls); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
