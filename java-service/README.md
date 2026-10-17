# ALTDA Java service

Spring Boot 21 orchestration API (заявка lifecycle, Go client). Listens on **port 8000**.

## Run

```bash
./run.sh
# equivalent:
./mvnw spring-boot:run
```

Docker:

```bash
docker build -t altda-java .
docker run --rm -p 8000:8000 altda-java
```

Default profile uses **mock** Go client (`altda.go.client=mock`). To call a real Go service:

```bash
ALTDA_GO_CLIENT=http ./run.sh
# or
./mvnw spring-boot:run -Dspring-boot.run.profiles=http
```

## Quick test

```bash
curl -s http://localhost:8000/health
curl -s -X POST http://localhost:8000/api/generate \
  -H 'Content-Type: application/json' \
  -d '{"request":"ДПП ПК Анализ данных на Python, 72 ч","answers":{"entryLevel":"Базовый"}}'
```

From repo root: `../scripts/smoke_test.sh`.

Open the UI: [http://localhost:8000/](http://localhost:8000/)

### Manual UI checklist (Playwright-free)

1. Start with `./run.sh`, open `/` — sidebar + five tabs, Russian labels.
2. **Заявка:** «+ Новая заявка» → send free-text → status badge moves; answer clarifications; see «идёт генерация…».
3. **Каркас:** three variants with scores; pick one; hours badge; approve → content starts; or return with comment.
4. **Содержание:** modules/topics list; point «Перегенерировать»; approve/return section.
5. **Проверка и отчёт:** issues by severity; sources verification; tokens/time table.
6. **Выгрузка:** download JSON; **Скачать .docx** hits `GET /programs/{id}/export.docx`.
7. History panel updates after approve/return; toast appears when a long step finishes (poll 2 s).
8. While busy, a mutating action yields a graceful **409** toast (not a blank page).
9. Force/observe **failed**: error banner + `error` / `failedFrom`; UI remains usable.
10. Approve/return accept `If-Match` or body `version`; stale version → **409** `version_conflict`.

## Tests

```bash
./mvnw test
```

## Layout

- Static SPA: `src/main/resources/static/` (`index.html`, `css/app.css`, `js/app.js`)
- `POST /programs` — interactive заявка (async; poll `GET /programs/{id}`)
- `POST /api/generate` — sync pipeline with auto-approval (hackathon tests)
- `GET /programs/{id}/export.docx` — DOCX export (Apache POI + styled template)
- Contract with Go: [`../docs/API.md`](../docs/API.md)
