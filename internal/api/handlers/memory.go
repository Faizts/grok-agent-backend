package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MemoryHandler struct {
	db *pgxpool.Pool
}

func NewMemoryHandler(db *pgxpool.Pool) *MemoryHandler {
	return &MemoryHandler{db: db}
}

type MemoryRow struct {
	ID        string    `json:"id"`
	Store     string    `json:"store"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *MemoryHandler) List(c *gin.Context) {
	agentID := c.Param("id")
	if !owns(c, h.db, "agents", agentID) {
		return
	}
	store := c.Query("store") // optional filter: main, solutions, skills, fragments

	var rows interface {
		Next() bool
		Scan(...any) error
		Close()
	}
	var err error
	if store != "" {
		rows, err = h.db.Query(context.Background(),
			`SELECT id, store, content, created_at FROM memories
			 WHERE agent_id = $1 AND store = $2 ORDER BY created_at DESC LIMIT 100`,
			agentID, store)
	} else {
		rows, err = h.db.Query(context.Background(),
			`SELECT id, store, content, created_at FROM memories
			 WHERE agent_id = $1 ORDER BY created_at DESC LIMIT 100`, agentID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var memories []MemoryRow
	for rows.Next() {
		var m MemoryRow
		rows.Scan(&m.ID, &m.Store, &m.Content, &m.CreatedAt)
		memories = append(memories, m)
	}
	if memories == nil {
		memories = []MemoryRow{}
	}
	c.JSON(http.StatusOK, memories)
}

func (h *MemoryHandler) Delete(c *gin.Context) {
	agentID := c.Param("id")
	if !owns(c, h.db, "agents", agentID) {
		return
	}
	memID := c.Param("mem_id")
	_, err := h.db.Exec(context.Background(),
		`DELETE FROM memories WHERE id = $1 AND agent_id = $2`, memID, agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
