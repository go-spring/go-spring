#!/usr/bin/env bash
#
# Smoke test for the observability showcase. The SAME dubbo-go Triple app
# (provider + consumer, run on the host) is pointed at one of several
# interchangeable backend "stacks" under stacks/. Pick the stack by name:
#
#   scripts/smoke-test.sh 1-classic     # Prometheus + Jaeger + Loki (direct, no collector)
#   scripts/smoke-test.sh 2-collector   # OTel Collector fan-out -> Jaeger/Prometheus/Loki
#   scripts/smoke-test.sh 3-lgtm        # OTel Collector -> Tempo/Prometheus/Loki (+correlation)
#   scripts/smoke-test.sh 5-elastic     # OTel Collector -> Elasticsearch/Kibana
#
# Default stack is 1-classic. The app itself is byte-for-byte identical across
# stacks — only which container owns :4317 / scrapes :9090 / tails ./logs
# changes. That is the whole point of this example.
#
# Verification is "endpoint + liveness" (not "data landed"): we assert the
# provider's /metrics serves dubbo_* series, the RPC round-trips, the request
# counter climbs, and the stack's containers stay up. Confirming the signals are
# actually queryable in each backend UI is the manual step in the README.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
ROOT="$(pwd)"

STACK="${1:-1-classic}"
STACK_DIR="${ROOT}/stacks/${STACK}"
if [ ! -f "${STACK_DIR}/docker-compose.yml" ]; then
    echo "unknown stack '${STACK}'. available:"
    ls -1 "${ROOT}/stacks"
    exit 2
fi
echo "=== observability smoke test: stack '${STACK}' ==="

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

# Prefer the compose v2 plugin, fall back to the standalone docker-compose.
if docker compose version >/dev/null 2>&1; then
    compose() { (cd "${STACK_DIR}" && docker compose "$@"); }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { (cd "${STACK_DIR}" && docker-compose "$@"); }
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
    rm -rf "${ROOT}/logs" 2>/dev/null || true
}
trap cleanup EXIT

# Create the log dir up front so the collector/Promtail bind-mount attaches to a
# host-created directory (not one docker fabricates as root).
mkdir -p "${ROOT}/logs"

compose up -d

# wait_tcp HOST PORT [TRIES] — returns 0 once the port accepts a connection.
wait_tcp() {
    local host="$1" port="$2" tries="${3:-30}"
    for _ in $(seq 1 "${tries}"); do
        if (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; then
            exec 3>&- 3<&- 2>/dev/null || true
            return 0
        fi
        sleep 1
    done
    return 1
}

# Elasticsearch is slow to warm up and the collector depends_on it being
# healthy, so give the OTLP ingress a longer runway on that stack.
otlp_tries=30
if [ "${STACK}" = "5-elastic" ]; then
    echo "waiting for Elasticsearch to warm up (this can take a minute)..."
    wait_tcp 127.0.0.1 9200 120 || { echo "elasticsearch not ready"; exit 1; }
    otlp_tries=120
fi

# etcd (registry) and the OTLP ingress (:4317 — Jaeger in stack 1, the collector
# in stacks 2/3/5) must be up before the app starts exporting.
wait_tcp 127.0.0.1 2379 || { echo "etcd not ready"; exit 1; }
wait_tcp 127.0.0.1 4317 "${otlp_tries}" || { echo "OTLP ingress (:4317) not ready"; exit 1; }

# Start the provider on the host; it registers into etcd, serves Triple on
# :20000, exposes Prometheus metrics on :9090, exports traces to :4317 and
# writes JSON logs to ./logs. Build to a temp binary and run it directly —
# "go run ./provider" would leave the compiled child orphaned when killed.
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
# :9091, logs to consumer.log), makes one canonical call plus a batch of further
# calls, so the backends receive real volume (many spans, climbing counters,
# several log lines). Capture stdout to assert on the echoed value.
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
# per-method request counter is served locally on /metrics and is cumulative.
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

# Assertion 4: the stack's containers are still up (none crashed).
if compose ps --status exited -q 2>/dev/null | grep -q .; then
    echo "FAIL: a backend container exited"
    compose ps
    exit 1
fi
echo "OK: stack '${STACK}' backends are up"

echo "smoke test passed (stack: ${STACK})"
