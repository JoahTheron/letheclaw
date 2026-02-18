package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/letheclaw/api/models"
)

// QdrantClient handles vector storage and semantic search
type QdrantClient struct {
	URL        string
	Collection string
	VectorSize int
	Client     *http.Client
}

// InitQdrant initializes the Qdrant client
func InitQdrant(config models.QdrantConfig) (*QdrantClient, error) {
	client := &QdrantClient{
		URL:        config.URL,
		Collection: config.Collection,
		VectorSize: config.VectorSize,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Create collection if it doesn't exist
	if err := client.CreateCollectionIfNotExists(); err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	return client, nil
}

// CreateCollectionIfNotExists creates the collection if it doesn't exist
func (c *QdrantClient) CreateCollectionIfNotExists() error {
	// Check if collection exists
	url := fmt.Sprintf("%s/collections/%s", c.URL, c.Collection)
	resp, err := c.Client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}
	defer resp.Body.Close()

	// If collection exists (200), return
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Create collection
	createURL := fmt.Sprintf("%s/collections/%s", c.URL, c.Collection)
	payload := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     c.VectorSize,
			"distance": "Cosine",
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal create request: %w", err)
	}

	req, err := http.NewRequest("PUT", createURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create collection, status: %d", resp.StatusCode)
	}

	return nil
}

// StoreVector stores a vector with metadata
func (c *QdrantClient) StoreVector(id string, vector []float32, metadata map[string]interface{}) error {
	url := fmt.Sprintf("%s/collections/%s/points", c.URL, c.Collection)
	
	payload := map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":      id,
				"vector":  vector,
				"payload": metadata,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to store vector: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to store vector, status: %d", resp.StatusCode)
	}

	return nil
}

// SearchSimilar searches for similar vectors
func (c *QdrantClient) SearchSimilar(vector []float32, limit int, minScore float64) ([]string, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", c.URL, c.Collection)
	
	payload := map[string]interface{}{
		"vector":      vector,
		"limit":       limit,
		"with_vector": false,
		"with_payload": true,
		"score_threshold": minScore,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.Client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed, status: %d", resp.StatusCode)
	}

	var result struct {
		Result []struct {
			ID    interface{} `json:"id"`
			Score float64     `json:"score"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	ids := make([]string, 0, len(result.Result))
	for _, item := range result.Result {
		// Qdrant IDs can be string or number, handle both
		switch v := item.ID.(type) {
		case string:
			ids = append(ids, v)
		case float64:
			ids = append(ids, fmt.Sprintf("%d", int(v)))
		default:
			ids = append(ids, fmt.Sprintf("%v", v))
		}
	}

	return ids, nil
}
