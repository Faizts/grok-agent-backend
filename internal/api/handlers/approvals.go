package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ApprovalHandler struct {
	db *pgxpool.Pool
}

func NewApprovalHandler(db *pgxpool.Pool) *ApprovalHandler {
	return &ApprovalHandler{db: db}
}

type ApprovalRow struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	ToolName  string    `json:"tool_name"`
	Action    string    `json:"action"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *ApprovalHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := h.db.Query(context.Background(),
		`SELECT a.id, a.agent_id, a.tool_name, a.action, a.status, a.created_at
		 FROM approvals a
		 JOIN agents ag ON a.agent_id = ag.id
		 WHERE ag.user_id = $1 AND a.status = 'pending'
		 ORDER BY a.created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var approvals []ApprovalRow
	for rows.Next() {
		var r ApprovalRow
		rows.Scan(&r.ID, &r.AgentID, &r.ToolName, &r.Action, &r.Status, &r.CreatedAt)
		approvals = append(approvals, r)
	}
	if approvals == nil {
		approvals = []ApprovalRow{}
	}
	c.JSON(http.StatusOK, approvals)
}

type respondReq struct {
	Decision string `json:"decision" binding:"required,oneof=approved denied always"`
}

func (h *ApprovalHandler) Respond(c *gin.Context) {
	approvalID := c.Param("id")
	var req respondReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := req.Decision
	if status == "always" {
		// Get the tool_name + agent_id and add to tool_rules
		var toolName, agentID string
		h.db.QueryRow(context.Background(),
			`SELECT tool_name, agent_id FROM approvals WHERE id = $1`, approvalID).
			Scan(&toolName, &agentID)

		h.db.Exec(context.Background(),
			`INSERT INTO tool_rules (agent_id, tool_name, rule)
			 VALUES ($1, $2, 'always_allow')
			 ON CONFLICT (agent_id, tool_name) DO UPDATE SET rule = 'always_allow'`,
			agentID, toolName)
		status = "approved"
	}

	_, err := h.db.Exec(context.Background(),
		`UPDATE approvals SET status = $1 WHERE id = $2`, status, approvalID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}
