#!/usr/bin/env bash
#
# Smoke test for the dubbo-go REST (HTTP/1.1 + go-restful) example, extended to
# exercise the built-in observability stack. Brings up etcd plus the
# observability backends (Prometheus, Jaeger, Loki, Promtail) via docker
# compose, starts the provider (registers com.example.GreetService into etcd,
# serves REST on :20003, exposes Prometheus metrics on :9090, exports traces to
# Jaeger, writes JSON logs to logs/), runs the raw dubbo-go consumer (discovers
# the provider, calls it, self-asserts, and exits non-zero on failure), then
# tears everything down. Skipped gracefully when docker is unavailable.
#
# Unlike the Triple/Dubbo/JSON-RPC sibling smoke tests, the consumer here is a
# STANDALONE raw dubbo-go main() (starter-dubbo's client bean has no REST
# protocol support), so it is not instrumented and makes only ONE call — the
# request-volume assertion those siblings make is intentionally omitted; we
# assert the RPC round-tripped and the metrics endpoint served dubbo_* metrics.
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

# Wait for etcd (registry) and Jaeger's OTLP endpoint (trace export target).
# Loki is best-effort: Promtail buffers, and log delivery is not asserted here.
wait_tcp 127.0.0.1 2379 || { echo "etcd not ready"; exit 1; }
wait_tcp 127.0.0.1 4317 || { echo "jaeger OTLP not ready"; exit 1; }

# Start the provider; it chdirs into the module root, loads conf/app.properties,
# registers into etcd, serves REST on :20003 and exposes Prometheus metrics on
# :9090. Build to a temp binary and run it directly — "go run ./provider" would
# leave the compiled child orphaned when killed (the signal hits the go-run
# wrapper, not its child), so the provider would keep holding the port.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

wait_tcp 127.0.0.1 20003 || { echo "provider rest port not ready"; exit 1; }
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

# Run the raw consumer. Its exit code IS the RPC result — the raw dubbo-go
# client.NewClient + Dial + CallUnary main() os.Exit(1)s on any failure. We
# still capture stdout to assert on the echoed line, because the consumer makes
# a single call only (no batch), so there is no request-volume assertion.
consumer_out="$(mktemp)"
rc=0
go run ./consumer 2>&1 | tee "${consumer_out}" || rc=$?
if [ "${rc}" -ne 0 ]; then
    echo "FAIL: consumer exited with code ${rc}"
    rm -f "${consumer_out}"
    exit 1
fi

# Assertion 2: the RPC round-tripped.
if ! grep -q "Response from discovered provider: Hello, Dubbo-Go!" "${consumer_out}"; then
    echo "FAIL: consumer did not get the expected greeting"
    rm -f "${consumer_out}"
    exit 1
fi
rm -f "${consumer_out}"
echo "OK: consumer discovered the provider and got the expected greeting"

# Assertion 3: the observability backends are still up (none crashed).
if compose ps --status exited -q 2>/dev/null | grep -q .; then
    echo "FAIL: an observability backend container exited"
    compose ps
    exit 1
fi
echo "OK: observability backends (prometheus/jaeger/loki/promtail) are up"

echo "smoke test passed"
