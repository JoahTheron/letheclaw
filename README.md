# letheClaw – Strategic Memory for AI Agents

**The memory system that knows when to forget.**

letheClaw gives LLM-based agents a long-term memory layer: signal-derived criticality, provenance tracking, active forgetting, and offline consolidation. Built in Go; runs as a small API plus a Python embedding sidecar (text-to-vector only, not a full LLM).

---

## Design

- **Signal-derived criticality** – No LLM-set numbers; scores computed from events (corrections, failures, successes, references)
- **Provenance** – Source and confidence: observed, operator input, inferred; full event audit trail
- **Layered retrieval** – Hot cache (Redis) → warm index (Qdrant) → cold archive (PostgreSQL)
- **Active forgetting** – Decay for unused memories *(Phase 3b)*
- **Consolidation** – Background worker to compress and prune *(Phase 3a — done)*

---

## Architecture

```
                    ┌──────────────┐
  Agent / Client ──►│ letheClaw API│
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
 ┌────────────┐     ┌────────────┐    ┌────────────┐
 │ PostgreSQL │     │   Qdrant    │    │   Redis    │
 │ (metadata) │     │ (vectors)   │    │  (cache)   │
 └────────────┘     └────────────┘    └────────────┘
        │
        ▼
 ┌────────────┐
 │ Embeddings │  ← Python sidecar (all-MiniLM-L6-v2, 384-dim, CPU)
 └────────────┘
```

---

## Quick start

```bash
git clone <your-repo-url> letheclaw
cd letheclaw

docker compose up -d
# Wait ~60s for the embedding model to load on first run.

curl http://localhost:51234/health
```

- **API:** `http://localhost:51234` (only exposed port; internal services use Docker network)
- **Windows:** [WINDOWS.md](WINDOWS.md)
- **Full setup:** [QUICKSTART.md](QUICKSTART.md)

---

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness check (no DB query) |
| GET | `/stats` | Memory counts by state, consolidation run metrics |
| POST | `/memory` | Store a memory (content, tags, source, …) |
| GET | `/memory/search?q=...&limit=5` | Semantic search |
| GET | `/memory/recent` | Recent memories (cache or DB) |
| GET | `/memory/corrections?limit=10` | Recent corrected memories, ordered by last correction |
| POST | `/memory/:id/criticality` | Send signal (`{"signal": "failure\|success\|referenced", "reason": "..."}`) |
| POST | `/memory/:id/correction` | Mark operator correction (boosts criticality, increments counter) |
| GET | `/memory/:id/provenance` | Get memory plus full criticality event history |

---

## Signal-based criticality

Criticality is computed from events, not LLM-supplied numbers. The old `{"criticality": 0.8}` format is rejected with a guide that teaches the new contract.

**Signals and when to use them:**

- **`failure`** — The memory led to a bad outcome (wrong decision, caused an error, user flagged it as harmful). Weight: 0.3.
- **`success`** — The memory contributed to a good outcome (decision held up, info proved useful, user confirmed it helped). Weight: 0.1.
- **`referenced`** — The memory was considered but outcome is neutral/unclear. Also applied automatically by search. Weight: 0.05.

Operator corrections (`POST /memory/:id/correction`) are the strongest signal (weight: 0.5). Weights are configurable in `config/letheclaw.yaml`.

---

## Roadmap

### Phase 1 — Core storage (done)

- POST /memory (storage pipeline: PostgreSQL + Qdrant + Redis)
- GET /memory/search (semantic search via embeddings)
- GET /memory/recent (hot cache)
- Python embedding sidecar (all-MiniLM-L6-v2)

### Phase 2 — Signal-based criticality and corrections (done — v1.1)

- Signal-based criticality: `POST /memory/:id/criticality` accepts `{"signal": "..."}`, rejects raw numbers with a self-correcting guide
- Automatic reference counting on search (no LLM call)
- `GET /memory/corrections` endpoint (provenance-based, ordered by last correction)
- `POST /memory/:id/correction` and `GET /memory/:id/provenance`
- Criticality column dropped; score derived from event chain

### Phase 3a — Consolidation (done)

- Background goroutine in the API process (`api/services/consolidation.go`)
- Query Qdrant for memory pairs with similarity > `consolidation.similarity_threshold` (0.95)
- Merge similar memories: keep the richer content, union tags, delete duplicate from Qdrant + Postgres
- Track each run in `consolidation_runs` table (already in schema)
- Invalidate Redis cache for affected memory IDs
- Runs every `consolidation.interval_hours`, processes up to `consolidation.batch_size` per run

### Phase 3b — Decay worker (next)

- Background goroutine in the API process (`api/services/decay.go`)
- Query `decay_candidates` SQL view (active memories not accessed in > `decay.threshold_days`)
- Apply `criticality.decay_weight` (-0.05) per tick, clamp at `decay.min_criticality` floor (0.3)
- Insert `decay` events into `criticality_events` so provenance tracks the decline
- Update `memories.decay_score` column

### Phase 3c — Archive / delete

- Background goroutine in the API process (`api/services/retention.go`)
- Scan memories whose computed criticality fell below retention thresholds
- `criticality < retention.archive_threshold` (0.2) and state `active` → archive (remove from Qdrant + Redis, keep in Postgres)
- `criticality < retention.delete_threshold` (0.1) and state `archived` → delete (or soft-delete)
- Respect `retention.min_days` (30): never archive/delete memories younger than 30 days
- Log actions in `consolidation_runs` (`memories_archived`, `memories_deleted` columns)

### Phase 4 — Contradiction and supersession

- Detect when a newer memory contradicts an older one (e.g. "feature X exists" vs. "feature X was removed")
- On store: compare incoming content against semantically similar existing memories; flag potential contradictions
- Supersession chain: link the newer memory to the older one it replaces, so retrieval always surfaces the latest version
- On search: when a superseded memory matches, return the superseding memory instead (or annotate it as outdated)
- Agent-visible staleness indicator: search results include a `superseded_by` field when a fresher contradicting memory exists
- Provenance trail: record supersession events so the full history of a fact's evolution is auditable

---

## Project layout

```
letheclaw/
├── api/              # Go API
│   ├── main.go
│   ├── handlers/
│   ├── models/
│   ├── services/         # includes Phase 3 background workers
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── embeddings/       # Python text→vector service (sentence-transformers)
│   ├── app.py
│   ├── requirements.txt
│   └── Dockerfile
├── schema/           # PostgreSQL
│   ├── 001_init.sql
│   └── 002_signals.sql
├── config/
│   └── letheclaw.yaml
├── docker-compose.yml
├── Makefile
├── QUICKSTART.md
├── WINDOWS.md
├── INTEGRATION.md
├── skill/             # ClawHub skill (instructions + manifest)
├── TESTING.md         # Manual test guide
├── LICENSE
└── README.md
```

---

## Configuration

Main config: `config/letheclaw.yaml`. Overrides via env: `DATABASE_URL`, `REDIS_URL`, `QDRANT_URL`, `EMBEDDING_ENDPOINT`, etc. See the file and [QUICKSTART.md](QUICKSTART.md).

---

## Development

```bash
cd api && go mod tidy && go build -o letheclaw-api .
# Run with config and services (Postgres, Redis, Qdrant, embeddings) up.
./letheclaw-api
```

Tests: `go test ./...`

---

## License

MIT. See [LICENSE](LICENSE).
