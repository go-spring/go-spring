#!/usr/bin/env bash
#
# Smoke test for starter-migration-gorm. The database is an in-memory sqlite, so
# this just runs the example, which lets the starter apply the embedded
# migrations at startup, then self-asserts startup apply, second-run idempotency
# and checksum-drift fail-fast, and exits non-zero on failure.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

CGO_ENABLED=1 go run . &
pid=$!
( sleep 30; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
