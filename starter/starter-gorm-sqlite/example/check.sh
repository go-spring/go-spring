#!/usr/bin/env bash
#
# Smoke test for starter-gorm-sqlite. SQLite is file/in-memory based — there
# is no server and no docker; we just run the example directly and let it
# self-assert. A 60s watchdog guards against hangs.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

CGO_ENABLED=1 go run . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
