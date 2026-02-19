package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	DB *sql.DB
}

func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{DB: db}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	var memoryCount int64 = -1
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM memories WHERE state = 'active'`).Scan(&memoryCount)

	c.JSON(http.StatusOK, gin.H{
		"status":       "healthy",
		"service":      "letheClaw API",
		"version":      "1.0.0",
		"memory_count": memoryCount,
	})
}
