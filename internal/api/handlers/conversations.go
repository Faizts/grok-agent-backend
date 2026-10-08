package handlers

import (
	"context"
	"encoding/json"
	"github.com/grok-agent/backend/internal/llm"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConversationHandler struct {
	db *pgxpool.Pool
}

func NewConversationHandler(db *pgxpool.Pool) *ConversationHandler {
	return &ConversationHandler{db: db}
}

type createConvReq struct {
	AgentID string `json:"agent_id" binding:"required"`
	Title   string `json:"title"`
}

type ConversationRow struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	MessageCount int64     `json:"message_count"`
	LastActivity time.Time `json:"last_activity"`
}

type MessageRow struct {
	Attachments    json.RawMessage `json:"attachments,omitempty"`
	ID             string          `json:"id"`
	ConversationID string          `json:"conversation_id"`
	Role           string          `json:"role"`
	Content        string          `json:"content"`
	ToolName       string          `json:"tool_name,omitempty"`
	ToolCallID     string          `json:"tool_call_id,omitempty"`
	ToolInput      interface{}     `json:"tool_input,omitempty"`
	ToolResult     string          `json:"tool_result,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

func (h *ConversationHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := h.db.Query(c.Request.Context(),
		`SELECT c.id, c.agent_id, c.title, c.created_at,
		        COUNT(m.id), COALESCE(MAX(m.created_at), c.created_at) AS last_activity
		 FROM conversations c LEFT JOIN messages m ON m.conversation_id = c.id
		 WHERE c.user_id = $1
		 GROUP BY c.id ORDER BY last_activity DESC, c.created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var convs []ConversationRow
	for rows.Next() {
		var r ConversationRow
		if err := rows.Scan(&r.ID, &r.AgentID, &r.Title, &r.CreatedAt, &r.MessageCount, &r.LastActivity); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load conversations"})
			return
		}
		convs = append(convs, r)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load conversations"})
		return
	}
	if convs == nil {
		convs = []ConversationRow{}
	}
	c.JSON(http.StatusOK, convs)
}

func (h *ConversationHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	var req createConvReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !owns(c, h.db, "agents", req.AgentID) {
		return
	}
	if req.Title == "" {
		req.Title = "New conversation"
	}
	id := uuid.New().String()
	_, err := h.db.Exec(context.Background(),
		`INSERT INTO conversations (id, agent_id, user_id, title) VALUES ($1, $2, $3, $4)`,
		id, req.AgentID, userID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "title": req.Title, "agent_id": req.AgentID})
}

func (h *ConversationHandler) Messages(c *gin.Context) {
	convID := c.Param("id")
	if !owns(c, h.db, "conversations", convID) {
		return
	}
	rows, err := h.db.Query(context.Background(),
		`SELECT id, conversation_id, role, COALESCE(content,''), COALESCE(tool_name,''), COALESCE(tool_call_id,''),tool_calls, created_at, attachments
		 FROM messages WHERE conversation_id = $1 ORDER BY sequence ASC`, convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var msgs []MessageRow
	for rows.Next() {
		var m MessageRow
		var calls []byte
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolName, &m.ToolCallID, &calls, &m.CreatedAt, &m.Attachments); err != nil {
			c.JSON(500, gin.H{"error": "invalid message"})
			return
		}
		if m.Role == "tool" {
			m.Role = "tool_result"
			m.ToolResult = m.Content
		}
		var toolCalls []llm.ToolCall
		if len(calls) > 0 {
			if err := json.Unmarshal(calls, &toolCalls); err != nil {
				c.JSON(500, gin.H{"error": "invalid tool history"})
				return
			}
		}
		if m.Content != "" || len(toolCalls) == 0 {
			msgs = append(msgs, m)
		}
		for _, tc := range toolCalls {
			var input interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &input)
			msgs = append(msgs, MessageRow{ID: m.ID + ":" + tc.ID, ConversationID: m.ConversationID, Role: "tool_call", ToolName: tc.Function.Name, ToolCallID: tc.ID, ToolInput: input, CreatedAt: m.CreatedAt})
		}
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load messages"})
		return
	}
	if msgs == nil {
		msgs = []MessageRow{}
	}
	c.JSON(http.StatusOK, msgs)
}

func (h *ConversationHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if !owns(c, h.db, "conversations", id) {
		return
	}
	var row ConversationRow
	if err := h.db.QueryRow(c.Request.Context(), `SELECT id,agent_id,title,created_at FROM conversations WHERE id=$1`, id).Scan(&row.ID, &row.AgentID, &row.Title, &row.CreatedAt); err != nil {
		c.JSON(500, gin.H{"error": "failed to load conversation"})
		return
	}
	c.JSON(200, row)
}
