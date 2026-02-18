---
name: letheclaw
version: 1.0.0
description: Use letheClaw to store, search, and manage memories with criticality and provenance.
trigger: "memory|letheclaw|remember|recall|criticality|provenance"
tools: [network]
---

# letheClaw — Agent memory

You can use the letheClaw API to store and retrieve memories for the user or the current session. The API base URL is in the environment variable **LETHECLAW_API_URL** (e.g. `http://letheclaw-api:8080` when running in Docker with letheClaw, or `http://localhost:51234` for local testing). If unset, ask the user for the letheClaw API URL or assume a default they provide.

## Store a memory

- **POST** `{LETHECLAW_API_URL}/memory`
- **Body (JSON):** `content` (required), optional: `source` (e.g. `operator_input`, `direct_observation`, `inferred`), `tags` (array), `operator`, `session_key`, `context`
- **Returns:** `memory_id` (UUID). Save it to update criticality or fetch provenance later.

## Search memories (semantic)

- **GET** `{LETHECLAW_API_URL}/memory/search?q={query}&limit=5`
- Optional: `min_criticality` (0–1) to filter by importance.
- **Returns:** `results` array with `id`, `content`, `criticality`, `tags`, `source`, etc.

## Recent memories

- **GET** `{LETHECLAW_API_URL}/memory/recent`
- **Returns:** Recently stored memories (from cache or DB).

## Update criticality (manual)

- **POST** `{LETHECLAW_API_URL}/memory/{memory_id}/criticality`
- **Body (JSON):** `criticality` (0–1, required), optional `reason`
- Use when the user or you want to mark a memory as more or less important.

## Mark operator correction

- **POST** `{LETHECLAW_API_URL}/memory/{memory_id}/correction`
- No body. Call when the user corrects something about this memory; this boosts criticality and increments a correction counter so provenance shows how often it was corrected.

## Get provenance

- **GET** `{LETHECLAW_API_URL}/memory/{memory_id}/provenance`
- **Returns:** Full memory object plus `events` (history of criticality changes: manual_boost, operator_correction, etc.) and `correction_count`.

## Errors

- **400** — Invalid request or invalid memory ID format.
- **404** — Memory not found (wrong or deleted ID).
- **5xx** — Server/upstream error; suggest checking if letheClaw is running and reachable.

When the user says they want to remember something, search memory, see why a memory is important, or correct a memory, use the appropriate endpoint above.
