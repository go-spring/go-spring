#!/usr/bin/env bash
#
# Smoke test for the starter-echo cloud-native flagship. Runs the app, which
# self-asserts all four capabilities (health, service discovery, resilience,
# dynamic config) and exits non-zero on failure. No external services are
# required — the app resolves a static discovery backend pointing at itself and
# hot-reloads a watched file locally.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "== example boot =="
go run -gcflags="all=-N -l" . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
if [ "${rc}" -ne 0 ]; then
    echo "== FAILED ==" >&2
    exit "${rc}"
fi
echo "== OK =="
exit 0
