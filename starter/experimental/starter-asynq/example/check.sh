#!/usr/bin/env bash
#
# Smoke test for starter-asynq. Brings up a local Redis via docker compose,
# runs the example (enqueue -> worker -> handler round trip, self-asserts and
# exits non-zero on failure), then tears the container down. Skipped
# gracefully when docker is unavailable.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

if docker compose version >/dev/null 2>&1; then
    compose() { docker compose -p gs-asynq-example "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose -p gs-asynq-example "$@"; }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d

# Wait for redis to answer PING (up to 30s).
echo "== waiting for the service to be ready =="
for _ in $(seq 1 30); do
    if docker exec asynq-redis redis-cli ping 2>/dev/null | grep -q PONG; then
        break
    fi
    sleep 1
done

echo "== example boot =="
go run . > smoke.out 2>&1 &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!

rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

if [ "${rc}" -ne 0 ] || ! grep -q "Asynq round trip OK:" smoke.out; then
    echo "== FAILED ==" >&2
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
echo "== OK =="
rm -f smoke.out
exit 0
