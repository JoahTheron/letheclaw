# letheClaw on Windows

## Quick Setup

### Prerequisites

1. **Docker Desktop for Windows** (with WSL 2 backend)
2. **Git Bash** or **PowerShell** (for running commands)
3. **(Optional) WSL 2** for better performance

---

## Installation

### Step 1: Ensure Docker Desktop is Running

- Open Docker Desktop
- Enable WSL 2 integration (Settings → Resources → WSL Integration)
- Ensure "Use the WSL 2 based engine" is checked

### Step 2: Navigate to Project

```powershell
cd C:\path\to\letheclaw
```

Or in Git Bash:
```bash
cd /c/path/to/letheclaw
```
Replace with your actual clone path (e.g. `C:\GithubProjects\letheclaw`).

### Step 3: Start Services

```powershell
docker compose up -d
```

**Note:** On Windows, use `docker compose` (space), not `docker-compose` (hyphen).

### Step 4: Wait for Services

First run downloads the embedding model (~80MB). This takes 1-2 minutes.

```powershell
# Check logs
docker compose logs -f embeddings

# Wait for: "Model loaded successfully. Embedding dimension: 384"
```

### Step 5: Test

```powershell
# Check API health (only exposed port)
curl http://localhost:51234/health
```

---

## Known Windows Issues

### Issue: Line endings (CRLF vs LF)

**Cause:** Git on Windows converts line endings to CRLF, which breaks shell scripts.

**Fix:** Configure Git to use LF:
```powershell
git config --global core.autocrlf input
```

Then re-clone the repository.

### Issue: Slow build/startup

**Cause:** Windows file system is slower than Linux for Docker volumes.

**Fix:** Use WSL 2 backend or move project to WSL 2 filesystem:
```bash
# In WSL 2
cd ~
git clone <your-repo>
cd letheclaw
docker compose up -d
```

### Issue: `make` command not found

**Cause:** Make is not installed on Windows by default.

**Fix:** Use PowerShell alternatives:
- `make start` → `docker compose up -d`
- `make stop` → `docker compose down`
- `make logs` → `docker compose logs -f`
- `make test` → Manual curl commands (see below)

---

## Example requests (PowerShell)

### 1. Health Check
```powershell
curl http://localhost:51234/health | ConvertFrom-Json
```

### 2. Store Memory
```powershell
$body = @{
    content = "Quantum computing threatens RSA signatures"
    source = "operator_input"
    tags = @("security", "quantum")
    operator = "Markus"
    session_key = "main:test123"
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "http://localhost:51234/memory" -Body $body -ContentType "application/json"
```

### 3. Search
```powershell
curl "http://localhost:51234/memory/search?q=quantum%20threat&limit=5"
```

### 4. Recent Memories
```powershell
curl http://localhost:51234/memory/recent
```

---

## Stopping Services

```powershell
docker compose down
```

To remove all data:
```powershell
docker compose down -v
```

---

## Database Access (PowerShell)

```powershell
# View tables
docker compose exec postgres psql -U letheclaw -d letheclaw -c "\dt"

# Interactive shell
docker compose exec postgres psql -U letheclaw -d letheclaw
```

---

## Performance Tips

1. **Use WSL 2:** Much faster than native Windows filesystem
2. **Allocate more memory:** Docker Desktop → Settings → Resources → Memory (4GB minimum)
3. **Disable antivirus scanning:** Exclude Docker volumes from real-time scanning

---

## Common Commands (PowerShell vs Bash)

| Task | Bash (Git Bash / WSL) | PowerShell |
|------|----------------------|------------|
| Start | `make start` or `docker compose up -d` | `docker compose up -d` |
| Stop | `make stop` or `docker compose down` | `docker compose down` |
| Logs | `make logs` or `docker compose logs -f` | `docker compose logs -f` |
| Health | `curl http://localhost:51234/health` | `curl http://localhost:51234/health` |
| JSON pretty | `curl ... \| jq .` | `curl ... \| ConvertFrom-Json` |

---

## Troubleshooting

### Ports Already in Use

```powershell
# Check what's using a port
netstat -ano | findstr :51234

# Kill process by PID
taskkill /PID <PID> /F
```

### Docker Not Starting

1. Restart Docker Desktop
2. Check WSL 2 status: `wsl --status`
3. Update WSL 2: `wsl --update`

### Can't Connect to Services

```powershell
# Check container status
docker compose ps

# Check container logs
docker compose logs <service-name>

# Restart specific service
docker compose restart <service-name>
```

---

## Recommended Setup (Best Performance)

1. **Install WSL 2:**
   ```powershell
   wsl --install
   ```

2. **Clone project in WSL 2:**
   ```bash
   # In WSL terminal
   cd ~
   git clone <your-repo-url> letheclaw
   cd letheclaw
   ```

3. **Run from WSL 2:**
   ```bash
   docker compose up -d
   curl http://localhost:51234/health
   ```

This gives you native Linux performance.

---

**Status:** letheClaw works on Windows with Docker Desktop + WSL 2 backend.  
**Issues?** Check Docker Desktop logs and ensure WSL 2 integration is enabled.
