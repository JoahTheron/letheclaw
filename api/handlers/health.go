package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/letheclaw/api/models"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "letheClaw API",
		"version": models.Version,
	})
}

type StatsHandler struct {
	DB *sql.DB
}

func NewStatsHandler(db *sql.DB) *StatsHandler {
	return &StatsHandler{DB: db}
}

func (s *StatsHandler) GetStats(c *gin.Context) {
	memories := gin.H{"active": 0, "archived": 0, "deleted": 0}
	rows, err := s.DB.QueryContext(c.Request.Context(),
		`SELECT state, COUNT(*) FROM memories GROUP BY state`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var state string
			var count int64
			if rows.Scan(&state, &count) == nil {
				memories[state] = count
			}
		}
	}

	consolidation := gin.H{
		"last_run":         nil,
		"last_status":      nil,
		"total_runs":       0,
		"total_compressed": 0,
	}

	var lastRun *time.Time
	var lastStatus *string
	if s.DB.QueryRowContext(c.Request.Context(),
		`SELECT started_at, status FROM consolidation_runs ORDER BY started_at DESC LIMIT 1`,
	).Scan(&lastRun, &lastStatus) == nil {
		consolidation["last_run"] = lastRun
		consolidation["last_status"] = lastStatus
	}

	var totalRuns int64
	var totalCompressed int64
	if s.DB.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*), COALESCE(SUM(memories_compressed), 0) FROM consolidation_runs WHERE status = 'completed'`,
	).Scan(&totalRuns, &totalCompressed) == nil {
		consolidation["total_runs"] = totalRuns
		consolidation["total_compressed"] = totalCompressed
	}

	c.JSON(http.StatusOK, gin.H{
		"memories":      memories,
		"consolidation": consolidation,
	})
}
