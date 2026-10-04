#!/usr/bin/env bash
#
# Smoke test for starter-gorm-sqlite. Self-contained: SQLite is in-process, so
# there is no docker dependency — the example runs a migrate/CRUD/transaction
# round trip against an in-memory database and self-exits.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

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
if [ "${rc}" -ne 0 ] || ! grep -q "SQLite round trip OK:" smoke.out; then
    echo "== FAILED ==" >&2
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
echo "== OK =="
rm -f smoke.out
exit 0
