#!/usr/bin/env bash
#
# Smoke test for starter-transaction-at-gorm. The AT coordinator and its global
# lock are purely in-process and the two databases are in-memory sqlite, so this
# just runs the example, which drives a commit, a rollback (restored from the
# captured before-images) and a write-write conflict, self-asserts the database
# states, and exits non-zero on failure.
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
