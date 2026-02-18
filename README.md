# letheClaw – Strategic Memory for AI Agents

**The memory system that knows when to forget.**

letheClaw gives LLM-based agents a human-like memory layer: hierarchical storage, active forgetting, provenance tracking, and (planned) offline consolidation. Built in Go; runs as a small API plus a Python embedding sidecar (text-to-vector only, not a full LLM).

---

## Design

- **Active forgetting** – Decay for unused, low-criticality memories  
- **Criticality** – Scores from operator corrections, failures, successes  
- **Layered retrieval** – Hot cache (Redis) → warm index (Qdrant) → cold archive (PostgreSQL)  
- **Provenance** – Source and confidence: observed, operator input, inferred  
- **Consolidation** *(planned)* – Background worker to compress and prune

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

## API (Phase 1)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Service health |
| POST | `/memory` | Store a memory (content, tags, source, …) |
| GET | `/memory/search?q=...&limit=5` | Semantic search |
| GET | `/memory/recent` | Recent memories (cache or DB) |

Phase 2 endpoints (criticality, correction, provenance) are stubbed and documented in [INTEGRATION.md](INTEGRATION.md).

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
│   └── 001_init.sql
├── config/
│   └── letheclaw.yaml
├── docker-compose.yml
├── docker-compose-addon.yml   # Snippet to merge into an existing compose
├── Makefile
├── QUICKSTART.md
├── WINDOWS.md
├── INTEGRATION.md
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
