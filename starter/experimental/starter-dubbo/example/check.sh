#!/usr/bin/env bash
#
# Smoke test for starter-dubbo. The example is a purely in-process Dubbo RPC
# server with no external service — it self-asserts and exits non-zero on failure.
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