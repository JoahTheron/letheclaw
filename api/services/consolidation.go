package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/letheclaw/api/models"
	"github.com/redis/go-redis/v9"
)

type ConsolidationWorker struct {
	DB     *sql.DB
	Redis  *redis.Client
	Qdrant *QdrantClient
	Config *models.Config
}

func NewConsolidationWorker(db *sql.DB, redisClient *redis.Client, qdrant *QdrantClient, config *models.Config) *ConsolidationWorker {
	return &ConsolidationWorker{
		DB:     db,
		Redis:  redisClient,
		Qdrant: qdrant,
		Config: config,
	}
}

// Start launches the consolidation ticker loop. Blocks until ctx is cancelled.
func (w *ConsolidationWorker) Start(ctx context.Context) {
	interval := time.Duration(w.Config.Consolidation.IntervalHours) * time.Hour
	if interval <= 0 {
		interval = 1 * time.Hour
	}

	log.Printf("[consolidation] worker started, interval=%v, similarity_threshold=%.2f, batch_size=%d",
		interval, w.Config.Consolidation.SimilarityThreshold, w.Config.Consolidation.BatchSize)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[consolidation] worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *ConsolidationWorker) runOnce(ctx context.Context) {
	runID := uuid.New()
	startedAt := time.Now()

	if _, err := w.DB.ExecContext(ctx,
		`INSERT INTO consolidation_runs (id, started_at, status) VALUES ($1, $2, 'running')`,
		runID, startedAt,
	); err != nil {
		log.Printf("[consolidation] failed to create run record: %v", err)
		return
	}

	processed, compressed, runErr := w.consolidate(ctx)

	status := "completed"
	var errText *string
	if runErr != nil {
		status = "failed"
		s := runErr.Error()
		errText = &s
	}

	if _, err := w.DB.ExecContext(ctx,
		`UPDATE consolidation_runs
		 SET completed_at = $1, memories_processed = $2, memories_compressed = $3, status = $4, error = $5
		 WHERE id = $6`,
		time.Now(), processed, compressed, status, errText, runID,
	); err != nil {
		log.Printf("[consolidation] failed to update run record: %v", err)
	}

	log.Printf("[consolidation] run=%s processed=%d compressed=%d status=%s", runID, processed, compressed, status)
}

func (w *ConsolidationWorker) consolidate(ctx context.Context) (processed int, compressed int, err error) {
	batchSize := w.Config.Consolidation.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}
	threshold := w.Config.Consolidation.SimilarityThreshold
	if threshold <= 0 {
		threshold = 0.95
	}

	rows, err := w.DB.QueryContext(ctx,
		`SELECT id, content, tags FROM memories WHERE state = 'active' ORDER BY created_at ASC LIMIT $1`,
		batchSize,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	type memRow struct {
		ID      string
		Content string
		Tags    []string
	}

	var memories []memRow
	for rows.Next() {
		var m memRow
		var tagsArr []byte
		if err := rows.Scan(&m.ID, &m.Content, &tagsArr); err != nil {
			return 0, 0, fmt.Errorf("scan memory: %w", err)
		}
		m.Tags = parsePgTextArray(string(tagsArr))
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("iterate memories: %w", err)
	}

	consumed := make(map[string]bool)

	for _, m := range memories {
		if consumed[m.ID] {
			continue
		}
		processed++

		vector, err := w.Qdrant.GetVector(m.ID)
		if err != nil {
			log.Printf("[consolidation] skip %s: get vector failed: %v", m.ID, err)
			continue
		}

		// +1 limit to account for the memory itself appearing in results
		results, err := w.Qdrant.SearchSimilarWithScores(vector, 10+1, threshold)
		if err != nil {
			log.Printf("[consolidation] skip %s: search failed: %v", m.ID, err)
			continue
		}

		var duplicates []string
		for _, r := range results {
			if r.ID == m.ID || consumed[r.ID] {
				continue
			}
			duplicates = append(duplicates, r.ID)
		}

		if len(duplicates) == 0 {
			continue
		}

		if err := w.mergeDuplicates(ctx, m.ID, m.Content, m.Tags, duplicates); err != nil {
			log.Printf("[consolidation] merge failed for %s: %v", m.ID, err)
			continue
		}

		for _, d := range duplicates {
			consumed[d] = true
		}
		compressed += len(duplicates)
	}

	return processed, compressed, nil
}

// mergeDuplicates keeps the canonical memory and absorbs duplicates into it.
func (w *ConsolidationWorker) mergeDuplicates(ctx context.Context, canonicalID, canonicalContent string, canonicalTags []string, duplicateIDs []string) error {
	tagSet := make(map[string]bool)
	for _, t := range canonicalTags {
		tagSet[t] = true
	}

	// Collect tags from duplicates and pick the longest content as canonical
	bestContent := canonicalContent
	for _, dupID := range duplicateIDs {
		var content string
		var tagsArr []byte
		err := w.DB.QueryRowContext(ctx,
			`SELECT content, tags FROM memories WHERE id = $1`, dupID,
		).Scan(&content, &tagsArr)
		if err != nil {
			log.Printf("[consolidation] skip duplicate %s: %v", dupID, err)
			continue
		}
		for _, t := range parsePgTextArray(string(tagsArr)) {
			tagSet[t] = true
		}
		if len(content) > len(bestContent) {
			bestContent = content
		}
	}

	mergedTags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		mergedTags = append(mergedTags, t)
	}

	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE memories SET content = $1, tags = $2 WHERE id = $3`,
		bestContent, pgTextArray(mergedTags), canonicalID,
	); err != nil {
		return fmt.Errorf("update canonical: %w", err)
	}

	for _, dupID := range duplicateIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM memories WHERE id = $1`, dupID); err != nil {
			return fmt.Errorf("delete duplicate %s: %w", dupID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if err := w.Qdrant.DeletePoints(duplicateIDs); err != nil {
		log.Printf("[consolidation] qdrant cleanup failed for duplicates of %s: %v", canonicalID, err)
	}

	redisCtx := context.Background()
	for _, dupID := range duplicateIDs {
		w.Redis.Del(redisCtx, "memory:recent:"+dupID)
	}

	return nil
}

func parsePgTextArray(s string) []string {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, `"`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func pgTextArray(tags []string) string {
	if len(tags) == 0 {
		return "{}"
	}
	escaped := make([]string, len(tags))
	for i, t := range tags {
		escaped[i] = `"` + strings.ReplaceAll(t, `"`, `\"`) + `"`
	}
	return "{" + strings.Join(escaped, ",") + "}"
}
