package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
)

func owns(c *gin.Context, db *pgxpool.Pool, kind, id string) bool {
	if _, err := uuid.Parse(id); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return false
	}
	queries := map[string]string{
		"agents":        `SELECT EXISTS(SELECT 1 FROM agents WHERE id=$1 AND user_id=$2)`,
		"conversations": `SELECT EXISTS(SELECT 1 FROM conversations c JOIN agents a ON a.id=c.agent_id WHERE c.id=$1 AND c.user_id=$2 AND a.user_id=$2)`,
	}
	query, ok := queries[kind]
	if !ok {
		return false
	}
	var found bool
	if err := db.QueryRow(c.Request.Context(), query, id, c.GetString("user_id")).Scan(&found); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "database unavailable"})
		return false
	}
	if !found {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
	}
	return found
}
