#!/usr/bin/env bash
#
# Smoke test for starter-cassandra. Brings up a local Cassandra 5 via docker
# compose, runs the example (which self-asserts a create/insert/read round
# trip and exits non-zero on failure), then tears the container down. Skipped
# gracefully when docker is unavailable.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

# Prefer the compose v2 plugin, fall back to the standalone docker-compose.
if docker compose version >/dev/null 2>&1; then
    compose() { docker compose -p gs-cassandra-example "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose -p gs-cassandra-example "$@"; }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d

# Wait for CQL readiness by asking cqlsh inside the container (up to 240s;
# Cassandra boot is slow, and the CQL layer accepts queries well after the
# native port opens). The container-internal probe also avoids any host-side
# network quirks.
cass_ready=false
echo "== waiting for the service to be ready =="
for _ in $(seq 1 120); do
    if docker exec cassandra-example cqlsh -e "DESCRIBE CLUSTER" >/dev/null 2>&1; then
        cass_ready=true
        break
    fi
    sleep 2
done
if [ "${cass_ready}" != true ]; then
    echo "ERROR: cassandra CQL not ready in 240s"
    docker logs cassandra-example >&2 2>&1 | tail -20 || true
    exit 1
fi

echo "== example boot =="
go run . > smoke.out 2>&1 &
pid=$!
( sleep 90; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!

rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

# gs.Run returns (exit code 0) even when bean wiring fails at startup, so gate
# on the example's success marker rather than the exit code alone.
if [ "${rc}" -ne 0 ] || ! grep -q "Cassandra round trip OK:" smoke.out; then
    echo "== FAILED ==" >&2
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
echo "== OK =="
rm -f smoke.out
exit 0
