#!/usr/bin/env bash
#
# Smoke test for the consolidated observability example. Runs the example,
# which self-asserts (1) trace_id/span_id log correlation and (2) that otel's
# Prometheus /metrics is served on the actuator management port (:9370), then
# exits non-zero on failure. No external services are required — tracing uses
# the stdout exporter and metrics are scraped in-process.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
