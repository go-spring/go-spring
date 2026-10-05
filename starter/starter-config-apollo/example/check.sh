#!/usr/bin/env bash
#
# Smoke test for starter-config-apollo. Self-contained: the example starts a
# mock Apollo config service, imports the starter, and asserts the remote
# property cold-loads into a Dync field and then hot-reloads after a publish —
# no docker needed.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "== example boot =="
go run . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
if [ "${rc}" -ne 0 ]; then
    echo "== FAILED ==" >&2
    exit "${rc}"
fi
echo "== OK =="
exit 0
