#!/usr/bin/env bash
#
# Smoke test for starter-actuator. Runs the example, which self-asserts the
# /health, /readiness, and /info endpoints and exits non-zero on failure. No
# external services are required — the example registers an in-process health
# indicator to exercise the readiness aggregation.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run . &
pid=$!
( sleep 40; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
