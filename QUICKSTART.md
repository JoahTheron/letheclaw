# letheClaw Quick Start

## Prerequisites

1. **Docker & Docker Compose** installed
   - Linux/Mac: Docker Engine + Docker Compose plugin
   - Windows: Docker Desktop with WSL 2 backend
2. **Go 1.21+** installed (for local development only)
3. **(Optional) jq** for pretty JSON output: `apt install jq` or `brew install jq`

**Windows Users:** See [WINDOWS.md](WINDOWS.md) for detailed Windows setup.

---

## Option A: Full Docker Stack (Recommended for Testing)

### Step 1: Start Everything

```bash
cd letheclaw

# Start all services (PostgreSQL, Qdrant, Redis, Embeddings, API)
make start

# Or manually:
docker compose up -d

# Check logs
make logs
# Or: docker compose logs -f
```

### Step 2: Wait for Services to Start

The Python embedding service takes ~60 seconds to download the model on first run.

```bash
# Check service health
make test

# Or manually check each:
curl http://localhost:51234/health  # API (only port exposed to host)
```

### Step 3: Try the API

Use the [Manual Testing](#manual-testing) section below (curl or your client). In the full stack, the API is called by your agent or other services, not by local scripts.

---

## Option B: Local API Development

### Step 1: Start Infrastructure Only

```bash
cd letheclaw

# Start PostgreSQL, Qdrant, Redis, Embeddings (but not API)
make infra

# Or manually:
docker compose up -d postgres qdrant redis embeddings
```

### Step 2: Build and Run API Locally

```bash
cd api

# Download dependencies
go mod tidy

# Build
go build -o letheclaw-api .

# Run (connects to Docker infrastructure)
./letheclaw-api
```

### Step 3: Test

In another terminal, use the [Manual Testing](#manual-testing) curl examples below (or call the API from your client).

---

## Manual Testing

### 1. Health Check

```bash
curl http://localhost:51234/health
```

Expected:
```json
{
  "service": "letheClaw API",
  "status": "healthy",
  "version": "1.0.0",
  "memory_count": 0
}
```

### 2. Store a Memory

```bash
curl -X POST http://localhost:51234/memory \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Quantum computing threatens RSA signatures",
    "source": "operator_input",
    "tags": ["security", "quantum"],
    "operator": "Markus",
    "session_key": "main:test123"
  }'
```

Expected:
```json
{
  "embedding_dimension": 384,
  "memory_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "success",
  "stored_in": ["postgresql", "qdrant", "redis"]
}
```

### 3. Search Memories

```bash
curl "http://localhost:51234/memory/search?q=quantum%20threat&limit=5"
```

Expected:
```json
{
  "count": 1,
  "query": "quantum threat",
  "results": [
    {
      "access_count": 1,
      "content": "Quantum computing threatens RSA signatures",
      "created_at": "2026-02-18T16:00:00Z",
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "source": "operator_input",
      "tags": ["security", "quantum"],
      "reference_count": 0,
      "correction_count": 0
    }
  ],
  "status": "success"
}
```

### 4. Get Recent Memories

```bash
curl http://localhost:51234/memory/recent
```

---

## Inspecting the Database

### View Tables

```bash
make db-tables

# Or manually:
docker compose exec postgres psql -U letheclaw -d letheclaw -c "\dt"
```

### Interactive SQL Shell

```bash
make db-shell

# Or manually:
docker compose exec postgres psql -U letheclaw -d letheclaw
```

Example queries:

```sql
-- See all memories
SELECT id, content, reference_count, correction_count, tags FROM memories LIMIT 10;

-- Count memories
SELECT COUNT(*) FROM memories;

-- View recent memories
SELECT content, created_at FROM memories ORDER BY created_at DESC LIMIT 5;
```

---

## Troubleshooting

### "Connection refused" to Embeddings Service

The Python sidecar takes time to start (downloads 80MB model on first run).

```bash
# Check logs
docker compose logs embeddings

# Wait for: "Model loaded successfully. Embedding dimension: 384"
```

### "Failed to store vector" (Qdrant Error)

```bash
# Check Qdrant logs
docker compose logs qdrant

# Qdrant is not exposed to the host; check API and qdrant container logs
```

### "Failed to store memory in database"

```bash
# Check PostgreSQL logs
docker compose logs postgres

# Verify schema was initialized
make db-tables
```

### Port Already in Use

```bash
# Check what's using the API port
lsof -i :51234

# Stop conflicting services or change ports in docker-compose.yml
```

---

## Upgrading from v1.0 to v1.1

If you have an existing database (created before v1.1), run the migration to add `reference_count` and remove the old `criticality` column:

```bash
docker compose exec postgres psql -U letheclaw -d letheclaw -f /docker-entrypoint-initdb.d/002_signals.sql
```

Fresh installs (`docker compose down -v && docker compose up -d`) need no action — both schema files run automatically on first boot.

---

## Stopping & Cleanup

### Stop Services (Keep Data)

```bash
make stop
# Or: docker compose down
```

### Stop Services & Delete All Data

```bash
make clean
# Or: docker compose down -v
```

This removes:
- PostgreSQL data (`postgres_data` volume)
- Qdrant vectors (`qdrant_data` volume)
- Redis cache (`redis_data` volume)

---

## Roadmap

### Phase 1 — Core storage ✅

- [x] POST /memory (storage pipeline)
- [x] GET /memory/search (semantic search)
- [x] GET /memory/recent (hot cache)
- [x] Python embedding service
- [x] PostgreSQL + Qdrant + Redis integration

### Phase 2 — Signal-based criticality and corrections ✅ (v1.1)

- [x] Signal-based criticality (`{"signal": "failure|success|referenced"}`, rejects raw numbers)
- [x] Automatic reference counting on search
- [x] GET /memory/corrections (provenance-based, ordered by last correction)
- [x] POST /memory/:id/correction, GET /memory/:id/provenance
- Manual test flow: [TESTING.md](TESTING.md)
- ClawHub skill: [skill/](skill/). OpenClaw integration: [INTEGRATION.md](INTEGRATION.md).

### Phase 3a — Consolidation ✅

- [x] Background goroutine in the API (`api/services/consolidation.go`)
- [x] Merge similar memories (similarity > 0.95), prune duplicates
- [x] Track runs in `consolidation_runs` table

### Phase 3b — Decay worker (next)

- [ ] Background goroutine in the API (`api/services/decay.go`)
- [ ] Query `decay_candidates` view, apply `decay_weight` per tick
- [ ] Insert `decay` events into `criticality_events`
- [ ] Respect `min_criticality` floor

### Phase 3c — Archive / delete

- [ ] Background goroutine in the API (`api/services/retention.go`)
- [ ] Archive memories below `archive_threshold`, remove from Qdrant + Redis
- [ ] Delete memories below `delete_threshold`
- [ ] Respect `retention.min_days` (never touch memories younger than 30 days)

### Phase 4 — Contradiction and supersession

- [ ] Detect contradictions between new and existing memories on store
- [ ] Supersession chain: link newer memory to the older one it replaces
- [ ] Search returns the latest version when a superseded memory matches
- [ ] `superseded_by` field in search results for staleness visibility
- [ ] Provenance trail for supersession events

---

## Development Workflow

1. **Make changes** to code
2. **Rebuild**:
   - Docker: `docker compose up -d --build`
   - Local: `cd api && go build -o letheclaw-api .`
3. **Test**: use the Manual Testing curl examples above
4. **Check logs**: `make logs`

---

**Issues?** Check logs: `docker compose logs [service-name]`
