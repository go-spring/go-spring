#!/usr/bin/env bash
#
# Smoke test for the Consul service registration & discovery example. Brings up a
# local Consul agent via docker compose, starts the provider (which registers the
# echo service into Consul), runs the consumer (which discovers the provider
# through Consul over both KitexProtobuf and gRPC, calls it and self-asserts,
# exiting non-zero on failure), then tears everything down. Skipped gracefully
# when docker is unavailable.
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

wait_tcp 127.0.0.1 8500 || { echo "consul not ready"; exit 1; }

# Start the provider; it registers into Consul and serves on :8888. Build to a
# temp binary and run it directly — "go run" would leave the compiled child
# orphaned when killed.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

wait_tcp 127.0.0.1 8888 || { echo "provider kitex port not ready"; exit 1; }

# Consul marks a freshly registered service healthy only after its first check
# interval; give the TCP health check a moment before the consumer resolves.
sleep 3

# Run the consumer; its exit code is the smoke-test result.
rc=0
go run ./consumer || rc=$?
exit "${rc}"
