package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/grok-agent/backend/internal/config"
	"github.com/grok-agent/backend/internal/sandbox"
)

type AdminHandler struct {
	db     *pgxpool.Pool
	cfg    *config.Config
	sanMgr *sandbox.Manager
}

func NewAdminHandler(db *pgxpool.Pool, cfg *config.Config) *AdminHandler {
	sm, _ := sandbox.NewManager(cfg.SandboxImage, cfg.WorkspacePath)
	return &AdminHandler{db: db, cfg: cfg, sanMgr: sm}
}

type UserAdminRow struct {
	ID                 string   `json:"id"`
	Email              string   `json:"email"`
	Role               string   `json:"role"`
	Status             string   `json:"status"`
	MonthlyBudgetUSD   float64  `json:"monthly_budget_usd"`
	SpentThisMonthUSD  float64  `json:"spent_this_month_usd"`
	AssignedModel      string   `json:"assigned_model"`
	AllowedModels      []string `json:"allowed_models"`
	CreatedAt          string   `json:"created_at"`
}

type updateUserReq struct {
	Role              *string   `json:"role"`
	Status            *string   `json:"status"`
	MonthlyBudgetUSD  *float64  `json:"monthly_budget_usd"`
	AssignedModel     *string   `json:"assigned_model"`
	AllowedModels     *[]string `json:"allowed_models"`
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	rows, err := h.db.Query(context.Background(), `
		SELECT id, email, role, status, monthly_budget_usd, spent_this_month_usd,
		       COALESCE(assigned_model, 'gpt-4o'), COALESCE(allowed_models, '{}'), created_at::TEXT
		FROM users ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var users []UserAdminRow
	for rows.Next() {
		var u UserAdminRow
		rows.Scan(&u.ID, &u.Email, &u.Role, &u.Status, &u.MonthlyBudgetUSD, &u.SpentThisMonthUSD, &u.AssignedModel, &u.AllowedModels, &u.CreatedAt)
		users = append(users, u)
	}
	if users == nil {
		users = []UserAdminRow{}
	}
	c.JSON(http.StatusOK, users)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Role != nil {
		h.db.Exec(context.Background(), `UPDATE users SET role = $1 WHERE id = $2`, *req.Role, userID)
	}
	if req.Status != nil {
		h.db.Exec(context.Background(), `UPDATE users SET status = $1 WHERE id = $2`, *req.Status, userID)
	}
	if req.MonthlyBudgetUSD != nil {
		h.db.Exec(context.Background(), `UPDATE users SET monthly_budget_usd = $1 WHERE id = $2`, *req.MonthlyBudgetUSD, userID)
	}
	if req.AssignedModel != nil {
		h.db.Exec(context.Background(), `UPDATE users SET assigned_model = $1 WHERE id = $2`, *req.AssignedModel, userID)
	}
	if req.AllowedModels != nil {
		h.db.Exec(context.Background(), `UPDATE users SET allowed_models = $1 WHERE id = $2`, *req.AllowedModels, userID)
	}

	c.JSON(http.StatusOK, gin.H{"updated": true})
}

func (h *AdminHandler) GlobalStats(c *gin.Context) {
	var totalUsers, activeSandboxes int
	var totalSpendUSD float64

	_ = h.db.QueryRow(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	_ = h.db.QueryRow(context.Background(), `SELECT COALESCE(SUM(spent_this_month_usd), 0) FROM users`).Scan(&totalSpendUSD)
	_ = h.db.QueryRow(context.Background(), `SELECT COUNT(*) FROM agents WHERE status = 'running'`).Scan(&activeSandboxes)

	c.JSON(http.StatusOK, gin.H{
		"total_users":          totalUsers,
		"total_spend_usd":      totalSpendUSD,
		"active_sandboxes":     activeSandboxes,
		"omniroute_url":        h.cfg.LLMBaseURL,
		"default_budget_usd":   10.00,
	})
}

type ModelRateRow struct {
	Model          string  `json:"model"`
	InputCostPer1K  float64 `json:"input_cost_per_1k"`
	OutputCostPer1K float64 `json:"output_cost_per_1k"`
}

func (h *AdminHandler) ListModelRates(c *gin.Context) {
	rows, err := h.db.Query(context.Background(), `SELECT model, input_cost_per_1k, output_cost_per_1k FROM model_rates ORDER BY model ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var rates []ModelRateRow
	for rows.Next() {
		var mr ModelRateRow
		rows.Scan(&mr.Model, &mr.InputCostPer1K, &mr.OutputCostPer1K)
		rates = append(rates, mr)
	}
	if rates == nil {
		rates = []ModelRateRow{}
	}
	c.JSON(http.StatusOK, rates)
}

func (h *AdminHandler) SaveModelRate(c *gin.Context) {
	var req ModelRateRow
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := h.db.Exec(context.Background(), `
		INSERT INTO model_rates (model, input_cost_per_1k, output_cost_per_1k)
		VALUES ($1, $2, $3)
		ON CONFLICT (model) DO UPDATE SET input_cost_per_1k = $2, output_cost_per_1k = $3, updated_at = NOW()`,
		req.Model, req.InputCostPer1K, req.OutputCostPer1K,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"saved": true})
}
