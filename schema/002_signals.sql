-- Migration: Signal-based criticality, reference counting
-- Safe to run on both old schema (has criticality column) and new schema (already migrated).
-- PostgreSQL runs all files in /docker-entrypoint-initdb.d alphabetically on first boot.

-- 1. Drop views FIRST (they may reference the old criticality column)
DROP VIEW IF EXISTS recent_critical_memories;
DROP VIEW IF EXISTS decay_candidates;

-- 2. Drop the static criticality column if it exists
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'memories' AND column_name = 'criticality') THEN
        ALTER TABLE memories DROP COLUMN criticality;
    END IF;
END $$;

-- 3. Add reference_count if it doesn't exist
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'memories' AND column_name = 'reference_count') THEN
        ALTER TABLE memories ADD COLUMN reference_count INTEGER DEFAULT 0;
    END IF;
END $$;

-- 4. Expand criticality_events.event_type to include 'referenced'
ALTER TABLE criticality_events DROP CONSTRAINT IF EXISTS criticality_events_event_type_check;
ALTER TABLE criticality_events ADD CONSTRAINT criticality_events_event_type_check
    CHECK (event_type IN ('operator_correction', 'failure', 'success', 'decay', 'manual_boost', 'referenced'));

-- 5. Drop the old criticality index if it exists
DROP INDEX IF EXISTS idx_memories_criticality;

-- 6. New indexes
CREATE INDEX IF NOT EXISTS idx_memories_reference_count ON memories(reference_count DESC);
CREATE INDEX IF NOT EXISTS idx_memories_correction_count ON memories(correction_count DESC);

-- 7. Recreate views with the new schema
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
