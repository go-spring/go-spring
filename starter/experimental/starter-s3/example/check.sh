#!/usr/bin/env bash
#
# Smoke test for starter-s3. Brings up a local MinIO via docker compose, runs
# the example (which self-asserts an object round trip and exits non-zero on
# failure), then tears the containers down. Skipped gracefully when docker is
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
    compose() { docker compose -p gs-s3-example "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose -p gs-s3-example "$@"; }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d

# Wait for the MinIO health endpoint (up to 60s).
echo "== waiting for the service to be ready =="
for _ in $(seq 1 60); do
    if curl -fsS http://127.0.0.1:9000/minio/health/live >/dev/null 2>&1; then
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

# gs.Run returns (exit code 0) even when bean wiring fails at startup, so gate
# on the example's success marker rather than the exit code alone.
if [ "${rc}" -ne 0 ] || ! grep -q "Object round trip OK:" smoke.out; then
    echo "== FAILED ==" >&2
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
echo "== OK =="
rm -f smoke.out
exit 0
