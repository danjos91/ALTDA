# Java ↔ Go HTTP contract (final MVP)

Payloads follow `schema/program.schema.json` (**schemaVersion `1.1`**). JSON fields are **camelCase**. Every request and response body includes `"schemaVersion": "1.1"` (except bare health).

**All Go calls are synchronous.** Java owns async orchestration and polling for the UI; it waits on each Go HTTP call within the per-call read timeout.

Base URL: `altda.go.base-url`. Client: `altda.go.client=mock|http` (default `mock`).

---

## Status names

| `program.status` (schema) | Notes |
|---------------------------|--------|
| `intake` | |
| `clarifying` | |
| `skeleton_proposed` | |
| `skeleton_approved` | |
| `content_generating` | |
| `content_ready` | |
| `validated` | |
| `exported` | |
| ~~`failed`~~ | **Java orchestration only** — never written into `program.status`; appears on Java `GET /programs/{id}` with `error` |

---

## Errors (Go → Java)

HTTP 4xx/5xx body:

```json
{ "error": "bad_request", "message": "human-readable" }
```

Fixed `error` codes:

| Code | Typical HTTP | Meaning |
|------|--------------|---------|
| `bad_request` | 400 | Malformed / incomplete payload |
| `node_locked` | 409 | Regenerate targeted a locked node while `respectLocks=true` |
| `llm_timeout` | **504** | Model call timed out (Java maps this code to HTTP 504 only — never 408) |
| `llm_error` | 502 | Model / upstream failure |
| `schema_invalid` | 422 | Response or input failed JSON Schema |

---

## Hour formulas (every level)

Every `hours` object (topic, module, final attestation) and curriculum `totals` use the same identities. Go’s allocator and `HOURS_*` validation rules (`HOURS_EQ_REQUESTED`, `HOURS_ROW_SUM`, …) enforce them; **Java never recalculates**:

```
contact = lecture + practice + lab
total   = contact + selfStudy + assessment
```

Values are in academic hours with **0.5** step. Module hours equal the sum of their topics (plus module-level assessment if any); program `totals` equal the sum of modules plus final-attestation hours.

---

## Per-call read timeouts (Java)

`altda.go.timeouts` in `java-service/.../application.yml`:

| Call | Default |
|------|---------|
| `POST /v1/skeleton/generate`, `POST /v1/skeleton/regenerate` | 180 000 ms (~3 min) |
| `POST /v1/content` | 600 000 ms (~10 min) |
| `POST /v1/validate`, `POST /v1/allocate` | 10 000 ms |
| `POST /v1/parse` | 60 000 ms |
| `GET /v1/health` | connect timeout only |

---

## 0. Health

```
GET /v1/health
```

**Response 200**

```json
{ "status": "ok" }
```

---

## 1. Parse / clarify

```
POST /v1/parse
```

**Request**

```json
{
  "schemaVersion": "1.1",
  "rawText": "string",
  "attachments": [],
  "clarifications": [
    { "param": "entryLevel", "question": "...", "answer": "..." }
  ]
}
```

**Response 200**

```json
{
  "schemaVersion": "1.1",
  "params": { },
  "missing": ["entryLevel"],
  "clarifications": [
    { "param": "entryLevel", "question": "...", "answer": null }
  ],
  "gapsReport": []
}
```

---

## 2. Generate skeleton (hours already allocated)

```
POST /v1/skeleton/generate
```

**Request**

```json
{
  "schemaVersion": "1.1",
  "program": { },
  "selectedVariantKey": null
}
```

**Response 200**

```json
{
  "schemaVersion": "1.1",
  "variants": [
    {
      "key": "A",
      "rationale": "...",
      "pros": ["..."],
      "cons": ["..."],
      "goal": "...",
      "competencies": [],
      "modules": [
        {
          "id": "M1",
          "title": "...",
          "isIntro": false,
          "weight": 0.5,
          "hours": { "lecture": 2, "practice": 4, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 6, "total": 6 },
          "topics": [
            {
              "id": "T1.1",
              "title": "...",
              "weight": 1,
              "kinds": ["lecture", "practice"],
              "outcomeIds": ["C1.K1"],
              "hours": { "lecture": 2, "practice": 4, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 6, "total": 6 }
            }
          ]
        }
      ],
      "finalAttestation": {
        "form": "защита проекта",
        "description": "...",
        "outcomeIds": [],
        "hours": { "lecture": 0, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 2, "contact": 0, "total": 2 }
      },
      "totals": { "lecture": 2, "practice": 4, "lab": 0, "selfStudy": 0, "assessment": 2, "contact": 6, "total": 8 }
    }
  ]
}
```

Up to **3** variants. Go’s allocator fills all `hours.*` and `totals` using the [hour formulas](#hour-formulas-every-level) (0.5-step). **Java never computes hours.** On approve, Java **merges the chosen variant locally** into the working program (no `/materialize` endpoint).

Transport may carry allocated `hours` / `totals` on variants beyond the weight-only `$defs/skeletonDraft` used for LLM output validation inside Go.

---

## 3. Regenerate (locked / dependsOn)

```
POST /v1/skeleton/regenerate
```

**Request**

```json
{
  "schemaVersion": "1.1",
  "program": { },
  "comment": "Упрости модуль 2",
  "nodeIds": ["M2", "T2.1"],
  "respectLocks": true
}
```

- Empty/null `nodeIds`: Go infers the subgraph from `comment` (or regenerates broadly).
- With `respectLocks: true`, nodes with `_meta.locked: true` are not overwritten.
- If any requested `nodeId` is locked and `respectLocks=true` → **409** `{ "error": "node_locked", "message": "..." }`.

**Response 200**

```json
{
  "schemaVersion": "1.1",
  "program": { },
  "changedPaths": ["/curriculum/modules/1"],
  "variants": null
}
```

Exactly one of `program` / `variants` is non-null. Regenerated fragments already have hours from Go.

---

## 4. Allocate (rebalance only)

```
POST /v1/allocate
```

Used **only after manual hour edits** in the UI (rebalance remaining weights/rows to match `header.totalHours` using the [hour formulas](#hour-formulas-every-level)). Not used on the happy path after skeleton generate.

**Request**

```json
{
  "schemaVersion": "1.1",
  "program": { }
}
```

**Response 200**

```json
{
  "schemaVersion": "1.1",
  "program": { }
}
```

---

## 5. Generate content

```
POST /v1/content
```

**Request**

```json
{
  "schemaVersion": "1.1",
  "program": { },
  "nodeIds": null
}
```

- `nodeIds: null` (or omitted): generate / fill content for the whole program.
- **Non-null `nodeIds`:** regenerate **only those nodes plus their dependents**, **skipping locked** nodes (`_meta.locked: true`) — same targeting semantics as [`/v1/skeleton/regenerate`](#3-regenerate-locked--dependson) (without requiring `respectLocks` / `node_locked` 409 on this endpoint: locked targets are simply skipped).

**Response 200**

```json
{
  "schemaVersion": "1.1",
  "program": { },
  "issues": [
    {
      "ruleId": "SOURCE_UNVERIFIED",
      "severity": "warning",
      "message": "...",
      "path": "/sources/0"
    }
  ]
}
```

`issues` is **always an array, never `null`** (empty `[]` when clean). `issues[].path` is a **JSON Pointer**. `issues[].ruleId` values are identical to rule names in `tools/validate_reference.py` (e.g. `HOURS_EQ_REQUESTED`, `HOURS_ROW_SUM`, `COVERAGE_OUTCOME_TAUGHT`, `SOURCE_UNVERIFIED`, `SECTION_MISSING`, …).

---

## 6. Validate

```
POST /v1/validate
```

**Request**

```json
{
  "schemaVersion": "1.1",
  "program": { }
}
```

**Response 200**

```json
{
  "schemaVersion": "1.1",
  "validationReport": { },
  "issues": [
    {
      "ruleId": "HOURS_EQ_REQUESTED",
      "severity": "error",
      "message": "...",
      "path": "/header/totalHours"
    }
  ]
}
```

`validationReport` matches the schema. `issues` is **always an array** (never `null`) = failed checks (`passed: false`), same `ruleId` / JSON Pointer conventions as above. `HOURS_*` rules use the [hour formulas](#hour-formulas-every-level).

---

## Java public REST (orchestration)

Port **8000**. UI polls Java; Go is never called from the browser.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/health` | Java liveness (+ which Go client) |
| `POST` | `/api/generate` | Sync test pipeline, auto-approve |
| `POST` | `/programs` … | Interactive заявка lifecycle (see java-service README) |
| `GET` | `/programs/{id}/export.docx` | Phase 3 DOCX export (Apache POI from styled template) |

Mutating approve/return/lock/allocate/regenerate endpoints accept optimistic concurrency via `If-Match` (etag or bare version) and/or body `version`; mismatch → HTTP **409** `version_conflict`. `GET /programs/{id}` returns a deep-copied snapshot (`failedFrom` when status is Java-only `failed`).

Java `HttpGoClient` maps Go `{ "error", "message" }` to typed `GoClientException`:

| Go `error` | UI HTTP | Program status |
|------------|---------|----------------|
| `node_locked` | 409 | Unchanged (never `FAILED`) |
| `bad_request` | 400 | Unchanged on sync calls; `FAILED` only if the call was inside an async pipeline step |
| `schema_invalid` | 422 | Same as `bad_request` |
| `llm_timeout` (incl. connection/read timeouts mapped to this code) | **504** | Java-only `FAILED` + `error` + `failedFrom` |
| `llm_error` | 502 | Java-only `FAILED` + `error` + `failedFrom` |

---

## Locked decisions (no longer open)

1. **Hours:** Go allocates with `contact = lecture+practice+lab`, `total = contact+selfStudy+assessment` at every level; Java never recalculates. Skeleton generate returns allocated variants; Java merges locally; **no `/materialize`**.
2. **`/v1/allocate`:** rebalance after manual edits only (same formulas).
3. **`/v1/content`:** `{ program, issues[] }` (`issues` always an array). Non-null `nodeIds` = those nodes + dependents, skipping locked (same as regenerate targeting).
4. **`llm_timeout` → HTTP 504 only** (not 408).
5. **`failed`:** Java-only orchestration status.
6. **Sync Go calls** + per-call timeouts as above.
7. **SSE:** not in this phase (Java polling).
