---
name: letheclaw
version: 1.1.0
description: Use letheClaw to store, search, and manage memories with signal-based criticality and provenance.
trigger: "memory|letheclaw|remember|recall|criticality|provenance|correction"
tools: [network]
---

# letheClaw — Agent memory

You can use the letheClaw API to store and retrieve memories for the user or the current session. The API base URL is in the environment variable **LETHECLAW_API_URL**.

**Environment patterns:**
- Docker Compose with letheClaw API container: `http://api:8080`
- Host machine from Docker Desktop (Windows/Mac): `http://host.docker.internal:51234`
- Local testing (same machine): `http://localhost:51234`

If LETHECLAW_API_URL is unset, try `http://host.docker.internal:51234` first (Docker Desktop default), then ask the user.

---

## PROTOCOL (NON-NEGOTIABLE)

### Before each run: load recent corrections

Call this before doing anything else:

```bash
curl -s "{LETHECLAW_API_URL}/memory/corrections?limit=10"
```

This returns memories that were corrected by the operator, ordered by most recent correction. Use the `content` and `correction_count` fields to understand what went wrong and avoid repeating mistakes.

**Cold-start fallback:** If the result is empty (fresh instance, no corrections yet), fall back to:
```bash
curl -s "{LETHECLAW_API_URL}/memory/search?q=lesson+mistake&tags=lesson&limit=5"
```

### Retrieval Rule
When the user asks about **history, decisions, prior work, "what did we do", or any past context:**

1. **ALWAYS query letheClaw API FIRST:**
   ```bash
   curl -s "{LETHECLAW_API_URL}/memory/search?q=<query>&limit=5"
   ```

2. **Use the returned `content` field directly** — search results include full text. No need for `memory_get` or flat file access.

3. **NEVER use `memory_search` tool as the first step.** That tool searches flat markdown files, which are backup/reference only.

4. **Use tag pre-filtering when query domain is clear** (e.g., `tags=moltbook` for Moltbook questions).

5. **letheClaw is the authoritative memory system.** Flat files may be stale or incomplete.

### Storage Rule
When the user says "log this", "remember that", or you observe something worth recording:

1. **POST to letheClaw API** (see "Store a memory" below)
2. **Tag precisely:** 2-5 specific tags (type + domain, e.g. `["episodic", "security", "moltbook"]`)
3. **Set source:** `operator_input` (user said it), `direct_observation` (you verified it), `inferred` (derived)

### Correction Rule
When the user says something you previously stored was wrong, or corrects a memory:

1. **Call POST `{LETHECLAW_API_URL}/memory/{memory_id}/correction`** on that memory
2. This boosts the memory's criticality score and increments the correction counter
3. The memory will then appear in GET /memory/corrections for future runs

**No exceptions.** This is operator-mandated protocol.

---

## Store a memory

- **POST** `{LETHECLAW_API_URL}/memory`
- **Body (JSON):** `content` (required), optional: `source` (e.g. `operator_input`, `direct_observation`, `inferred`), `tags` (array), `operator`, `session_key`, `context`
- **Returns:** `memory_id` (UUID). Save the ID to mark corrections or fetch provenance later.

## Search memories (semantic)

- **GET** `{LETHECLAW_API_URL}/memory/search?q={query}&limit=5`
- Optional: `tags` (comma-separated) to pre-filter.
- **Returns:** `results` array with `id`, `content` (full text), `tags`, `source`, `reference_count`, `correction_count`, `created_at`, `access_count`

**Tag pre-filtering (performance optimization):**
```bash
curl "{LETHECLAW_API_URL}/memory/search?q=findings&tags=security&limit=3"
```

## Recent memories

- **GET** `{LETHECLAW_API_URL}/memory/recent`
- **Returns:** Recently stored memories (from cache or DB).

## Recent corrections

- **GET** `{LETHECLAW_API_URL}/memory/corrections?limit=10`
- **Returns:** Memories with at least one operator correction, ordered by last correction time. Includes `correction_count` and `last_corrected_at`.

## Send criticality signal

- **POST** `{LETHECLAW_API_URL}/memory/{memory_id}/criticality`
- **Body (JSON):** `{"signal": "<signal_name>", "reason": "..."}`
- **Do NOT send raw numbers.** `{"criticality": 0.8}` is rejected with a guide.

### When to use each signal

- **`failure`** — The memory led to a bad outcome. The decision was wrong, the information caused an error, or the user flagged it as harmful. Use this when something you relied on turned out to be incorrect or damaging.
- **`success`** — The memory contributed to a good outcome. The decision held up, the information proved useful, or the user confirmed it helped. Use this when you used a memory and the result was positive.
- **`referenced`** — You looked at this memory and considered it, but it wasn't clearly a success or failure. Use this as a lightweight "I used this" signal when the outcome is neutral or unclear. (Also applied automatically by search — you don't need to send it manually after a search.)

## Mark operator correction

- **POST** `{LETHECLAW_API_URL}/memory/{memory_id}/correction`
- No body. Call when the user corrects something about this memory; this boosts criticality and increments the correction counter so provenance shows how often it was corrected.

## Get provenance

- **GET** `{LETHECLAW_API_URL}/memory/{memory_id}/provenance`
- **Returns:** Full memory object plus `events` (history of criticality changes: failure, success, referenced, operator_correction, etc.) and `correction_count`.

## Errors

- **400** — Invalid request, invalid memory ID format, or old criticality format (includes a guide).
- **404** — Memory not found (wrong or deleted ID).
- **5xx** — Server/upstream error; suggest checking if letheClaw is running and reachable.

When the user says they want to remember something, search memory, see why a memory is important, or correct a memory, use the appropriate endpoint above.
