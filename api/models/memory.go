package models

import (
	"time"

	"github.com/google/uuid"
)

// Memory represents a stored memory with metadata
type Memory struct {
	ID              uuid.UUID  `json:"id"`
	Content         string     `json:"content"`
	Source          string     `json:"source"`
	Confidence      float64    `json:"confidence"`
	Criticality     float64    `json:"criticality"`
	Tags            []string   `json:"tags"`
	Operator        *string    `json:"operator,omitempty"`
	SessionKey      *string    `json:"session_key,omitempty"`
	Context         *string    `json:"context,omitempty"`
	CorrectionCount int        `json:"correction_count"`
	CreatedAt       time.Time  `json:"created_at"`
	LastAccessed    time.Time  `json:"last_accessed"`
	AccessCount     int        `json:"access_count"`
	DecayScore      float64    `json:"decay_score"`
	State           string     `json:"state"`
}

// StoreMemoryRequest is the request body for POST /memory
type StoreMemoryRequest struct {
	Content    string   `json:"content" binding:"required"`
	Source     string   `json:"source"`
	Confidence float64  `json:"confidence"`
	Tags       []string `json:"tags"`
	Operator   string   `json:"operator"`
	SessionKey string   `json:"session_key"`
	Context    string   `json:"context"`
}

// SearchMemoriesRequest is the query parameters for GET /memory/search
type SearchMemoriesRequest struct {
	Query          string   `form:"q" binding:"required"`
	Limit          int      `form:"limit"`
	MinCriticality float64  `form:"min_criticality"`
	Tags           []string `form:"tags"`
	Source         string   `form:"source"`
}

// UpdateCriticalityRequest is the request body for POST /memory/:id/criticality
type UpdateCriticalityRequest struct {
	Criticality float64 `json:"criticality" binding:"required"`
	Reason      string  `json:"reason"`
}

// CriticalityEvent represents a criticality score change
type CriticalityEvent struct {
	ID        uuid.UUID `json:"id"`
	MemoryID  uuid.UUID `json:"memory_id"`
	EventType string    `json:"event_type"`
	OldScore  float64   `json:"old_score"`
	NewScore  float64   `json:"new_score"`
	Reason    *string   `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Provenance represents the full provenance chain of a memory
type Provenance struct {
	Memory Memory             `json:"memory"`
	Events []CriticalityEvent `json:"events"`
}
