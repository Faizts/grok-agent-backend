package handlers

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MCPHandler struct {
	db *pgxpool.Pool
}

func NewMCPHandler(db *pgxpool.Pool) *MCPHandler {
	return &MCPHandler{db: db}
}

type MCPServerRow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Transport string    `json:"transport"`
	URL       string    `json:"url"`
	Command   string    `json:"command"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type createMCPReq struct {
	Name      string `json:"name"      binding:"required"`
	Transport string `json:"transport" binding:"required,oneof=http stdio"`
	URL       string `json:"url"`
	Command   string `json:"command"`
}

func (h *MCPHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := h.db.Query(context.Background(),
		`SELECT id, name, transport, COALESCE(url,''), COALESCE(command,''), enabled, created_at
		 FROM mcp_servers WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var servers []MCPServerRow
	for rows.Next() {
		var s MCPServerRow
		rows.Scan(&s.ID, &s.Name, &s.Transport, &s.URL, &s.Command, &s.Enabled, &s.CreatedAt)
		servers = append(servers, s)
	}
	if servers == nil {
		servers = []MCPServerRow{}
	}
	c.JSON(http.StatusOK, servers)
}

func (h *MCPHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	var req createMCPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Transport != "http" {
		c.JSON(400, gin.H{"error": "only HTTP MCP transport is supported"})
		return
	}
	endpoint, err := url.Parse(req.URL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		c.JSON(400, gin.H{"error": "valid HTTP(S) MCP endpoint required"})
		return
	}
	id := uuid.New().String()
	_, err = h.db.Exec(context.Background(),
		`INSERT INTO mcp_servers (id, user_id, name, transport, url, command)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, userID, req.Name, req.Transport, req.URL, req.Command)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "server name already exists"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "name": req.Name})
}

func (h *MCPHandler) Toggle(c *gin.Context) {
	userID := c.GetString("user_id")
	serverID := c.Param("id")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := h.db.Exec(context.Background(),
		`UPDATE mcp_servers SET enabled = $1 WHERE id = $2 AND user_id = $3`,
		req.Enabled, serverID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": req.Enabled})
}

func (h *MCPHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	_, err := h.db.Exec(context.Background(),
		`DELETE FROM mcp_servers WHERE id = $1 AND user_id = $2`,
		c.Param("id"), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
