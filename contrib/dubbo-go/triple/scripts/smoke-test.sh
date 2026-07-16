#!/usr/bin/env bash
#
# Smoke test for the dubbo-go Triple (gRPC/HTTP2) example, extended to exercise
# the built-in observability stack. Brings up etcd plus the observability
# backends (Prometheus, Jaeger, Loki, Promtail) via docker compose, starts the
# provider (registers greet.GreetService into etcd, exposes Prometheus metrics
# on :9090, exports traces to Jaeger, writes JSON logs to logs/), runs the
# consumer (discovers the provider, calls it, and prints the echoed value),
# then tears everything down. Skipped gracefully when docker is unavailable.
#
# Verification is "endpoint + liveness" (not "data landed"): we assert the
# provider's /metrics endpoint serves dubbo metrics, the RPC round-trips, and
# the backend containers stay up. Confirming the data is actually queryable in
# Prometheus/Jaeger/Loki is a manual step documented in the README.
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

# Wait for etcd (registry) and Jaeger's OTLP endpoint (trace export target).
# Loki is best-effort: Promtail buffers, and log delivery is not asserted here.
wait_tcp 127.0.0.1 2379 || { echo "etcd not ready"; exit 1; }
wait_tcp 127.0.0.1 4317 || { echo "jaeger OTLP not ready"; exit 1; }

# Start the provider; it registers into etcd, serves Triple on :20000 and
# exposes Prometheus metrics on :9090. Build to a temp binary and run it
# directly — "go run ./provider" would leave the compiled child orphaned when
# killed.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

wait_tcp 127.0.0.1 20000 || { echo "provider triple port not ready"; exit 1; }
wait_tcp 127.0.0.1 9090  || { echo "provider metrics port not ready"; exit 1; }

# Assertion 1: the metrics endpoint serves dubbo metrics.
if command -v curl >/dev/null 2>&1; then
    if ! curl -fs http://127.0.0.1:9090/metrics | grep -q '^dubbo_'; then
        echo "FAIL: /metrics did not expose dubbo_* metrics"
        exit 1
    fi
    echo "OK: provider exposes dubbo_* metrics on :9090/metrics"
else
    echo "WARNING: curl not found — skipping /metrics assertion"
fi

# Run the consumer. It loads its own consumer/conf/app.properties (metrics on
# :9091, logs to consumer.log), so it no longer collides with the provider and
# needs no env-var overrides. The consumer makes one canonical call plus a
# batch of further calls, so the observability backends receive real volume
# (many spans, climbing counters, several log lines) rather than a single
# sample. Capture stdout to assert on the echoed value — the consumer runs via
# gs.Run, which exits 0 even if wiring fails, so the exit code alone is not a
# reliable smoke signal.
consumer_out="$(mktemp)"
go run ./consumer 2>&1 | tee "${consumer_out}" || true

# Assertion 2: the RPC round-tripped.
if ! grep -q "Response from discovered provider: Hello, Dubbo-Go!" "${consumer_out}"; then
    echo "FAIL: consumer did not get the expected greeting"
    rm -f "${consumer_out}"
    exit 1
fi
rm -f "${consumer_out}"
echo "OK: consumer discovered the provider and got the expected greeting"

# Assertion 3: the batch actually produced metric volume. The provider's
# per-method request counter is served locally on /metrics and is cumulative,
# so after the consumer finishes it reflects every call (canonical + batch).
# We assert it climbed well past 1 — proof the enriched load reached the
# provider and is being counted (this is provider-local and deterministic;
# whether it then landed in Prometheus/Jaeger/Loki is the README's manual step).
if command -v curl >/dev/null 2>&1; then
    served="$(curl -fs http://127.0.0.1:9090/metrics \
        | awk '/^dubbo_provider_requests_total\{.*method="Greet".*\}/ {v=$NF} END {print v+0}')"
    if [ "${served:-0}" -lt 10 ]; then
        echo "FAIL: provider counted only ${served:-0} Greet requests, expected >= 10"
        exit 1
    fi
    echo "OK: provider counted ${served} Greet requests (enriched batch landed as metrics)"
else
    echo "WARNING: curl not found — skipping request-volume assertion"
fi

# Assertion 4: the observability backends are still up (none crashed).
if compose ps --status exited -q 2>/dev/null | grep -q .; then
    echo "FAIL: an observability backend container exited"
    compose ps
    exit 1
fi
echo "OK: observability backends (prometheus/jaeger/loki/promtail) are up"

echo "smoke test passed"
