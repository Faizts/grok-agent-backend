package handlers

import (
	"context"
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
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type MessageRow struct {
	ID             string      `json:"id"`
	ConversationID string      `json:"conversation_id"`
	Role           string      `json:"role"`
	Content        string      `json:"content"`
	ToolName       string      `json:"tool_name,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}

func (h *ConversationHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := h.db.Query(context.Background(),
		`SELECT id, agent_id, title, created_at FROM conversations
		 WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var convs []ConversationRow
	for rows.Next() {
		var r ConversationRow
		rows.Scan(&r.ID, &r.AgentID, &r.Title, &r.CreatedAt)
		convs = append(convs, r)
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
	rows, err := h.db.Query(context.Background(),
		`SELECT id, conversation_id, role, COALESCE(content,''), COALESCE(tool_name,''), created_at
		 FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC`, convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var msgs []MessageRow
	for rows.Next() {
		var m MessageRow
		rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolName, &m.CreatedAt)
		msgs = append(msgs, m)
	}
	if msgs == nil {
		msgs = []MessageRow{}
	}
	c.JSON(http.StatusOK, msgs)
}
