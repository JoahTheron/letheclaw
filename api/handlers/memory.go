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

	// Set defaults
	if req.Confidence == 0 {
		req.Confidence = 0.5
	}
	if req.Source == "" {
		req.Source = "operator_input"
	}

	// 1. Generate embedding
	embedding, err := h.Embedding.GenerateEmbedding(req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate embedding", "details": err.Error()})
		return
	}

	// 2. Store in PostgreSQL
	memoryID := uuid.New()
	criticality := 0.5 // Default criticality

	query := `
		INSERT INTO memories (
			id, content, source, confidence, criticality,
			tags, operator, session_key, context, created_at, last_accessed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
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
		criticality,
		pq.Array(tags),
		nullString(req.Operator),
		nullString(req.SessionKey),
		nullString(req.Context),
	).Scan(&returnedID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store memory in database", "details": err.Error()})
		return
	}

	// 3. Store vector in Qdrant
	metadata := map[string]interface{}{
		"criticality": criticality,
		"tags":        req.Tags,
		"source":      req.Source,
	}

	if err := h.Qdrant.StoreVector(memoryID.String(), embedding, metadata); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store vector", "details": err.Error()})
		return
	}

	// 4. Cache in Redis (store for 24h)
	ctx := context.Background()
	cacheKey := "memory:recent:" + memoryID.String()
	cacheData := map[string]interface{}{
		"id":          memoryID.String(),
		"content":     req.Content,
		"criticality": criticality,
		"tags":        req.Tags,
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

	minCriticality := 0.0
	if minCritStr := c.Query("min_criticality"); minCritStr != "" {
		if parsed, err := strconv.ParseFloat(minCritStr, 64); err == nil {
			minCriticality = parsed
		}
	}
	// 1. Generate query embedding
	embedding, err := h.Embedding.GenerateEmbedding(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate query embedding", "details": err.Error()})
		return
	}

	// 2. Search Qdrant for similar vectors
	memoryIDs, err := h.Qdrant.SearchSimilar(embedding, limit, minCriticality)
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

	// 3. Fetch full memory metadata from PostgreSQL
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
		SELECT id, content, source, confidence, criticality, tags,
			   operator, session_key, created_at, last_accessed, access_count
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
	for rows.Next() {
		var memory models.Memory

		err := rows.Scan(
			&memory.ID,
			&memory.Content,
			&memory.Source,
			&memory.Confidence,
			&memory.Criticality,
			pq.Array(&memory.Tags),
			&memory.Operator,
			&memory.SessionKey,
			&memory.CreatedAt,
			&memory.LastAccessed,
			&memory.AccessCount,
		)

		if err != nil {
			continue
		}

		results = append(results, gin.H{
			"id":           memory.ID,
			"content":      memory.Content,
			"criticality":  memory.Criticality,
			"tags":         memory.Tags,
			"source":       memory.Source,
			"created_at":   memory.CreatedAt,
			"access_count": memory.AccessCount,
		})

		// Update last_accessed
		h.DB.Exec("UPDATE memories SET last_accessed = NOW(), access_count = access_count + 1 WHERE id = $1", memory.ID)
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

	// Try Redis cache first
	pattern := "memory:recent:*"
	keys, err := h.Redis.Keys(ctx, pattern).Result()
	
	if err == nil && len(keys) > 0 {
		// Cache hit
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

	// Fallback to PostgreSQL (last 24h)
	query := `
		SELECT id, content, criticality, tags, source, created_at
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
		var criticality float64
		var tagList []string
		var createdAt time.Time

		if err := rows.Scan(&id, &content, &criticality, pq.Array(&tagList), &source, &createdAt); err != nil {
			continue
		}

		results = append(results, gin.H{
			"id":          id,
			"content":     content,
			"criticality": criticality,
			"tags":        tagList,
			"source":      source,
			"created_at":  createdAt,
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
func (h *MemoryHandler) UpdateCriticality(c *gin.Context) {
	memoryID := c.Param("id")
	
	var req models.UpdateCriticalityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// TODO: Implement criticality update logic
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Criticality update not yet implemented",
		"memory_id": memoryID,
	})
}

// MarkCorrection - POST /memory/:id/correction
func (h *MemoryHandler) MarkCorrection(c *gin.Context) {
	memoryID := c.Param("id")

	// TODO: Implement operator correction tracking
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Correction marking not yet implemented",
		"memory_id": memoryID,
	})
}

// GetProvenance - GET /memory/:id/provenance
func (h *MemoryHandler) GetProvenance(c *gin.Context) {
	memoryID := c.Param("id")

	// TODO: Implement provenance retrieval
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Provenance retrieval not yet implemented",
		"memory_id": memoryID,
	})
}

// Helper function for nullable strings
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
