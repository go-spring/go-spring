#!/usr/bin/env bash
#
# Smoke test for starter-bigcache/example-otel. The example generates BigCache
# traffic, scrapes the Prometheus exporter at :9090/metrics, self-asserts the
# bigcache.* gauges appear labeled with the instance name, and exits non-zero on
# failure. No external service is required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "== example boot =="
go run . &
pid=$!
( sleep 30; kill -9 "${pid}" 2>/dev/null ) &
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
