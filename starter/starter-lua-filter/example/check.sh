#!/usr/bin/env bash
#
# Smoke test for starter-lua-filter. The Lua VM is purely in-process with no
# external service, so this just runs the example, which self-asserts (a Lua
# filter allows /hello, blocks /admin without a token, then allows it with one)
# and exits non-zero on failure.
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
