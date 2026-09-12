#!/usr/bin/env bash
#
# Smoke test for the resilience seams added on top of the stdlib abstraction.
# The example spins up in-process HTTP servers and exercises the inbound Handler
# seam (rate limiting), the client Dialer seam (circuit breaking) and a composed
# rate-limit + breaker + retry policy, all carried by the sentinel driver. It
# self-asserts and exits non-zero on failure, so no external services (or docker)
# are required.
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
