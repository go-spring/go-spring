#!/usr/bin/env bash
#
# Smoke test for starter-admin-ui. Runs the example, which stands up two fake
# actuator instances on 127.0.0.1:19371 and 127.0.0.1:19372, lets the Admin UI
# poll them, then asserts that the dashboard renders both as UP and the JSON
# status endpoint returns matching data. Exits non-zero on any failure. No
# external services are required.
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
