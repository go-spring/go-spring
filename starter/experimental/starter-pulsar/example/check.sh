#!/usr/bin/env bash
#
# Smoke test for starter-pulsar. Brings up a local Pulsar via docker compose,
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

# Wait for Pulsar to accept connections (up to 90s; standalone boot is slow).
for _ in $(seq 1 90); do
    if (exec 3<>/dev/tcp/127.0.0.1/6650) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        break
    fi
    sleep 1
done
# The 6650 port opens well before the broker is actually ready, so gate on the
# admin health endpoint instead of a fixed sleep (up to another 60s).
for _ in $(seq 1 60); do
    if curl -fsS http://127.0.0.1:8080/admin/v2/brokers/health 2>/dev/null | grep -qi ok; then
        break
    fi
    sleep 1
done

go run . &
pid=$!
( sleep 40; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!

# Scrape the native Prometheus /metrics endpoint the starter exposes while the
# example is running. Best-effort: a failure here does not fail the smoke, but a
# success confirms pulsar_client_* metrics are actually served.
( for _ in $(seq 1 20); do
    if curl -fsS http://127.0.0.1:9091/metrics 2>/dev/null | grep -q "pulsar_client_"; then
        echo "METRICS OK: pulsar_client_* served on :9091/metrics"
        break
    fi
    sleep 1
  done ) &

rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
