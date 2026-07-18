#!/usr/bin/env bash
#
# Smoke test for the stdlib/event container-integration example. Runs the
# example, which wires listener beans onto the event bus, publishes a
# ConfigChanged event, self-asserts synchronous ordering and asynchronous
# delivery, and exits non-zero on failure. No external services are required.
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
