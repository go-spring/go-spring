#!/usr/bin/env bash
#
# Smoke test for the starter-kafka cloud-native flagship. Brings up a single-node
# KRaft Kafka via docker compose, runs the example (which self-asserts health,
# resilience and dynamic config over a produce -> consume round-trip), then tears
# the container down. Skipped gracefully when docker is unavailable.
#
# Kafka is slow to boot, so the watchdog allows a longer window (90s) than the
# other starters.
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

# Wait for Kafka to accept connections (up to 90s; broker boot is slow).
echo "== waiting for the service to be ready =="
for _ in $(seq 1 90); do
    if (exec 3<>"/dev/tcp/127.0.0.1/9092") 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        break
    fi
    sleep 1
done
# Give the broker a moment to finish initializing after the port opens.
sleep 5

echo "== example boot =="
go run -gcflags="all=-N -l" . &
pid=$!
( sleep 90; kill -9 "${pid}" 2>/dev/null ) &
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
