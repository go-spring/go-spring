#!/usr/bin/env bash
#
# Smoke test for starter-batch + starter-batch-redis. Brings up a local Redis
# via docker compose, then runs the example TWICE against the same Redis:
#
#   PHASE=1 crashes (os.Exit) once ~half the items have committed — expected to
#           exit non-zero, leaving committed chunks + checkpoint durable in Redis.
#   PHASE=2 resumes the same job instance from the checkpoint and finishes,
#           self-asserting every item was written exactly once and printing the
#           success marker.
#
# Skipped gracefully when docker is unavailable.
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

wait_port 6379 30 || {
    echo "ERROR: redis 6379 did not come up"
    exit 1
}

# ---- PHASE 1: expected to crash (non-zero exit) --------------------------
echo "=== PHASE 1: run until mid-run crash ==="
out1=$(mktemp)
rc1=0
PHASE=1 go run . >"${out1}" 2>&1 || rc1=$?
cat "${out1}"
if [[ "${rc1}" -eq 0 ]]; then
    echo "ERROR: PHASE 1 was expected to crash (non-zero exit) but exited 0"
    exit 1
fi
echo "PHASE 1 exited non-zero as expected (${rc1})"

# ---- PHASE 2: resume from checkpoint, must complete ----------------------
echo "=== PHASE 2: resume from checkpoint ==="
out2=$(mktemp)
PHASE=2 go run . >"${out2}" 2>&1 &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc2=0
wait "${pid}" 2>/dev/null || rc2=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

cat "${out2}"
if [[ "${rc2}" -ne 0 ]]; then
    echo "ERROR: PHASE 2 exited with ${rc2}"
    exit "${rc2}"
fi
if ! grep -q "starter-batch smoke test passed" "${out2}"; then
    echo "ERROR: expected success marker not found in PHASE 2 output"
    exit 1
fi
echo "smoke test OK"
