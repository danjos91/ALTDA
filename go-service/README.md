# ALTDA Go service

The engine behind the web service: it reads a methodologist's request, proposes a program skeleton, allocates hours, writes the content and validates the result. Java calls it over HTTP; the contract is [`../docs/API.md`](../docs/API.md). It listens on **port 8081**.

Standard library only (`net/http`, `log/slog`): no external dependencies.

## Run

```bash
make run          # loads .env, runs on :8081
make test         # unit tests
go test -race ./...
```

Or with Docker, together with the Java service: `cd ../deploy && docker compose up --build`.

Settings are environment variables, read from `.env` by `make run` and by Docker Compose:

| Variable | Default | |
| --- | --- | --- |
| `HOST` | `0.0.0.0` | Bind address |
| `HTTP_PORT` | `8081` | Port |
| `ENV` | `local` | Environment name |
| `LOG_LEVEL` | `debug` | `debug`, `info`, `warn`, `error` |

The container image has no shell, so Docker checks it with the binary itself: `/app healthcheck` (calls `GET /v1/health` and exits 0 or 1).

## Routes

All seven routes of the contract are implemented. Bodies carry `schemaVersion: "1.1"`; errors are `{error, message}`.

| Route | What it does |
| --- | --- |
| `GET /v1/health` | Liveness |
| `POST /v1/parse` | Free text (plus answers) → parameters, missing parameters, questions, assumptions |
| `POST /v1/skeleton/generate` | Up to three skeleton variants, hours already allocated |
| `POST /v1/skeleton/regenerate` | Fresh variants before the plan exists; rebuilt modules (with `changedPaths`) afterwards. `409 node_locked` for locked nodes |
| `POST /v1/content` | Activities, lecture plans, tests, seminar tasks, calendar, template sections; returns the program and `issues`. Takes `nodeIds` to regenerate only some nodes |
| `POST /v1/validate` | Validation report and issues |
| `POST /v1/allocate` | Recomputes hours after manual edits; locked nodes keep theirs |

## Layout

```
cmd/app                 entry point (also: `app healthcheck`)
internal/
  app                   wiring, config, health check
  contract              constants of the Java↔Go contract
  program               the program.json model (see below)
  skeleton              types of the skeleton step (drafts)
  server/http           server, routes' handlers, errors, responses
  service/
    allocator           hours: largest remainder in half-hours (pure, no model)
    allocate            /v1/allocate: reads a program, runs the allocator
    validate            all validation rules
    parse               request parsing
    skeleton            skeleton variants and regeneration
    content             content of the topics, calendar, template sections
```

Each route has a handler (`server/http/handler/<name>`) and a service (`service/<name>`). Handlers decode and validate the request and turn service errors into contract errors; services hold the logic.

### Errors

Services return plain Go errors; each handler maps them to a contract code (`bad_request` 400, `node_locked` 409, `schema_invalid` 422, `llm_error` 502, `llm_timeout` 504). Anything unmapped is logged and answered as a generic 500, without leaking its cause.

### The program model

`program.Node[T]` wraps a JSON object: `V` is a typed view of the few fields Go reads or writes, and every other field passes through untouched. Encoding writes only what changed since decoding, so Go never loses or invents fields that other services own, and the schema can keep growing. A node built from scratch writes all of its typed fields.

## Replacing the stand-ins with an LLM

Three parts are rule-based or template stand-ins. Each sits behind an interface, so an LLM-backed implementation replaces it without touching the rest:

| Stand-in | Interface | Where |
| --- | --- | --- |
| Request parsing by rules | `parse.Service` | `service/parse` |
| Template skeleton | `skeleton.Drafter` | `service/skeleton/template.go` |
| Template module rebuild | `skeleton.Regenerator` | `service/skeleton/template_regen.go` |
| Template content | `content.Generator` | `service/content/template.go` |

The services around them are real and stay: weights are checked, hours come from the allocator (the model never does arithmetic), IDs are assigned by the service, locked nodes are skipped, topics are generated in parallel with a bound (`content.Config.MaxParallel`), and the validator checks the result.

A generator that returns unusable output is handled: an invalid skeleton draft is dropped, and if none is left the route answers `llm_error`. Generator failures and timeouts map to `llm_error` and `llm_timeout`.

Template text starts with `[Шаблон]` so it is easy to spot in a program or a document.

## Hours

Everything is computed in half-hours as integers, so sums are exact. The proportions (final attestation 5 % of the total within 1–8 h, 60 % practice, self-study by study form…) are in `allocator.DefaultConfig()`. They are provisional: calibrate them against the АЦР examples.

## Tests

`go test ./...` runs everything; the property tests check that for random programs of 16 to 500 hours the allocation is exact and every identity of the schema holds, and that what `allocate` and `content` produce passes the validator. Some tests read `../schema/example_program.json`: if it is missing they fail rather than skip.

The validator is a port of `../tools/validate_reference.py`; on the schema's example it gives the same result (one `SOURCE_UNVERIFIED` warning).
