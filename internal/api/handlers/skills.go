package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"

	"github.com/grok-agent/backend/internal/skills"
)

type SkillsHandler struct {
	store *skills.Store
	db    *pgxpool.Pool
}

func NewSkillsHandler(db *pgxpool.Pool) *SkillsHandler {
	return &SkillsHandler{store: skills.NewStore(db), db: db}
}

type createSkillReq struct {
	AgentID     string   `json:"agent_id"`
	Name        string   `json:"name"        binding:"required"`
	Description string   `json:"description"`
	Content     string   `json:"content"     binding:"required"`
	Tags        []string `json:"tags"`
}

func (h *SkillsHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")
	if agentID != "" && !owns(c, h.db, "agents", agentID) {
		return
	}
	list, err := h.store.List(context.Background(), userID, agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *SkillsHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	var req createSkillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Content) > 65536 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Skill content must be at most 64 KB"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	if req.AgentID != "" && !owns(c, h.db, "agents", req.AgentID) {
		return
	}
	sk, err := h.store.Save(context.Background(), req.AgentID, userID, req.Name, req.Description, req.Content, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sk)
}

func (h *SkillsHandler) Get(c *gin.Context) {
	sk, err := h.store.Get(context.Background(), c.Param("id"))
	if err != nil || (!sk.Builtin && sk.UserID != c.GetString("user_id")) {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}
	c.JSON(http.StatusOK, sk)
}

func (h *SkillsHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	if strings.HasPrefix(c.Param("id"), "builtin:") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Built-in skills cannot be deleted. Create a custom copy to adapt them."})
		return
	}
	if err := h.store.Delete(context.Background(), c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
