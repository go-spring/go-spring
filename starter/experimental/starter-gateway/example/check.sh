#!/usr/bin/env bash
#
# Smoke test for starter-gateway. The gateway and its upstream are both
# in-process (no external service), so this just runs the example, which
# self-asserts (a route strips the /api prefix and injects a header before
# forwarding to the upstream, and an unmatched path returns 404) and exits
# non-zero on failure.
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
