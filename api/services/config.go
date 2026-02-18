package services

import (
	"net/url"
	"os"
	"strconv"
	"strings"

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
	// Database URL override (so integrated compose can use a different host, e.g. letheclaw-postgres)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if err := applyDatabaseURL(config, dbURL); err != nil {
			// Log but don't fail: fall back to YAML config
			// (caller has no logger; rely on InitDB failing with clear message if wrong host)
			_ = err
		}
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

// applyDatabaseURL parses DATABASE_URL (e.g. postgresql://user:pass@host:5432/dbname?sslmode=disable)
// and sets config.Database so InitDB uses the correct host when integrated into another compose.
func applyDatabaseURL(config *models.Config, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	host := u.Hostname()
	config.Database.Host = host
	if port := u.Port(); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Database.Port = p
		}
	}
	if u.User != nil {
		config.Database.User = u.User.Username()
		if p, ok := u.User.Password(); ok {
			config.Database.Password = p
		}
	}
	if u.Path != "" {
		config.Database.Name = strings.TrimPrefix(u.Path, "/")
	}
	return nil
}
