#!/usr/bin/env bash
#
# Smoke test for starter-xxljob. Self-contained: the example starts a mock
# xxl-job admin and drives the executor through its own protocol — no docker
# needed.
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

if [ "${rc}" -ne 0 ] || ! grep -q "xxl-job round trip OK:" smoke.out \
    || ! grep -q "kill round trip OK:" smoke.out \
    || ! grep -q "callback shape OK:" smoke.out; then
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
rm -f smoke.out
exit 0
