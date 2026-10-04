#!/usr/bin/env bash
#
# Smoke test for starter-milvus. Brings up a Milvus standalone (etcd + minio +
# milvus) via docker compose, runs the example (create/insert/load/search
# round trip), then tears it down. Skipped gracefully when docker is
# unavailable.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi
if docker compose version >/dev/null 2>&1; then
    compose() { docker compose -p gs-milvus-example "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose -p gs-milvus-example "$@"; }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d

# Wait for the gRPC port (up to 120s; milvus boot is slow).
echo "== waiting for the service to be ready =="
for _ in $(seq 1 120); do
    if (exec 3<>/dev/tcp/127.0.0.1/19530) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        break
    fi
    sleep 1
done

echo "== example boot =="
go run . > smoke.out 2>&1 &
pid=$!
( sleep 90; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!

rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

if [ "${rc}" -ne 0 ] || ! grep -q "Milvus round trip OK:" smoke.out; then
    echo "== FAILED ==" >&2
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
echo "== OK =="
rm -f smoke.out
exit 0
