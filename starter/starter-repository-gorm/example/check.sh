#!/usr/bin/env bash
#
# Smoke test for starter-repository-gorm. The repository is built over an
# in-memory sqlite *gorm.DB entirely in-process, so this just runs the example,
# which drives CRUD, paging, composite conditions and audit-field population,
# self-asserts every expectation, and exits non-zero on the first failure.
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
