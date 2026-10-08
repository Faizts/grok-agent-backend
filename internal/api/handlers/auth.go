package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/grok-agent/backend/internal/auth"
)

type AuthHandler struct {
	db     *pgxpool.Pool
	secret string
}

func NewAuthHandler(db *pgxpool.Pool, secret string) *AuthHandler {
	return &AuthHandler{db: db, secret: secret}
}

type registerReq struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginReq struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	ctx := c.Request.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": "database unavailable"})
		return
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(734218)`); err != nil {
		c.JSON(500, gin.H{"error": "registration unavailable"})
		return
	}
	// Serialize first-admin creation.
	var userCount int
	if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		c.JSON(500, gin.H{"error": "registration unavailable"})
		return
	}

	role := "user"
	if userCount == 0 {
		role = "admin"
	}

	var userID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password, role, monthly_budget_usd)
		 VALUES ($1, $2, $3, 10.00) RETURNING id`,
		req.Email, hash, role,
	).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	if err = tx.Commit(ctx); err != nil {
		c.JSON(500, gin.H{"error": "registration unavailable"})
		return
	}
	token, err := auth.GenerateToken(userID, role, h.secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":                token,
		"user_id":              userID,
		"role":                 role,
		"monthly_budget_usd":   10.00,
		"spent_this_month_usd": 0.00,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID, hash, role, status string
	var budget, spent float64
	err := h.db.QueryRow(context.Background(),
		`SELECT id, password, role, status, monthly_budget_usd, spent_this_month_usd FROM users WHERE email = $1`,
		req.Email,
	).Scan(&userID, &hash, &role, &status, &budget, &spent)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if status == "suspended" {
		c.JSON(http.StatusForbidden, gin.H{"error": "account suspended by administrator"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(userID, role, h.secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":                token,
		"user_id":              userID,
		"role":                 role,
		"status":               status,
		"monthly_budget_usd":   budget,
		"spent_this_month_usd": spent,
	})
}
