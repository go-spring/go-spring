#!/usr/bin/env bash
#
# Smoke test for the go-kratos HTTP registry example. Brings up etcd via docker
# compose, starts the provider (registers the kratos HTTP server "kratos-http"
# into etcd, serves HTTP on :8000, writes JSON business logs to logs/), runs the
# consumer (discovers the provider through etcd, calls it over HTTP, asserts the
# echo, exits non-zero on failure), then tears everything down. Skipped
# gracefully when docker is unavailable.
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

# etcd (the registry) must be up before the provider tries to register.
wait_tcp 127.0.0.1 2379 || { echo "etcd not ready"; exit 1; }

# Start the provider; it registers into etcd and serves HTTP on :8000. Build to
# a temp binary and run it directly — "go run" would leave the compiled child
# orphaned when killed (the signal hits the go-run wrapper, not its child), so
# the provider would keep holding the port.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

wait_tcp 127.0.0.1 8000 || { echo "provider HTTP port not ready"; exit 1; }

# Run the consumer; it discovers the provider through etcd and calls it over
# HTTP. Capture stdout to assert on the echoed value.
consumer_out="$(mktemp)"
go run ./consumer 2>&1 | tee "${consumer_out}" || true

if ! grep -q "Response from discovered provider (HTTP): Hello Kratos" "${consumer_out}"; then
    echo "FAIL: consumer did not get the expected HTTP greeting"
    rm -f "${consumer_out}"
    exit 1
fi
rm -f "${consumer_out}"
echo "OK: consumer discovered the provider and got the expected HTTP greeting"

echo "smoke test passed"
