#!/usr/bin/env bash
#
# Smoke test for starter-security-jwt. Verification uses a shared HMAC secret, so
# there is no external identity provider — the example mints its own tokens and
# self-asserts (missing/invalid tokens are rejected, a valid token authenticates,
# and a method-level authority check gates /admin), exiting non-zero on failure.
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
