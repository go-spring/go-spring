#!/usr/bin/env bash
# Load test for starter-mongodb. Spins up MongoDB via docker-compose, runs the
# example-load binary against it, and lets it self-exit after the load window.
# Toggle fault.* in conf/app.properties to inject failures.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"; exit 0
fi
if docker compose version >/dev/null 2>&1; then
    compose() { docker compose "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose "$@"; }
else
    echo "WARNING: docker compose not available — skipping"; exit 0
fi
trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d
echo "== waiting for the service to be ready =="
for _ in $(seq 1 30); do
    if (exec 3<>/dev/tcp/127.0.0.1/27017) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true; break
    fi
    sleep 1
done
echo "== example boot =="
go run . &
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
