# letheClaw – Strategic Memory for AI Agents

**The memory system that knows when to forget.**

letheClaw gives LLM-based agents a long-term memory layer: signal-derived criticality, provenance tracking, active forgetting, and offline consolidation. Built in Go; runs as a small API plus a Python embedding sidecar (text-to-vector only, not a full LLM).

---

## Design

- **Signal-derived criticality** – No LLM-set numbers; scores computed from events (corrections, failures, successes, references)
- **Provenance** – Source and confidence: observed, operator input, inferred; full event audit trail
- **Layered retrieval** – Hot cache (Redis) → warm index (Qdrant) → cold archive (PostgreSQL)
- **Active forgetting** – Decay for unused memories *(Phase 3)*
- **Consolidation** – Background worker to compress and prune *(Phase 3)*

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
| GET | `/health` | Service health |
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

### Phase 3 — Decay and consolidation (next)

- Background worker that applies `decay_weight` to memories unused for > `threshold_days`
- Inserts `decay` events into `criticality_events` so provenance tracks the decline
- Respects `min_criticality` floor from config
- Archive/delete thresholds from `retention` config
- Consolidation: compress similar memories (similarity > threshold), prune duplicates
- Consolidation tracking via `consolidation_runs` table (already in schema)

---

## Project layout

```
letheclaw/
├── api/              # Go API
│   ├── main.go
│   ├── handlers/
│   ├── models/
│   ├── services/
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
