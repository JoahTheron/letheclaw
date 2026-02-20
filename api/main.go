package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/letheclaw/api/handlers"
	"github.com/letheclaw/api/services"
)

func main() {
	// Load configuration
	config, err := services.LoadConfig("config/letheclaw.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize services
	db, err := services.InitDB(config.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	redis, err := services.InitRedis(config.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	qdrant, err := services.InitQdrant(config.Qdrant)
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}

	embedding := services.NewEmbeddingService(config.Embedding)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	statsHandler := handlers.NewStatsHandler(db)
	memoryHandler := handlers.NewMemoryHandler(db, redis, qdrant, embedding, config)

	// Setup Gin router
	if config.API.LogLevel == "info" {
		gin.SetMode(gin.ReleaseMode)
	}
	
	router := gin.Default()

	router.GET("/health", healthHandler.HealthCheck)
	router.HEAD("/health", healthHandler.HealthCheck)
	router.GET("/stats", statsHandler.GetStats)

	// Phase 1: Core endpoints
	router.POST("/memory", memoryHandler.StoreMemory)
	router.GET("/memory/search", memoryHandler.SearchMemories)
	router.GET("/memory/recent", memoryHandler.GetRecentMemories)
	router.GET("/memory/corrections", memoryHandler.GetCorrections)

	// Phase 2: Signal-based criticality, corrections, provenance
	router.POST("/memory/:id/criticality", memoryHandler.UpdateCriticality)
	router.POST("/memory/:id/correction", memoryHandler.MarkCorrection)
	router.GET("/memory/:id/provenance", memoryHandler.GetProvenance)

	// Phase 3a: Consolidation background worker
	consolidationWorker := services.NewConsolidationWorker(db, redis, qdrant, config)
	go consolidationWorker.Start(context.Background())

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = fmt.Sprintf("%d", config.API.Port)
	}

	log.Printf("letheClaw API starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
