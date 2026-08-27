#!/usr/bin/env bash
#
# Smoke test for starter-outbox-gorm. The database is in-memory sqlite and the
# binder is an in-process "mem" implementation, so this just runs the example,
# which publishes committed / rolled-back / flaky / poison records, waits for
# the relay to settle, self-asserts delivery, retry and dead-letter behavior,
# and exits non-zero on failure.
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
if [ "${rc}" -ne 0 ]; then
  echo "outbox example FAILED (exit ${rc})"
  exit "${rc}"
fi
echo "outbox example smoke test passed"
