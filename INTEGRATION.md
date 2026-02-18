# letheClaw + OpenClaw Integration Guide

This guide shows how to add letheClaw to your **existing** OpenClaw docker-compose.yml. There is no separate addon file: use the main **docker-compose.yml** in this repo as the source and adapt it as below.

---

## Step 1: Add letheClaw Services to Your Compose File

1. **Copy the full `services:` block** from this repo’s **docker-compose.yml** (postgres, qdrant, redis, letheclaw-embeddings, letheclaw-api) into your OpenClaw `docker-compose.yml`.

2. **Avoid name clashes** with existing services:
   - If you already have `postgres`, `redis`, or `qdrant`, rename the letheClaw ones (e.g. `letheclaw-postgres`, `letheclaw-redis`, `letheclaw-qdrant`) and set the API env vars to use those hostnames (e.g. `letheclaw-postgres:5432`, `letheclaw-redis:6379`, `letheclaw-qdrant:6333`).
   - If names don’t clash, you can keep `postgres`, `redis`, `qdrant` as-is.

3. **Point build and volumes at your letheClaw copy:**
   - Replace `./api` and `./embeddings` with the path to this repo (or the letheclaw folder) on your machine, e.g. `./workspace/letheclaw/api` and `./workspace/letheclaw/embeddings`.
   - Replace `./config` and `./schema` with the same base path (e.g. `./workspace/letheclaw/config`, `./workspace/letheclaw/schema`).

4. **Add the same volumes** as in docker-compose.yml: `postgres_data`, `qdrant_data`, `redis_data` (or prefixed names if you renamed the services).

---

## Step 2: Build and Start

```bash
cd /path/to/your-openclaw-project

# Build letheClaw services (paths from Step 1)
docker compose build letheclaw-api letheclaw-embeddings

# Start everything (existing + new services)
docker compose up -d

# Check status
docker compose ps
```

Expected: openclaw, browser, and all letheClaw services (postgres, qdrant, redis, letheclaw-embeddings, letheclaw-api) Up / healthy.
```

---

## Step 3: Test letheClaw API

**From your Windows host:**
```powershell
# Health check
curl http://localhost:51234/health

# Store a memory
curl -X POST http://localhost:51234/memory `
  -H "Content-Type: application/json" `
  -d '{
    "content": "Test memory from Windows",
    "source": "operator_input",
    "tags": ["test"],
    "operator": "Markus"
  }'
```

**From inside OpenClaw container:**
```bash
# OpenClaw agents can reach it at:
# http://letheclaw-api:8080/memory
```

---

## Step 4: Use from OpenClaw Agent

### Option A: Direct HTTP Calls (exec tool)

In your agent code (Python/Node.js/etc):

```python
import requests

# Store memory
response = requests.post(
    "http://letheclaw-api:8080/memory",
    json={
        "content": "Important decision made today",
        "source": "direct_observation",
        "tags": ["decision", "important"],
        "operator": "Markus",
        "session_key": "main:abc123"
    }
)

memory_id = response.json()["memory_id"]
```

```python
# Search memory
response = requests.get(
    "http://letheclaw-api:8080/memory/search",
    params={"q": "decision", "limit": 5}
)

results = response.json()["results"]
```

### Option B: OpenClaw Skill Wrapper (Recommended)

Create a skill that wraps letheClaw:

**File:** e.g. `skills/letheclaw/skill.sh` in your OpenClaw workspace

```bash
#!/bin/bash
# letheClaw Memory Skill

API="http://letheclaw-api:8080"

case "$1" in
  store)
    curl -X POST "$API/memory" \
      -H "Content-Type: application/json" \
      -d "$2"
    ;;
  search)
    curl "$API/memory/search?q=$2&limit=${3:-5}"
    ;;
  recent)
    curl "$API/memory/recent"
    ;;
  *)
    echo "Usage: $0 {store|search|recent} [args]"
    exit 1
    ;;
esac
```

**Usage in agent:**
```bash
# Store memory
./skills/letheclaw/skill.sh store '{"content":"Test","source":"operator_input"}'

# Search
./skills/letheclaw/skill.sh search "quantum threat"

# Recent
./skills/letheclaw/skill.sh recent
```

### Option C: Memory Tool Integration (Advanced)

Modify OpenClaw's memory tools to route through letheClaw:

**In OpenClaw config** (if it supports custom memory backends):
```json
{
  "memory": {
    "backend": "http",
    "endpoint": "http://letheclaw-api:8080"
  }
}
```

---

## Network Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Docker Network (default bridge)                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  openclaw:8080 (your agent runtime)                    │
│    ↓ HTTP                                              │
│  letheclaw-api:8080 (memory API)                       │
│    ├─→ letheclaw-embeddings:5000 (Python)             │
│    ├─→ letheclaw-postgres:5432 (metadata)              │
│    ├─→ letheclaw-qdrant:6333 (vectors)                 │
│    └─→ letheclaw-redis:6379 (cache)                    │
│                                                         │
└─────────────────────────────────────────────────────────┘
     ↑
     │ Port 51234 (exposed to host)
     │
   Windows host (for testing/debugging)
```

**Important:**
- OpenClaw → letheClaw: Use `http://letheclaw-api:8080` (internal network)
- Host → letheClaw: Use `http://localhost:51234` (exposed port)

---

## Environment Variables (Optional)

Add to your `openclaw` service environment if you want the agent to know about letheClaw:

```yaml
services:
  openclaw:
    environment:
      # ... your existing vars
      - LETHECLAW_API_URL=http://letheclaw-api:8080
```

Then in agent code:
```python
import os
api_url = os.getenv("LETHECLAW_API_URL", "http://letheclaw-api:8080")
```

---

## Troubleshooting

### letheClaw API not reachable from OpenClaw

**Check network:**
```powershell
# Exec into openclaw container
docker compose exec openclaw sh

# Test connectivity
curl http://letheclaw-api:8080/health
```

### Services not starting

**Check logs:**
```powershell
docker compose logs letheclaw-api
docker compose logs letheclaw-embeddings
docker compose logs letheclaw-postgres
```

### Build failures (go.sum issues)

See `BUILD-FIX.md` for detailed fixes. The key files needed:
- `api/go.mod` ✅
- `api/go.sum` ✅ (minimal seed)
- `api/.dockerignore` ✅ (excludes go.sum from COPY)

---

## Testing the Full Stack

```powershell
# 1. Check all services are up
docker compose ps

# 2. Test letheClaw health
curl http://localhost:51234/health

# 3. Store a memory from Windows
curl -X POST http://localhost:51234/memory `
  -H "Content-Type: application/json" `
  -d '{"content":"Integration test","source":"operator_input","tags":["test"]}'

# 4. Search from Windows
curl "http://localhost:51234/memory/search?q=test"

# 5. Test from OpenClaw container
docker compose exec openclaw curl http://letheclaw-api:8080/health
```

---

## Next Steps

1. **Add to compose file** - Copy services from this repo’s `docker-compose.yml` and adapt (see Step 1 above)
2. **Build services** - `docker compose build`
3. **Start everything** - `docker compose up -d`
4. **Test API** - `curl http://localhost:51234/health`
5. **Create skill wrapper** - Wrap letheClaw in an OpenClaw skill
6. **Use from agents** - Store/search memories via HTTP

---

## File Locations

```
C:\GithubProjects\persoclaw\openclaw-data\
├── docker-compose.yml          (add letheClaw services here)
├── workspace\
│   └── letheclaw\
│       ├── api\                (Go API source)
│       ├── embeddings\         (Python embedding service)
│       ├── schema\             (PostgreSQL init script)
│       ├── config\             (letheclaw.yaml)
│       └── docker-compose.yml        (copy service definitions into your compose)
```

---

**Status:** Ready to integrate. Follow Step 1-3 above to add letheClaw to your OpenClaw stack.
