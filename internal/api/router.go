package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/grok-agent/backend/internal/api/handlers"
	"github.com/grok-agent/backend/internal/auth"
)

func setupRouter(s *Server) {
	r := s.router
	r.Use(corsMiddleware())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "1.0.0"})
	})

	v1 := r.Group("/api/v1")

	// ── Public ────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(s.pool, s.cfg.JWTSecret)
	v1.POST("/auth/register", authHandler.Register)
	v1.POST("/auth/login", authHandler.Login)

	// ── Protected ─────────────────────────────────────────────────
	protected := v1.Group("/", auth.Middleware(s.cfg.JWTSecret))
	{
		// Agents
		agentHandler := handlers.NewAgentHandler(s.pool)
		protected.GET("/agents", agentHandler.List)
		protected.POST("/agents", agentHandler.Create)
		protected.GET("/agents/:id", agentHandler.Get)
		protected.DELETE("/agents/:id", agentHandler.Delete)

		// Conversations
		convHandler := handlers.NewConversationHandler(s.pool)
		protected.GET("/conversations", convHandler.List)
		protected.POST("/conversations", convHandler.Create)
		protected.GET("/conversations/:id/messages", convHandler.Messages)

		// WebSocket — main chat + streaming
		wsHandler := handlers.NewWSHandler(s.pool, s.cfg)
		protected.GET("/ws/:conversation_id", wsHandler.Handle)

		// Approvals
		approvalHandler := handlers.NewApprovalHandler(s.pool)
		protected.GET("/approvals", approvalHandler.List)
		protected.POST("/approvals/:id/respond", approvalHandler.Respond)

		// Memory
		memHandler := handlers.NewMemoryHandler(s.pool)
		protected.GET("/agents/:id/memory", memHandler.List)
		protected.DELETE("/agents/:id/memory/:mem_id", memHandler.Delete)

		// Phase 7 ── Skills
		skillsHandler := handlers.NewSkillsHandler(s.pool)
		protected.GET("/skills", skillsHandler.List)
		protected.POST("/skills", skillsHandler.Create)
		protected.GET("/skills/:id", skillsHandler.Get)
		protected.DELETE("/skills/:id", skillsHandler.Delete)

		// Phase 7 ── MCP servers
		mcpHandler := handlers.NewMCPHandler(s.pool)
		protected.GET("/mcp", mcpHandler.List)
		protected.POST("/mcp", mcpHandler.Create)
		protected.PATCH("/mcp/:id/toggle", mcpHandler.Toggle)
		protected.DELETE("/mcp/:id", mcpHandler.Delete)

		// Phase 7 ── Usage / dashboard
		usageHandler := handlers.NewUsageHandler(s.pool)
		protected.GET("/usage/stats", usageHandler.Stats)
		protected.GET("/usage/tools", usageHandler.TopTools)
		protected.GET("/usage/daily", usageHandler.Daily)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
