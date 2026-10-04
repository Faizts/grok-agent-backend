package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/grok-agent/backend/internal/skills"
)

type SkillsHandler struct {
	store *skills.Store
}

func NewSkillsHandler(db *pgxpool.Pool) *SkillsHandler {
	return &SkillsHandler{store: skills.NewStore(db)}
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
	if req.AgentID == "" {
		req.AgentID = uuid.New().String() // placeholder when no agent context
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
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}
	c.JSON(http.StatusOK, sk)
}

func (h *SkillsHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.store.Delete(context.Background(), c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
