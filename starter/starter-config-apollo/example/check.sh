#!/usr/bin/env bash
#
# Smoke test for starter-config-apollo. Self-contained: the example starts a
# mock Apollo config service, imports the starter, and asserts the remote
# property cold-loads into a Dync field — no docker needed.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run . > smoke.out 2>&1 &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!

rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

if [ "${rc}" -ne 0 ] || ! grep -q "Apollo cold-load OK:" smoke.out; then
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
rm -f smoke.out
exit 0
