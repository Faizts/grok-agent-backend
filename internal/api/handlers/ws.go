package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/grok-agent/backend/internal/agent"
	"github.com/grok-agent/backend/internal/config"
	"github.com/grok-agent/backend/internal/llm"
	"github.com/grok-agent/backend/internal/sandbox"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// agentPool keeps one agent instance per conversation so history persists.
var (
	agentPool   = map[string]*agent.Agent{}
	agentPoolMu sync.Mutex
)

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
	convID := c.Param("conversation_id")
	userID := c.GetString("user_id")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Load or create agent for this conversation
	a, err := h.getOrCreateAgent(c.Request.Context(), convID, userID)
	if err != nil {
		conn.WriteJSON(agent.Event{Type: agent.EventError, Content: "failed to start agent: " + err.Error()})
		return
	}

	writeMu := sync.Mutex{}
	writeEvent := func(e agent.Event) {
		data, _ := json.Marshal(e)
		writeMu.Lock()
		conn.WriteMessage(websocket.TextMessage, data)
		writeMu.Unlock()
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var incoming wsIncoming
		if err := json.Unmarshal(msgBytes, &incoming); err != nil || incoming.Message == "" {
			continue
		}

		// Persist user message to DB
		h.saveMessage(convID, "user", incoming.Message, "", nil)

		events := make(chan agent.Event, 256)
		var assistantContent string

		go func() {
			a.Run(context.Background(), incoming.Message, events)
		}()

		for ev := range events {
			writeEvent(ev)
			if ev.Type == agent.EventText {
				assistantContent += ev.Content
			}
		}

		// Persist assistant message to DB
		if assistantContent != "" {
			h.saveMessage(convID, "assistant", assistantContent, "", nil)
		}
	}
}

func (h *WSHandler) getOrCreateAgent(ctx context.Context, convID, userID string) (*agent.Agent, error) {
	agentPoolMu.Lock()
	defer agentPoolMu.Unlock()

	if a, ok := agentPool[convID]; ok {
		return a, nil
	}

	// Load agent config from DB via conversation → agent
	var agentID, agentName, systemPrompt, sandboxID string
	err := h.db.QueryRow(ctx,
		`SELECT ag.id, ag.name, ag.system_prompt, COALESCE(ag.sandbox_id, '')
		 FROM conversations c JOIN agents ag ON c.agent_id = ag.id
		 WHERE c.id = $1 AND c.user_id = $2`, convID, userID).
		Scan(&agentID, &agentName, &systemPrompt, &sandboxID)
	if err != nil {
		// Fallback: create a default agent on-the-fly
		agentID = uuid.New().String()
		agentName = "Assistant"
		systemPrompt = "You are a helpful AI assistant with access to a Linux computer. Use your tools to complete tasks thoroughly and accurately. When you need to run code, search the web, or manage files, use the appropriate tools."
	}

	// Provision sandbox
	sandboxMgr, err := sandbox.NewManager(h.cfg.SandboxImage, h.cfg.WorkspacePath)
	if err != nil {
		return nil, err
	}

	sandboxInfo, err := sandboxMgr.GetOrCreate(ctx, agentID)
	if err != nil {
		// Sandbox may not be available (image not built yet) — run without it
		sandboxInfo = &sandbox.SandboxInfo{ContainerID: ""}
	}

	// Update sandbox ID in DB
	if sandboxInfo.ContainerID != "" {
		h.db.Exec(ctx,
			`UPDATE agents SET sandbox_id = $1, novnc_port = $2, vnc_port = $3, status = 'running'
			 WHERE id = $4`,
			sandboxInfo.ContainerID, sandboxInfo.NoVNCPort, sandboxInfo.VNCPort, agentID)
	}

	llmClient := llm.NewClient(h.cfg.LLMBaseURL, h.cfg.LLMAPIKey, h.cfg.LLMModel)
	a := agent.New(agentID, agentName, systemPrompt, llmClient, sandboxMgr, sandboxInfo.ContainerID)

	agentPool[convID] = a
	return a, nil
}

func (h *WSHandler) saveMessage(convID, role, content, toolName string, toolInput interface{}) {
	h.db.Exec(context.Background(),
		`INSERT INTO messages (id, conversation_id, role, content, tool_name)
		 VALUES ($1, $2, $3, $4, $5)`,
		uuid.New().String(), convID, role, content, toolName)
}
