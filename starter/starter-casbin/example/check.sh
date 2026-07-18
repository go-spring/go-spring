#!/usr/bin/env bash
#
# Smoke test for starter-casbin. Casbin runs fully in-process against local
# model/policy files, so there is nothing to spin up: just run the example,
# which self-asserts the RBAC decisions and exits non-zero on failure.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
