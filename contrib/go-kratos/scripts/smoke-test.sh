#!/usr/bin/env bash
#
# Smoke test for the go-kratos registry + observability example. Brings up etcd
# plus the observability backends (Prometheus, Jaeger, Loki, Promtail) via
# docker compose, starts the provider (registers the kratos.App "kratos-greeter"
# into etcd, exposes a Prometheus endpoint on :9090, exports traces to Jaeger,
# writes JSON business logs to logs/provider.log), runs the consumer (discovers
# the provider through etcd, calls it over gRPC — one canonical call plus a
# batch — then dials the WebSocket transport and asserts an echo), then tears
# everything down. Skipped gracefully when docker is unavailable.
#
# Verification is "endpoint + liveness" (not "data landed"): we assert the
# provider's /metrics endpoint serves the kratos request counter, the RPCs
# round-trip, the counter climbs past the batch, and the backend containers stay
# up. Confirming data is actually queryable in Prometheus/Jaeger/Loki is a
# manual step documented in the README.
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

# Start the provider; it registers into etcd, serves gRPC on :9000, HTTP on
# :8000, WebSocket on :9002, and exposes Prometheus metrics on :9090. Build to a
# temp binary and run it directly — "go run" would leave the compiled child
# orphaned when killed (the signal hits the go-run wrapper, not its child), so
# the provider would keep holding the ports.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

# Wait for the gRPC, WebSocket and metrics ports (they come up in parallel under
# kratos.App.Run + the standalone metrics listener; any can win the race).
wait_tcp 127.0.0.1 9000 || { echo "provider gRPC port not ready"; exit 1; }
wait_tcp 127.0.0.1 9002 || { echo "provider WebSocket port not ready"; exit 1; }
wait_tcp 127.0.0.1 9090 || { echo "provider metrics port not ready"; exit 1; }

# Assertion 1: the metrics endpoint is up and serving. The kratos request
# counter is registered lazily, so the series only appears AFTER the first RPC —
# here we just assert the endpoint responds; the volume check below confirms the
# counter after the consumer has called.
if command -v curl >/dev/null 2>&1; then
    if ! curl -fs http://127.0.0.1:9090/metrics >/dev/null; then
        echo "FAIL: /metrics endpoint did not respond"
        exit 1
    fi
    echo "OK: provider serves /metrics on :9090"
else
    echo "WARNING: curl not found — skipping /metrics assertion"
fi

# Run the consumer; it discovers the provider, makes one canonical gRPC call
# plus a batch, then a WebSocket round-trip. Capture stdout to assert on the
# echoed values.
consumer_out="$(mktemp)"
go run ./consumer 2>&1 | tee "${consumer_out}" || true

# Assertion 2: the gRPC and WebSocket round-trips both succeeded.
if ! grep -q "Response from discovered provider (gRPC): Hello Kratos" "${consumer_out}"; then
    echo "FAIL: consumer did not get the expected gRPC greeting"
    rm -f "${consumer_out}"
    exit 1
fi
if ! grep -q "Response from discovered provider (WebSocket): Hello Kratos-WS" "${consumer_out}"; then
    echo "FAIL: consumer did not get the expected WebSocket greeting"
    rm -f "${consumer_out}"
    exit 1
fi
rm -f "${consumer_out}"
echo "OK: consumer discovered the provider and got the expected gRPC + WebSocket greetings"

# Assertion 3: the batch actually produced metric volume. The kratos request
# counter is served locally on /metrics and is cumulative, so after the consumer
# finishes it reflects every gRPC call (canonical + batch). WebSocket is NOT
# instrumented (no middleware chain), so it does not contribute. We assert the
# counter climbed past 10 — proof the enriched load reached the provider and is
# being counted (provider-local and deterministic; whether it then landed in
# Prometheus/Jaeger/Loki is the README's manual step).
if command -v curl >/dev/null 2>&1; then
    served="$(curl -fs http://127.0.0.1:9090/metrics \
        | awk '/^server_requests_code_total\{/ {v+=$NF} END {print v+0}')"
    if [ "${served:-0}" -lt 10 ]; then
        echo "FAIL: provider counted only ${served:-0} requests, expected >= 10"
        exit 1
    fi
    echo "OK: provider counted ${served} requests (enriched batch landed as metrics)"
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
