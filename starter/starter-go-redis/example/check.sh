#!/usr/bin/env bash
#
# Smoke test for starter-go-redis. Brings up a local Redis via docker compose,
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

# Wait for a TCP port to accept connections (up to $2 seconds, default 30).
wait_port() {
    local port="$1" tries="${2:-30}"
    for _ in $(seq 1 "${tries}"); do
        if (exec 3<>"/dev/tcp/127.0.0.1/${port}") 2>/dev/null; then
            exec 3>&- 3<&- 2>/dev/null || true
            return 0
        fi
        sleep 1
    done
    return 1
}

# Single (6379), sentinel (26379), and the cluster seed (7000). The cluster
# needs longer to form its slots, so give it a wider window.
wait_port 6379 30 || true
wait_port 26379 30 || true
wait_port 7000 60 || true

go run . &
pid=$!
( sleep 30; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
