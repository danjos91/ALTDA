# Deploy

Runs both services with Docker Compose:

| Service | What | Port |
| --- | --- | --- |
| `go` | Engine: parse, skeleton, content, validate, allocate (contract in [`docs/API.md`](../docs/API.md)) | 8081 |
| `java` | Web UI and orchestration (Spring Boot) | 8000 |

Java starts only when Go reports healthy. Go's image has no shell, so its binary checks itself (`/app healthcheck`).

## Run

```bash
cd deploy
docker compose up --build                       # Java uses our Go service
ALTDA_GO_CLIENT=mock docker compose up --build  # Java uses its own built-in stand-in
docker compose down
```

The UI is at <http://localhost:8000/>. The first build is slow: Java compiles with Maven inside the image.

Settings can go in `deploy/.env` (see `.env.example`). Go reads its own from `go-service/.env`.

## Check it

```bash
curl -s localhost:8000/health        # Java
curl -s localhost:8081/v1/health     # Go

curl -s -X POST localhost:8081/v1/parse -H 'Content-Type: application/json' \
  -d '{"schemaVersion":"1.1","rawText":"ДПП ПК Анализ данных на Python, 72 ч"}'

../scripts/smoke_test.sh http://localhost:8000
```

## Notes

- Java keeps programs in memory: restarting the container loses them.
- After a code change, rebuild only what changed: `docker compose up -d --build go` (or `java`).
- Logs: `docker compose logs -f go` (or `java`).
- To run Go on your machine instead of in Docker, start it (`make run` in `go-service/`) and use
  `GO_BASE_URL=http://host.docker.internal:8081`.
