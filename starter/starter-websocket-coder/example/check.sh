#!/usr/bin/env bash
#
# Smoke test for starter-websocket-coder. Runs the example, which self-asserts
# WebSocket text/JSON echo and middleware gating (coder/websocket library).
# No external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run . &
pid=$!
( sleep 30; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"