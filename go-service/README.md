# ALTDA Go service

HTTP skills engine called by `java-service` (`docs/API.md`). Allocates hours, talks to an OpenAI-compatible LLM (or stub), and validates programs (`tools/validate_reference.py` port).

## Endpoints

| Method | Path | Notes |
|--------|------|--------|
| `GET` | `/v1/health` | `{ "status": "ok" }` |
| `POST` | `/v1/parse` | Clarify / extract params |
| `POST` | `/v1/skeleton/generate` | ≤3 variants in parallel + deterministic hours |
| `POST` | `/v1/skeleton/regenerate` | Respects `_meta.locked` / `dependsOn`; `409 node_locked` |
| `POST` | `/v1/allocate` | Rebalance hours after manual edits (no LLM) |
| `POST` | `/v1/content` | Content + `issues[]` (always an array) |
| `POST` | `/v1/validate` | Full validator; `validationReport` + flat `issues` |

Paths match `HttpGoClient` in java-service (same as `docs/API.md`).

## Run (stub — no API key)

```bash
cd go-service
./run.sh
# listens on :8081; OPENAI_API_KEY empty ⇒ stub fixtures
curl -s localhost:8081/v1/health
```

Force stub: `ALTDA_STUB=true ./run.sh` or `./run.sh -stub`.

## Run (real LLM)

```bash
export OPENAI_BASE_URL=https://your-gateway/v1
export OPENAI_API_KEY=...
export OPENAI_MODEL=...
./run.sh
```

Invalid LLM JSON is retried once; request context cancels in-flight calls when Java disconnects.

## Docker

```bash
# from repo root
cp .env.example .env   # leave key empty for stub
docker compose up --build
# Java :8000 → Go http://go-service:8081 (ALTDA_GO_CLIENT=http)
./scripts/smoke_test.sh http://localhost:8000
```

- Go `WriteTimeout` = 12 minutes (covers `/v1/content` Java read timeout of 10 min).
- Healthcheck: `GET /v1/health` (image includes `wget`; binary also supports `-healthcheck`).

## Tests

```bash
cd go-service
go vet ./...
go test ./...
```

## Errors

`{"error":"<code>","message":"..."}` with codes: `bad_request` (400), `node_locked` (409), `schema_invalid` (422), `llm_error` (502), `llm_timeout` (504).
