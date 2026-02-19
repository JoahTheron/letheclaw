package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/letheclaw/api/models"
	"github.com/letheclaw/api/services"
	"github.com/redis/go-redis/v9"
)

type MemoryHandler struct {
	DB        *sql.DB
	Redis     *redis.Client
	Qdrant    *services.QdrantClient
	Embedding *services.EmbeddingService
	Config    *models.Config
}

func NewMemoryHandler(db *sql.DB, redis *redis.Client, qdrant *services.QdrantClient, embedding *services.EmbeddingService, config *models.Config) *MemoryHandler {
	return &MemoryHandler{
		DB:        db,
		Redis:     redis,
		Qdrant:    qdrant,
		Embedding: embedding,
		Config:    config,
	}
}

// StoreMemory - POST /memory
func (h *MemoryHandler) StoreMemory(c *gin.Context) {
	var req models.StoreMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	if req.Confidence == 0 {
		req.Confidence = 0.5
	}
	if req.Source == "" {
		req.Source = "operator_input"
	}

	embedding, err := h.Embedding.GenerateEmbedding(req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate embedding", "details": err.Error()})
		return
	}

	memoryID := uuid.New()

	query := `
		INSERT INTO memories (
			id, content, source, confidence,
			tags, operator, session_key, context,
			created_at, last_accessed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id
	`

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	var returnedID uuid.UUID
	err = h.DB.QueryRow(
		query,
		memoryID,
		req.Content,
		req.Source,
		req.Confidence,
		pq.Array(tags),
		nullString(req.Operator),
		nullString(req.SessionKey),
		nullString(req.Context),
	).Scan(&returnedID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store memory in database", "details": err.Error()})
		return
	}

	metadata := map[string]interface{}{
		"tags":   req.Tags,
		"source": req.Source,
	}

	if err := h.Qdrant.StoreVector(memoryID.String(), embedding, metadata); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store vector", "details": err.Error()})
		return
	}

	ctx := context.Background()
	cacheKey := "memory:recent:" + memoryID.String()
	cacheData := map[string]interface{}{
		"id":      memoryID.String(),
		"content": req.Content,
		"tags":    req.Tags,
	}

	cacheJSON, _ := json.Marshal(cacheData)
	ttl := time.Duration(h.Config.Redis.TTLHours) * time.Hour
	h.Redis.Set(ctx, cacheKey, cacheJSON, ttl)

	c.JSON(http.StatusOK, gin.H{
		"status":              "success",
		"memory_id":           memoryID.String(),
		"stored_in":           []string{"postgresql", "qdrant", "redis"},
		"embedding_dimension": len(embedding),
	})
}

// SearchMemories - GET /memory/search
func (h *MemoryHandler) SearchMemories(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	limit := 5
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	embedding, err := h.Embedding.GenerateEmbedding(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate query embedding", "details": err.Error()})
		return
	}

	memoryIDs, err := h.Qdrant.SearchSimilar(embedding, limit, 0.0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search vectors", "details": err.Error()})
		return
	}

	if len(memoryIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"query":   query,
			"results": []interface{}{},
			"count":   0,
		})
		return
	}

	placeholders := ""
	args := make([]interface{}, len(memoryIDs))
	for i, id := range memoryIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	querySQL := `
		SELECT id, content, source, confidence, tags,
			   operator, session_key, created_at, last_accessed,
			   access_count, reference_count, correction_count
		FROM memories
		WHERE id::text IN (` + placeholders + `)
		AND state = 'active'
	`

	rows, err := h.DB.Query(querySQL, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch memories", "details": err.Error()})
		return
	}
	defer rows.Close()

	results := []gin.H{}
	returnedIDs := []uuid.UUID{}
	for rows.Next() {
		var memory models.Memory

		err := rows.Scan(
			&memory.ID,
			&memory.Content,
			&memory.Source,
			&memory.Confidence,
			pq.Array(&memory.Tags),
			&memory.Operator,
			&memory.SessionKey,
			&memory.CreatedAt,
			&memory.LastAccessed,
			&memory.AccessCount,
			&memory.ReferenceCount,
			&memory.CorrectionCount,
		)

		if err != nil {
			continue
		}

		returnedIDs = append(returnedIDs, memory.ID)

		results = append(results, gin.H{
			"id":               memory.ID,
			"content":          memory.Content,
			"tags":             memory.Tags,
			"source":           memory.Source,
			"reference_count":  memory.ReferenceCount,
			"correction_count": memory.CorrectionCount,
			"created_at":       memory.CreatedAt,
			"access_count":     memory.AccessCount,
		})
	}

	// Auto-increment reference_count and access tracking for each returned memory (no LLM call)
	for _, memID := range returnedIDs {
		h.DB.Exec(
			"UPDATE memories SET reference_count = reference_count + 1, last_accessed = NOW(), access_count = access_count + 1 WHERE id = $1",
			memID,
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

// GetRecentMemories - GET /memory/recent
func (h *MemoryHandler) GetRecentMemories(c *gin.Context) {
	ctx := context.Background()

	pattern := "memory:recent:*"
	keys, err := h.Redis.Keys(ctx, pattern).Result()

	if err == nil && len(keys) > 0 {
		results := []gin.H{}
		for _, key := range keys {
			data, err := h.Redis.Get(ctx, key).Result()
			if err == nil {
				var memory map[string]interface{}
				if json.Unmarshal([]byte(data), &memory) == nil {
					results = append(results, memory)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"source":  "cache",
			"results": results,
			"count":   len(results),
		})
		return
	}

	query := `
		SELECT id, content, tags, source, created_at
		FROM memories
		WHERE created_at >= NOW() - INTERVAL '24 hours'
		AND state = 'active'
		ORDER BY created_at DESC
		LIMIT 50
	`

	rows, err := h.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent memories", "details": err.Error()})
		return
	}
	defer rows.Close()

	results := []gin.H{}
	for rows.Next() {
		var id uuid.UUID
		var content, source string
		var tagList []string
		var createdAt time.Time

		if err := rows.Scan(&id, &content, pq.Array(&tagList), &source, &createdAt); err != nil {
			continue
		}

		results = append(results, gin.H{
			"id":         id,
			"content":    content,
			"tags":       tagList,
			"source":     source,
			"created_at": createdAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"source":  "database",
		"results": results,
		"count":   len(results),
	})
}

// UpdateCriticality - POST /memory/:id/criticality
// Signal-based. Rejects old {criticality, reason} format with a guide.
func (h *MemoryHandler) UpdateCriticality(c *gin.Context) {
	id, err := parseMemoryID(c)
	if err != nil {
		return
	}

	var raw map[string]interface{}
	body, _ := c.GetRawData()
	if err := json.Unmarshal(body, &raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON body"})
		return
	}

	// Reject old format: if body contains "criticality" (a float), teach the agent the new way
	if _, hasOld := raw["criticality"]; hasOld {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Raw criticality scores are no longer accepted.",
			"guide": `Send {"signal": "referenced|failure|success", "reason": "..."} instead. Signals are mapped to configured weights. The system computes criticality from events, not LLM-supplied numbers.`,
		})
		return
	}

	signalVal, hasSignal := raw["signal"]
	if !hasSignal {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing 'signal' field.",
			"guide": `Send {"signal": "referenced|failure|success", "reason": "..."}.`,
		})
		return
	}

	signal, ok := signalVal.(string)
	if !ok || !models.ValidSignals[signal] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "Invalid signal value.",
			"valid_signals": []string{"referenced", "failure", "success"},
		})
		return
	}

	reason, _ := raw["reason"].(string)

	weight := h.getSignalWeight(signal)

	var oldScore float64
	err = h.DB.QueryRow(
		`SELECT COALESCE(
			(SELECT new_score FROM criticality_events WHERE memory_id = $1 ORDER BY created_at DESC LIMIT 1),
			0.0
		)`, id,
	).Scan(&oldScore)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load current score", "details": err.Error()})
		return
	}

	var exists bool
	err = h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM memories WHERE id = $1 AND state = 'active')`, id).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
		return
	}

	newScore := oldScore + weight
	if newScore > 1 {
		newScore = 1
	}
	if newScore < 0 {
		newScore = 0
	}

	reasonPtr := nullString(reason)
	_, err = h.DB.Exec(
		`INSERT INTO criticality_events (memory_id, event_type, old_score, new_score, reason) VALUES ($1, $2, $3, $4, $5)`,
		id, signal, oldScore, newScore, reasonPtr,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record event", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"memory_id": id,
		"signal":    signal,
		"old_score": oldScore,
		"new_score": newScore,
	})
}

func (h *MemoryHandler) getSignalWeight(signal string) float64 {
	if h.Config == nil {
		defaults := map[string]float64{"referenced": 0.05, "failure": 0.3, "success": 0.1}
		return defaults[signal]
	}
	switch signal {
	case "referenced":
		if h.Config.Criticality.ReferencedWeight > 0 {
			return h.Config.Criticality.ReferencedWeight
		}
		return 0.05
	case "failure":
		if h.Config.Criticality.FailureWeight > 0 {
			return h.Config.Criticality.FailureWeight
		}
		return 0.3
	case "success":
		if h.Config.Criticality.SuccessWeight > 0 {
			return h.Config.Criticality.SuccessWeight
		}
		return 0.1
	default:
		return 0.0
	}
}

// MarkCorrection - POST /memory/:id/correction
func (h *MemoryHandler) MarkCorrection(c *gin.Context) {
	id, err := parseMemoryID(c)
	if err != nil {
		return
	}

	var oldScore float64
	err = h.DB.QueryRow(
		`SELECT COALESCE(
			(SELECT new_score FROM criticality_events WHERE memory_id = $1 ORDER BY created_at DESC LIMIT 1),
			0.0
		)`, id,
	).Scan(&oldScore)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load current score", "details": err.Error()})
		return
	}

	var exists bool
	err = h.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM memories WHERE id = $1 AND state = 'active')`, id).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
		return
	}

	weight := 0.5
	if h.Config != nil && h.Config.Criticality.OperatorCorrectionWeight > 0 {
		weight = h.Config.Criticality.OperatorCorrectionWeight
	}
	newScore := oldScore + weight
	if newScore > 1 {
		newScore = 1
	}

	_, err = h.DB.Exec(
		`UPDATE memories SET correction_count = correction_count + 1 WHERE id = $1`,
		id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update memory", "details": err.Error()})
		return
	}

	_, err = h.DB.Exec(
		`INSERT INTO criticality_events (memory_id, event_type, old_score, new_score) VALUES ($1, 'operator_correction', $2, $3)`,
		id, oldScore, newScore,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record event", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"memory_id": id,
		"old_score": oldScore,
		"new_score": newScore,
	})
}

// GetCorrections - GET /memory/corrections
// Returns corrected memories ordered by last correction time.
func (h *MemoryHandler) GetCorrections(c *gin.Context) {
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	query := `
		SELECT m.id, m.content, m.correction_count, m.tags, m.source, m.created_at,
			   MAX(ce.created_at) AS last_corrected_at
		FROM memories m
		INNER JOIN criticality_events ce ON ce.memory_id = m.id AND ce.event_type = 'operator_correction'
		WHERE m.state = 'active'
		GROUP BY m.id
		ORDER BY last_corrected_at DESC
		LIMIT $1
	`

	rows, err := h.DB.Query(query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch corrections", "details": err.Error()})
		return
	}
	defer rows.Close()

	results := []gin.H{}
	for rows.Next() {
		var r models.CorrectionResult
		if err := rows.Scan(&r.ID, &r.Content, &r.CorrectionCount, pq.Array(&r.Tags), &r.Source, &r.CreatedAt, &r.LastCorrectedAt); err != nil {
			continue
		}
		results = append(results, gin.H{
			"id":                r.ID,
			"content":           r.Content,
			"correction_count":  r.CorrectionCount,
			"tags":              r.Tags,
			"source":            r.Source,
			"created_at":        r.CreatedAt,
			"last_corrected_at": r.LastCorrectedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"results": results,
		"count":   len(results),
	})
}

// GetProvenance - GET /memory/:id/provenance
func (h *MemoryHandler) GetProvenance(c *gin.Context) {
	id, err := parseMemoryID(c)
	if err != nil {
		return
	}

	var m models.Memory
	err = h.DB.QueryRow(`
		SELECT id, content, source, confidence, tags,
		       operator, session_key, context, correction_count,
		       reference_count,
		       created_at, last_accessed, access_count, decay_score, state
		FROM memories WHERE id = $1 AND state = 'active'`,
		id,
	).Scan(
		&m.ID, &m.Content, &m.Source, &m.Confidence, pq.Array(&m.Tags),
		&m.Operator, &m.SessionKey, &m.Context, &m.CorrectionCount,
		&m.ReferenceCount,
		&m.CreatedAt, &m.LastAccessed, &m.AccessCount, &m.DecayScore, &m.State,
	)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load memory", "details": err.Error()})
		return
	}

	rows, err := h.DB.Query(
		`SELECT id, memory_id, event_type, old_score, new_score, reason, created_at
		 FROM criticality_events WHERE memory_id = $1 ORDER BY created_at ASC`,
		id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load events", "details": err.Error()})
		return
	}
	defer rows.Close()

	var events []models.CriticalityEvent
	for rows.Next() {
		var e models.CriticalityEvent
		var reason *string
		if err := rows.Scan(&e.ID, &e.MemoryID, &e.EventType, &e.OldScore, &e.NewScore, &reason, &e.CreatedAt); err != nil {
			continue
		}
		e.Reason = reason
		events = append(events, e)
	}

	c.JSON(http.StatusOK, models.Provenance{Memory: m, Events: events})
}

func parseMemoryID(c *gin.Context) (uuid.UUID, error) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memory id"})
		return uuid.Nil, err
	}
	return id, nil
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
