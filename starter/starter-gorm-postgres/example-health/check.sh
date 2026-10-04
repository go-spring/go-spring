#!/usr/bin/env bash
#
# Smoke test for starter-gorm-postgres health. Brings up a local PostgreSQL via
# docker compose, runs the example (which pings the DB and asserts the actuator
# probes report UP), then tears the container down. Skipped gracefully when
# docker is unavailable.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

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

# Wait for PostgreSQL to report healthy (up to ~90s; first-run init is slow).
echo "== waiting for the service to be ready =="
for _ in $(seq 1 45); do
    status="$(docker inspect -f '{{.State.Health.Status}}' starter-gorm-postgres-health 2>/dev/null || true)"
    [ "${status}" = "healthy" ] && break
    sleep 2
done

echo "== example boot =="
go run -gcflags="all=-N -l" . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
if [ "${rc}" -ne 0 ]; then
    echo "== FAILED ==" >&2
    exit "${rc}"
fi
echo "== OK =="
exit 0
