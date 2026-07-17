#!/usr/bin/env bash
#
# Smoke test for the goframe HTTP example, extended to exercise the observability
# stack. Brings up etcd plus the observability backends (Prometheus, Jaeger,
# Loki, Promtail) via docker compose, starts the provider (registers
# goframe.hello into etcd, serves the Prometheus endpoint on :8000/metrics,
# exports traces to Jaeger over OTLP/HTTP, writes JSON logs via glog to logs/),
# runs the consumer (discovers the provider through etcd, calls it and
# self-asserts), then tears everything down. Skipped gracefully when docker is
# unavailable.
#
# Verification is "endpoint + liveness" (not "data landed"): we assert the
# provider's /metrics endpoint is live, the HTTP call round-trips, the bridged
# JSON log pipeline wrote provider.log, and the backend containers stay up.
# Confirming the data is actually queryable in Prometheus/Jaeger/Loki is a
# manual step (see README).
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

# Prefer the compose v2 plugin, fall back to the standalone docker-compose.
if docker compose version >/dev/null 2>&1; then
    compose() { docker compose "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose "$@"; }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

provider_pid=""
provider_bin=""
cleanup() {
    [ -n "${provider_pid}" ] && kill "${provider_pid}" 2>/dev/null || true
    [ -n "${provider_bin}" ] && rm -rf "$(dirname "${provider_bin}")" 2>/dev/null || true
    compose down -v >/dev/null 2>&1 || true
    rm -rf ./logs 2>/dev/null || true
}
trap cleanup EXIT

# Create the log dir up front so Promtail's bind-mount attaches to a
# host-created directory (not one docker fabricates as root).
mkdir -p ./logs

compose up -d

# wait_tcp HOST PORT — returns 0 once the port accepts a connection (up to 30s).
wait_tcp() {
    local host="$1" port="$2"
    for _ in $(seq 1 30); do
        if (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; then
            exec 3>&- 3<&- 2>/dev/null || true
            return 0
        fi
        sleep 1
    done
    return 1
}

# Wait for etcd (registry) and Jaeger's OTLP/HTTP endpoint (trace export target).
# Loki is best-effort: Promtail buffers, and log delivery is not asserted here.
wait_tcp 127.0.0.1 2379 || { echo "etcd not ready"; exit 1; }
wait_tcp 127.0.0.1 4318 || { echo "jaeger OTLP not ready"; exit 1; }

# Start the provider; it registers into etcd and serves HTTP + /metrics on :8000.
# Build to a temp binary and run it directly — "go run" would leave the compiled
# child orphaned when killed (the signal hits the go-run wrapper, not its child).
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

wait_tcp 127.0.0.1 8000 || { echo "provider HTTP port not ready"; exit 1; }

# Assertion 1: the metrics endpoint is live. The OTel Prometheus exporter always
# emits target_info, so its presence proves the metric pillar is wired.
if command -v curl >/dev/null 2>&1; then
    if ! curl -fs http://127.0.0.1:8000/metrics | grep -q 'target_info'; then
        echo "FAIL: /metrics did not expose Prometheus metrics"
        exit 1
    fi
    echo "OK: provider exposes Prometheus metrics on :8000/metrics"
else
    echo "WARNING: curl not found — skipping /metrics assertion"
fi

# Run the consumer; its exit code is the smoke-test result. Capture stdout to
# also assert on the echoed body for robustness.
consumer_out="$(mktemp)"
rc=0
go run ./consumer 2>&1 | tee "${consumer_out}" || rc=$?

# Assertion 2: the HTTP call round-tripped.
if ! grep -q "Hello World!" "${consumer_out}"; then
    echo "FAIL: consumer did not get the expected body"
    rm -f "${consumer_out}"
    exit 1
fi
rm -f "${consumer_out}"
echo "OK: consumer discovered the provider and got the expected body"

# Assertion 3: the log pillar produced output through the unified pipeline.
# The starter bridges goframe's glog into go-spring's log module, so the root
# FileLogger writes JSON lines (framework lifecycle + the business "handling
# hello request" line) to provider.log. We assert the file exists and is
# non-empty — proving the glog→go-spring bridge and the FileLogger sink are wired
# — rather than grepping glog's native "TraceId" field, which the bridged
# single-pipeline no longer emits (trace/log correlation now rides go-spring's
# own trace_id/span_id fields, matching the go-zero example). Whether the line
# then reached Loki is the README's manual step.
if [ -f ./logs/provider.log ] && [ -s ./logs/provider.log ]; then
    echo "OK: provider.log carries bridged JSON log lines"
else
    echo "FAIL: provider.log was not written"
    exit 1
fi

# Assertion 4: the observability backends are still up (none crashed).
if compose ps --status exited -q 2>/dev/null | grep -q .; then
    echo "FAIL: an observability backend container exited"
    compose ps
    exit 1
fi
echo "OK: observability backends (prometheus/jaeger/loki/promtail) are up"

echo "smoke test passed"
exit "${rc}"
