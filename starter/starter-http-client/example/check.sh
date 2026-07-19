#!/usr/bin/env bash
#
# Smoke test for the declarative HTTP client example. Runs the example, which
# self-asserts the four acceptance outcomes end to end and exits non-zero on
# failure: (1) direct-address call, (2) service discovery + round-robin load
# balancing, (3) trace propagation across both ends, and (4) the resilience
# breaker opening and fast-failing. Everything runs in-process; no external
# services are required.
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
