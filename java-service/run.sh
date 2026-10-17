#!/usr/bin/env bash
# One-command start of the ALTDA Java service on port 8000 (mock Go client by default).
set -euo pipefail
cd "$(dirname "$0")"

PORT="${PORT:-8000}"
export SERVER_PORT="$PORT"

if [[ -x ./mvnw ]]; then
  MVN=./mvnw
elif command -v mvn >/dev/null 2>&1; then
  MVN=mvn
else
  echo "Maven not found. Install Maven or generate the wrapper with: mvn -N wrapper:wrapper" >&2
  exit 1
fi

GO_CLIENT="${ALTDA_GO_CLIENT:-mock}"
GO_BASE="${ALTDA_GO_BASE_URL:-http://localhost:8081}"
echo "Starting ALTDA java-service on :$PORT (altda.go.client=${GO_CLIENT} base-url=${GO_BASE})"
exec $MVN -q spring-boot:run \
  -Dspring-boot.run.jvmArguments="-Dserver.port=${PORT}" \
  -Dspring-boot.run.arguments="--altda.go.client=${GO_CLIENT} --altda.go.base-url=${GO_BASE}"
