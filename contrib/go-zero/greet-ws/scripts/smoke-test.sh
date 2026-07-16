#!/usr/bin/env bash
#
# Smoke test for the go-zero WebSocket example, extended to exercise the
# built-in observability stack. Brings up the observability backends
# (Prometheus, Jaeger, Loki, Promtail) via docker compose, starts the provider
# (serves WS on :8890, exposes go-zero's DevServer /metrics on :6060, exports
# traces to Jaeger, writes JSON logs to logs/), runs the WS consumer (dials
# the provider, exchanges one JSON frame and self-asserts, exits non-zero on
# mismatch), then tears everything down. Skipped gracefully when docker is
# unavailable.
#
# WS rides on the same rest.Server as greet-api, so go-zero wires the three
# pillars natively inside rest.MustNewServer via ServiceConf.SetUp() — no
# hand-written OTel/Prometheus code. The consumer is a raw gorilla/websocket
# client and is not instrumented, so only the provider is scraped and only
# provider logs land in Loki.
#
# Verification is "endpoint + liveness" (not "data landed"): confirming the
# metrics/traces/logs are actually queryable in Prometheus/Jaeger/Loki is a
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

# Wait for Jaeger's OTLP endpoint (trace export target). Loki is best-effort:
# Promtail buffers, and log delivery is not asserted here.
wait_tcp 127.0.0.1 4317 || { echo "jaeger OTLP not ready"; exit 1; }

# Start the provider; it chdirs into the module root, loads conf/app.properties,
# serves WS on :8890 (the HTTP upgrade handshake shares that port) and exposes
# DevServer /metrics on :6060. Build to a temp binary and run it directly —
# "go run ./provider" would leave the compiled child orphaned when killed (the
# signal hits the go-run wrapper, not its child), so the provider would keep
# holding the port.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

wait_tcp 127.0.0.1 8890 || { echo "provider rest port not ready"; exit 1; }
wait_tcp 127.0.0.1 6060 || { echo "provider metrics port not ready"; exit 1; }

# Run the consumer first so at least one WS frame exchange is recorded, then
# assert on /metrics. Its exit code IS the RPC result — the consumer os.Exit(1)s
# on any failure.
rc=0
go run ./consumer || rc=$?
if [ "${rc}" -ne 0 ]; then
    echo "FAIL: consumer exited with code ${rc}"
    exit 1
fi
echo "OK: consumer dialed the provider and got the expected greeting"

# Assertion: the DevServer /metrics endpoint is up and serving. We assert on
# go_* process metrics (always exposed by the default Prometheus registry) —
# NOT on http_server_requests_*. Reason: WS is a long-lived connection; the
# rest.Server Prometheus middleware only records the http_server_requests_*
# sample when the connection closes, so its timing is inherently racy relative
# to this assertion. go_* metrics are request-independent and stable.
if command -v curl >/dev/null 2>&1; then
    if ! curl -fs http://127.0.0.1:6060/metrics | grep -q '^go_'; then
        echo "FAIL: /metrics did not expose go_* process metrics"
        exit 1
    fi
    echo "OK: provider exposes go_* metrics on :6060/metrics"
else
    echo "WARNING: curl not found — skipping /metrics assertion"
fi

# Assertion: the observability backends are still up (none crashed).
if compose ps --status exited -q 2>/dev/null | grep -q .; then
    echo "FAIL: an observability backend container exited"
    compose ps
    exit 1
fi
echo "OK: observability backends (prometheus/jaeger/loki/promtail) are up"

echo "smoke test passed"
