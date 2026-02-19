-- letheClaw Initial Schema
-- Phase 1+2: Core storage, signal-based criticality, provenance

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Main memories table
CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    content TEXT NOT NULL,

    -- Metadata
    source VARCHAR(50) NOT NULL CHECK (source IN ('direct_observation', 'operator_input', 'inferred', 'system_generated')),
    confidence DECIMAL(3,2) NOT NULL DEFAULT 0.50 CHECK (confidence >= 0 AND confidence <= 1),
    tags TEXT[] DEFAULT '{}',

    -- Provenance
    operator VARCHAR(255),
    session_key VARCHAR(255),
    context TEXT,
    correction_count INTEGER DEFAULT 0,

    -- Signal counters
    reference_count INTEGER DEFAULT 0,

    -- Access tracking
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_accessed TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    access_count INTEGER DEFAULT 0,
    decay_score DECIMAL(3,2) DEFAULT 0.00 CHECK (decay_score >= 0 AND decay_score <= 1),

    -- State
    state VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'archived', 'deleted')),

    CONSTRAINT memories_content_not_empty CHECK (LENGTH(content) > 0)
);

-- Criticality events table (audit log — criticality is derived from these events)
CREATE TABLE criticality_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    memory_id UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL CHECK (event_type IN ('operator_correction', 'failure', 'success', 'decay', 'manual_boost', 'referenced')),
    old_score DECIMAL(3,2) NOT NULL,
    new_score DECIMAL(3,2) NOT NULL,
    reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Consolidation runs table (tracking)
CREATE TABLE consolidation_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    memories_processed INTEGER DEFAULT 0,
    memories_archived INTEGER DEFAULT 0,
    memories_deleted INTEGER DEFAULT 0,
    memories_compressed INTEGER DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    error TEXT
);

-- Indexes for performance
CREATE INDEX idx_memories_reference_count ON memories(reference_count DESC);
CREATE INDEX idx_memories_correction_count ON memories(correction_count DESC);
CREATE INDEX idx_memories_created_at ON memories(created_at DESC);
CREATE INDEX idx_memories_last_accessed ON memories(last_accessed DESC);
CREATE INDEX idx_memories_state ON memories(state);
CREATE INDEX idx_memories_tags ON memories USING GIN(tags);
CREATE INDEX idx_memories_operator ON memories(operator);
CREATE INDEX idx_memories_session_key ON memories(session_key);

CREATE INDEX idx_criticality_events_memory_id ON criticality_events(memory_id);
CREATE INDEX idx_criticality_events_created_at ON criticality_events(created_at DESC);

-- Function: Update last_accessed on SELECT
CREATE OR REPLACE FUNCTION update_memory_access() RETURNS TRIGGER AS $$
BEGIN
    NEW.last_accessed = NOW();
    NEW.access_count = NEW.access_count + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- View: Recent high-signal memories (by computed criticality from events)
CREATE VIEW recent_critical_memories AS
SELECT
    m.id,
    m.content,
    m.tags,
    m.operator,
    m.reference_count,
    m.correction_count,
    m.created_at,
    m.last_accessed,
    COALESCE(latest_event.new_score, 0.0) AS computed_criticality
FROM memories m
LEFT JOIN LATERAL (
    SELECT new_score FROM criticality_events
    WHERE memory_id = m.id
    ORDER BY created_at DESC LIMIT 1
) latest_event ON true
WHERE
    m.state = 'active'
    AND m.created_at >= NOW() - INTERVAL '7 days'
ORDER BY COALESCE(latest_event.new_score, 0.0) DESC, m.created_at DESC;

-- View: Decay candidates
CREATE VIEW decay_candidates AS
SELECT
    m.id,
    m.content,
    m.last_accessed,
    m.access_count,
    m.decay_score,
    m.reference_count,
    m.correction_count,
    EXTRACT(DAY FROM NOW() - m.last_accessed) AS days_since_access,
    COALESCE(latest_event.new_score, 0.0) AS computed_criticality
FROM memories m
LEFT JOIN LATERAL (
    SELECT new_score FROM criticality_events
    WHERE memory_id = m.id
    ORDER BY created_at DESC LIMIT 1
) latest_event ON true
WHERE
    m.state = 'active'
    AND m.last_accessed < NOW() - INTERVAL '90 days'
ORDER BY m.last_accessed ASC;

COMMENT ON TABLE memories IS 'Core memory storage with provenance and signal-derived criticality';
COMMENT ON TABLE criticality_events IS 'Audit log for criticality score changes (criticality is derived from the latest event)';
COMMENT ON TABLE consolidation_runs IS 'Background worker execution tracking';
