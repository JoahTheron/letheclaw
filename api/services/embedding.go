package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/letheclaw/api/models"
)

// EmbeddingService handles text embedding generation
type EmbeddingService struct {
	Provider string
	Endpoint string
	Model    string
	Timeout  time.Duration
	Client   *http.Client
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(config models.EmbeddingConfig) *EmbeddingService {
	return &EmbeddingService{
		Provider: config.Provider,
		Endpoint: config.Endpoint,
		Model:    config.Model,
		Timeout:  time.Duration(config.TimeoutSeconds) * time.Second,
		Client: &http.Client{
			Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		},
	}
}

// GenerateEmbedding generates an embedding vector for the given text
func (s *EmbeddingService) GenerateEmbedding(text string) ([]float32, error) {
	switch s.Provider {
	case "ollama":
		return s.generateOllamaEmbedding(text)
	case "python-sidecar":
		return s.generateSidecarEmbedding(text)
	case "openclaw-gateway":
		return s.generateOpenClawEmbedding(text)
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", s.Provider)
	}
}

// generateOllamaEmbedding generates embedding using Ollama
func (s *EmbeddingService) generateOllamaEmbedding(text string) ([]float32, error) {
	url := fmt.Sprintf("%s/api/embeddings", s.Endpoint)
	
	payload := map[string]interface{}{
		"model":  s.Model,
		"prompt": text,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := s.Client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}
	
	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result.Embedding, nil
}

// generateSidecarEmbedding generates embedding using Python sidecar
func (s *EmbeddingService) generateSidecarEmbedding(text string) ([]float32, error) {
	url := fmt.Sprintf("%s/embed", s.Endpoint)
	
	payload := map[string]interface{}{
		"text": text,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := s.Client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call sidecar: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar returned status %d", resp.StatusCode)
	}
	
	var result struct {
		Embedding []float32 `json:"embedding"`
		Dimension int       `json:"dimension"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result.Embedding, nil
}

// generateOpenClawEmbedding generates embedding using OpenClaw Gateway
func (s *EmbeddingService) generateOpenClawEmbedding(text string) ([]float32, error) {
	// TODO: Implement OpenClaw Gateway embedding
	return nil, fmt.Errorf("openclaw-gateway provider not yet implemented")
}
