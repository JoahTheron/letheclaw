package services

import (
	"os"

	"github.com/letheclaw/api/models"
	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from YAML file
func LoadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	applyEnvOverrides(&config)

	return &config, nil
}

func applyEnvOverrides(config *models.Config) {
	// Database URL override
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// Parse DATABASE_URL and update config.Database
		// For simplicity, we'll just note that this should be implemented
	}

	// Redis URL override
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		config.Redis.URL = redisURL
	}

	// Qdrant URL override
	if qdrantURL := os.Getenv("QDRANT_URL"); qdrantURL != "" {
		config.Qdrant.URL = qdrantURL
	}

	// Embedding config overrides
	if provider := os.Getenv("EMBEDDING_PROVIDER"); provider != "" {
		config.Embedding.Provider = provider
	}
	if endpoint := os.Getenv("EMBEDDING_ENDPOINT"); endpoint != "" {
		config.Embedding.Endpoint = endpoint
	}
}
