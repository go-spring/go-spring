#!/usr/bin/env bash
#
# Observability smoke test for starter-gin. Runs the example, which
# self-asserts:
#   (1) trace context propagation (W3C traceparent),
#   (2) Prometheus /metrics on the actuator management port (:9370), and
#   (3) trace_id/span_id log correlation.
# Exits non-zero on failure. No external services required.
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