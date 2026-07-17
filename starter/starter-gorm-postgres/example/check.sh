#!/usr/bin/env bash
#
# Smoke test for starter-gorm-postgres. Brings up a local PostgreSQL via docker compose,
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

# Wait for PostgreSQL to report healthy (up to ~90s; first-run init is slow, and the
# port opens before the server can actually serve queries).
for _ in $(seq 1 45); do
    status="$(docker inspect -f '{{.State.Health.Status}}' starter-gorm-postgres 2>/dev/null || true)"
    [ "${status}" = "healthy" ] && break
    sleep 2
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
