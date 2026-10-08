package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/grok-agent/backend/internal/config"
)

type AdminHandler struct {
	db  *pgxpool.Pool
	cfg *config.Config
}

func NewAdminHandler(db *pgxpool.Pool, cfg *config.Config) *AdminHandler {
	return &AdminHandler{db: db, cfg: cfg}
}

type UserAdminRow struct {
	ID                string   `json:"id"`
	Email             string   `json:"email"`
	Role              string   `json:"role"`
	Status            string   `json:"status"`
	MonthlyBudgetUSD  float64  `json:"monthly_budget_usd"`
	SpentThisMonthUSD float64  `json:"spent_this_month_usd"`
	AssignedModel     string   `json:"assigned_model"`
	AllowedModels     []string `json:"allowed_models"`
	CreatedAt         string   `json:"created_at"`
}

type updateUserReq struct {
	Role             *string   `json:"role"`
	Status           *string   `json:"status"`
	MonthlyBudgetUSD *float64  `json:"monthly_budget_usd"`
	AssignedModel    *string   `json:"assigned_model"`
	AllowedModels    *[]string `json:"allowed_models"`
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

	if (req.Role != nil && *req.Role != "admin" && *req.Role != "user") || (req.Status != nil && *req.Status != "active" && *req.Status != "suspended") || (req.MonthlyBudgetUSD != nil && *req.MonthlyBudgetUSD < 0) || (req.AssignedModel != nil && *req.AssignedModel == "") {
		c.JSON(400, gin.H{"error": "invalid user settings"})
		return
	}
	if userID == c.GetString("user_id") && ((req.Role != nil && *req.Role != "admin") || (req.Status != nil && *req.Status != "active")) {
		c.JSON(400, gin.H{"error": "cannot revoke your own administrator access"})
		return
	}
	tag, err := h.db.Exec(c.Request.Context(), `UPDATE users SET role=COALESCE($1,role), status=COALESCE($2,status),monthly_budget_usd=COALESCE($3,monthly_budget_usd),assigned_model=COALESCE($4,assigned_model),allowed_models=COALESCE($5,allowed_models) WHERE id=$6`, req.Role, req.Status, req.MonthlyBudgetUSD, req.AssignedModel, req.AllowedModels, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to update user"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": true})
}

func (h *AdminHandler) GlobalStats(c *gin.Context) {
	var totalUsers, activeSandboxes int
	var totalSpendUSD float64

	_ = h.db.QueryRow(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	_ = h.db.QueryRow(context.Background(), `SELECT COALESCE(SUM(spent_this_month_usd), 0) FROM users`).Scan(&totalSpendUSD)
	_ = h.db.QueryRow(context.Background(), `SELECT COUNT(DISTINCT sandbox_id) FROM agents WHERE status = 'running'`).Scan(&activeSandboxes)

	c.JSON(http.StatusOK, gin.H{
		"total_users":        totalUsers,
		"total_spend_usd":    totalSpendUSD,
		"active_sandboxes":   activeSandboxes,
		"provider_url":       h.cfg.LLMBaseURL,
		"default_budget_usd": 10.00,
	})
}

type ModelRateRow struct {
	Model           string  `json:"model"`
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

	if req.Model == "" || req.InputCostPer1K < 0 || req.OutputCostPer1K < 0 {
		c.JSON(400, gin.H{"error": "invalid model rate"})
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

// FetchModels proxies GET /models from the LLM provider.
func (h *AdminHandler) FetchModels(c *gin.Context) {
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", h.cfg.LLMBaseURL+"/models", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.cfg.LLMAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch models from provider"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "provider returned error"})
		return
	}

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode provider response"})
		return
	}

	c.JSON(http.StatusOK, data)
}
