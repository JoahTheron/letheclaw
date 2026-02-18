# letheClaw + OpenClaw Integration Guide

This guide shows how to add letheClaw to your **existing** OpenClaw docker-compose.yml.

---

## Step 1: Add Services to Your Compose File

**Location:** Your OpenClaw project directory — e.g. `docker-compose.yml` in the repo root.

**Action:** Append the contents of `docker-compose-addon.yml` to your existing compose file.

### Your Current Structure:
```yaml
services:
  openclaw:
    ...
  browser:
    ...
```

### After Adding letheClaw:
```yaml
services:
  openclaw:
    ...
  browser:
    ...
  
  # Copy everything from docker-compose-addon.yml here
  letheclaw-postgres:
    ...
  letheclaw-qdrant:
    ...
  letheclaw-redis:
    ...
  letheclaw-embeddings:
    ...
  letheclaw-api:
    ...

volumes:
  # Your existing volumes
  ...
  # letheClaw volumes
  letheclaw-postgres-data:
    ...
  letheclaw-qdrant-data:
    ...
  letheclaw-redis-data:
    ...
```

---

## Step 2: Build and Start

```bash
cd /path/to/your-openclaw-project

# Build new services
docker compose build letheclaw-api letheclaw-embeddings

# Start everything (existing + new services)
docker compose up -d

# Check status
docker compose ps
```

Expected output:
```
NAME                    STATUS
openclaw                Up
browser                 Up
letheclaw-postgres      Up (healthy)
letheclaw-qdrant        Up (healthy)
letheclaw-redis         Up (healthy)
letheclaw-embeddings    Up (healthy)
letheclaw-api           Up (healthy)
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

1. **Add to compose file** - Copy services from `docker-compose-addon.yml`
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
│       └── docker-compose-addon.yml  (copy contents to main compose)
```

---

**Status:** Ready to integrate. Follow Step 1-3 above to add letheClaw to your OpenClaw stack.
