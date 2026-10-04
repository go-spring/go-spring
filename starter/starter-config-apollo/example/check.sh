#!/usr/bin/env bash
#
# Smoke test for starter-config-apollo. Self-contained: the example starts a
# mock Apollo config service, imports the starter, and asserts the remote
# property cold-loads into a Dync field and then hot-reloads after a publish —
# no docker needed.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# Unlike its siblings, this example's output is captured (not streamed) because
# the gate below greps it. Print the markers back on success so a run still
# shows what it verified.
echo "== example boot =="
go run . > smoke.out 2>&1 &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!

rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

if [ "${rc}" -ne 0 ] || ! grep -q "Apollo cold-load OK:" smoke.out || ! grep -q "hot-reload observed:" smoke.out; then
    echo "== FAILED ==" >&2
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
grep -E "Apollo cold-load OK:|hot-reload observed:" smoke.out
echo "== OK =="
rm -f smoke.out
exit 0
