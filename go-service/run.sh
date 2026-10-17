#!/usr/bin/env sh
set -eu
# Local: ./run.sh   |  Container ENTRYPOINT uses /app/altda-go
# Empty OPENAI_API_KEY (or ALTDA_STUB=true / -stub) → deterministic stub mode.
ROOT="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
ADDR="${PORT:-:8081}"
case "$ADDR" in
  :*) ;;
  *:*) ;;
  *) ADDR=":$ADDR" ;;
esac

BIN="${ALTDA_GO_BIN:-}"
if [ -z "$BIN" ]; then
  if [ -x "$ROOT/altda-go" ]; then
    BIN="$ROOT/altda-go"
  elif [ -x /app/altda-go ]; then
    BIN=/app/altda-go
  else
    BIN="go"
  fi
fi

export ALTDA_FIXTURES_DIR="${ALTDA_FIXTURES_DIR:-$ROOT/testdata}"

if [ "$BIN" = "go" ]; then
  cd "$ROOT"
  exec go run ./cmd/server -addr="$ADDR" "$@"
fi
exec "$BIN" -addr="$ADDR" "$@"
