package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/letheclaw/api/models"
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
		"version":      models.Version,
		"memory_count": memoryCount,
	})
}
