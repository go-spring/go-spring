#!/usr/bin/env bash
#
# Smoke test for the etcd service registration & discovery example. Brings up a
# local etcd via docker compose, starts the provider (registers greet.GreetService
# into etcd), runs the consumer (discovers the provider through etcd, calls it and
# asserts the echoed value), then tears everything down. Skipped gracefully when
# docker is unavailable.
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
}
trap cleanup EXIT

compose up -d

# wait_tcp HOST PORT — returns 0 once the port accepts a connection (up to 60s).
wait_tcp() {
    local host="$1" port="$2"
    for _ in $(seq 1 60); do
        if (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; then
            exec 3>&- 3<&- 2>/dev/null || true
            return 0
        fi
        sleep 1
    done
    return 1
}

wait_tcp 127.0.0.1 2181 || { echo "registry not ready"; exit 1; }

# Start the provider; it registers into the registry and serves Triple on :20000.
# Build to a temp binary and run it directly — "go run ./provider" would leave the
# compiled child orphaned when killed.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

wait_tcp 127.0.0.1 20000 || { echo "provider triple port not ready"; exit 1; }

# Run the consumer and assert the RPC round-tripped.
consumer_out="$(mktemp)"
go run ./consumer 2>&1 | tee "${consumer_out}" || true

if ! grep -q "Response from discovered provider: Hello, Dubbo-Go!" "${consumer_out}"; then
    echo "FAIL: consumer did not get the expected greeting"
    rm -f "${consumer_out}"
    exit 1
fi
rm -f "${consumer_out}"
echo "OK: consumer discovered the provider through zookeeper and got the expected greeting"
echo "smoke test passed"
