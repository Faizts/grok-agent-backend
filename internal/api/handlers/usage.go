package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/grok-agent/backend/internal/usage"
)

type UsageHandler struct {
	tracker *usage.Tracker
}

func NewUsageHandler(db *pgxpool.Pool) *UsageHandler {
	return &UsageHandler{tracker: usage.NewTracker(db)}
}

func (h *UsageHandler) Stats(c *gin.Context) {
	userID := c.GetString("user_id")
	stats, err := h.tracker.GetStats(context.Background(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *UsageHandler) TopTools(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	tools, err := h.tracker.GetTopTools(context.Background(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tools)
}

func (h *UsageHandler) Daily(c *gin.Context) {
	userID := c.GetString("user_id")
	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	daily, err := h.tracker.GetDailyUsage(context.Background(), userID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, daily)
}
