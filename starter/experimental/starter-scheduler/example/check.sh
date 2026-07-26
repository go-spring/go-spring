#!/usr/bin/env bash
#
# Smoke test for starter-scheduler. Runs the example, which schedules
# fixed-rate/fixed-delay/cron jobs (plus a lock-guarded job backed by an
# in-process MemoryLocker), self-asserts that they fired, then exits. No docker
# is required — the lock backend is in-process.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out=$(mktemp)
go run . >"${out}" 2>&1 &
pid=$!
( sleep 30; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

cat "${out}"
if [[ "${rc}" -ne 0 ]]; then
    echo "ERROR: example exited with ${rc}"
    exit "${rc}"
fi
if ! grep -q "starter-scheduler smoke test passed" "${out}"; then
    echo "ERROR: expected success marker not found in output"
    exit 1
fi
