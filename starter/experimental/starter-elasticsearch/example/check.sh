#!/usr/bin/env bash
#
# Smoke test for starter-elasticsearch. Brings up a local Elasticsearch via
# docker compose, runs the example (which self-asserts and exits non-zero on
# failure), then tears the container down. Skipped gracefully when docker is
# unavailable.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

# Prefer the compose v2 plugin, fall back to the standalone docker-compose.
if docker compose version >/dev/null 2>&1; then
    compose() { docker compose "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose "$@"; }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d

# Wait for Elasticsearch to answer on its HTTP port (up to 120s; ES is slow to boot).
for _ in $(seq 1 120); do
    if curl -fsS http://127.0.0.1:9200 >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

go run . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
