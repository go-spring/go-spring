#!/usr/bin/env bash
#
# Smoke test for starter-neo4j. Brings up a local Neo4j via docker compose,
# runs the example (which self-asserts and exits non-zero on failure), then
# tears the container down. Skipped gracefully when docker is unavailable.
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

# Wait for Neo4j to accept Bolt connections (up to 120s).
for _ in $(seq 1 120); do
    if (exec 3<>/dev/tcp/127.0.0.1/7687) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        break
    fi
    sleep 1
done

# The Bolt port opens before the database is ready to serve queries, so give
# Neo4j a short grace period to finish initializing the default database.
sleep 10

go run . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
