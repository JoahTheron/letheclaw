# Manual testing guide

Use this after the stack is running (see [QUICKSTART.md](QUICKSTART.md)).

**Base URL:** `http://localhost:51234` (Docker) or your API host.
**Windows:** Use Git Bash or WSL for `curl`; or PowerShell with `Invoke-RestMethod` (examples use `curl`).

---

## Phase 1 (smoke test)

```bash
curl -s http://localhost:51234/health
curl -s -X POST http://localhost:51234/memory -H "Content-Type: application/json" -d "{\"content\":\"Test memory for Phase 2\",\"source\":\"operator_input\",\"tags\":[\"test\"]}"
# Save the returned memory_id (UUID) for the next steps.
curl -s "http://localhost:51234/memory/search?q=test&limit=5"
curl -s http://localhost:51234/memory/recent
```

---

## Phase 2: Signals, corrections, provenance

Use one memory ID through the whole flow. Replace `MEMORY_ID` with the UUID from the store response.

### 1. Store a memory

```bash
curl -s -X POST http://localhost:51234/memory \
  -H "Content-Type: application/json" \
  -d '{"content":"Important decision: use Go for the API","source":"operator_input","tags":["decision"]}'
```

Example response:
```json
{"memory_id":"a1b2c3d4-e5f6-7890-abcd-ef1234567890", "status":"success", ...}
```

### 2. Send a criticality signal (new contract)

```bash
curl -s -X POST "http://localhost:51234/memory/$MEMORY_ID/criticality" \
  -H "Content-Type: application/json" \
  -d '{"signal": "failure", "reason": "Decision led to a deployment issue"}'
```

Expected: `{"status":"success","memory_id":"...","signal":"failure","old_score":0,"new_score":0.3}`

### 3. Old format is rejected with a guide

```bash
curl -s -X POST "http://localhost:51234/memory/$MEMORY_ID/criticality" \
  -H "Content-Type: application/json" \
  -d '{"criticality": 0.85, "reason": "High-impact decision"}'
```

Expected: `{"error":"Raw criticality scores are no longer accepted.","guide":"Send {\"signal\": ...} instead..."}`

### 4. Mark operator correction

```bash
curl -s -X POST "http://localhost:51234/memory/$MEMORY_ID/correction"
```

Expected: `{"status":"success","memory_id":"...","old_score":0.3,"new_score":0.8}`

### 5. Get recent corrections

```bash
curl -s "http://localhost:51234/memory/corrections?limit=5"
```

Expected: Array of corrected memories with `correction_count`, `last_corrected_at`, ordered by most recent correction.

### 6. Get provenance

```bash
curl -s "http://localhost:51234/memory/$MEMORY_ID/provenance"
```

Expected: JSON with `memory` (full memory object including `correction_count`, `reference_count`) and `events` (array of criticality events: failure, operator_correction, etc.).

### 7. 404 behaviour

```bash
# Invalid UUID
curl -s -X POST "http://localhost:51234/memory/not-a-uuid/criticality" -H "Content-Type: application/json" -d '{"signal":"failure"}'
# Expected: 400 invalid memory id

# Valid UUID but non-existent memory
curl -s "http://localhost:51234/memory/00000000-0000-0000-0000-000000000000/provenance"
# Expected: 404 memory not found
```

---

## One-liner flow (bash)

```bash
ID=$(curl -s -X POST http://localhost:51234/memory -H "Content-Type: application/json" -d '{"content":"E2E test memory","source":"operator_input"}' | jq -r .memory_id)
curl -s -X POST "http://localhost:51234/memory/$ID/criticality" -H "Content-Type: application/json" -d '{"signal":"failure","reason":"test"}'
curl -s -X POST "http://localhost:51234/memory/$ID/correction"
curl -s "http://localhost:51234/memory/corrections" | jq .
curl -s "http://localhost:51234/memory/$ID/provenance" | jq .
```

Requires `jq`. If you don't have it, use the step-by-step commands above and copy the `memory_id` by hand.

---

## Troubleshooting

- **`Failed to generate embedding` / `connection refused` to `embeddings:5000`**
  The embedding sidecar may still be starting (e.g. first run loads the model). Wait 30–60s and retry the POST; `/memory/recent` and `/health` can still work while the sidecar is down.
