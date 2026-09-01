#!/usr/bin/env bash
#
# Smoke test for the cloudnative flagship example. Runs the app, which
# self-asserts all four cloud-native capabilities (health, service discovery,
# resilience, dynamic config) and exits non-zero on failure. No external
# services are required — the app resolves a static discovery backend pointing
# at itself and hot-reloads a watched file locally.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run -gcflags="all=-N -l" . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
