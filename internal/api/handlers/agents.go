package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentHandler struct {
	db *pgxpool.Pool
}

func NewAgentHandler(db *pgxpool.Pool) *AgentHandler {
	return &AgentHandler{db: db}
}

type createAgentReq struct {
	Name         string `json:"name"          binding:"required"`
	SystemPrompt string `json:"system_prompt"`
}

type AgentRow struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SystemPrompt string    `json:"system_prompt"`
	Status       string    `json:"status"`
	SandboxID    string    `json:"sandbox_id"`
	NoVNCPort    string    `json:"novnc_port"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *AgentHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := h.db.Query(context.Background(),
		`SELECT id, name, system_prompt, status, COALESCE(sandbox_id,''), COALESCE(novnc_port,''), created_at
		 FROM agents WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var agents []AgentRow
	for rows.Next() {
		var a AgentRow
		rows.Scan(&a.ID, &a.Name, &a.SystemPrompt, &a.Status, &a.SandboxID, &a.NoVNCPort, &a.CreatedAt)
		agents = append(agents, a)
	}
	if agents == nil {
		agents = []AgentRow{}
	}
	c.JSON(http.StatusOK, agents)
}

func (h *AgentHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	var req createAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SystemPrompt == "" {
		req.SystemPrompt = "You are a helpful AI assistant with access to a Linux computer. Use your tools to complete tasks thoroughly and accurately."
	}

	id := uuid.New().String()
	_, err := h.db.Exec(context.Background(),
		`INSERT INTO agents (id, user_id, name, system_prompt) VALUES ($1, $2, $3, $4)`,
		id, userID, req.Name, req.SystemPrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "name": req.Name, "status": "idle"})
}

func (h *AgentHandler) Get(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Param("id")

	var a AgentRow
	err := h.db.QueryRow(context.Background(),
		`SELECT id, name, system_prompt, status, COALESCE(sandbox_id,''), COALESCE(novnc_port,''), created_at
		 FROM agents WHERE id = $1 AND user_id = $2`, agentID, userID).
		Scan(&a.ID, &a.Name, &a.SystemPrompt, &a.Status, &a.SandboxID, &a.NoVNCPort, &a.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *AgentHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Param("id")
	_, err := h.db.Exec(context.Background(),
		`DELETE FROM agents WHERE id = $1 AND user_id = $2`, agentID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
