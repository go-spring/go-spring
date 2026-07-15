#!/usr/bin/env bash
#
# Smoke test for the dubbo-go registry example. Brings up a local etcd via
# docker compose, starts the provider (which registers greet.GreetService into
# etcd), runs the consumer (which discovers the provider through etcd, calls it
# and self-asserts, exiting non-zero on failure), then tears everything down.
# Skipped gracefully when docker is unavailable.
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

# Wait for etcd to accept TCP connections (up to 30s).
for _ in $(seq 1 30); do
    if (exec 3<>/dev/tcp/127.0.0.1/2379) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        break
    fi
    sleep 1
done

# Start the provider; it registers into etcd and serves on :20000.
# Build the provider to a temp binary and run it directly. "go run ./provider"
# would leave the compiled child orphaned when killed (the signal hits the
# go-run wrapper, not its child), so the provider would keep holding the port.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

# Wait for the provider's Dubbo port to accept connections (up to 30s).
for _ in $(seq 1 30); do
    if (exec 3<>/dev/tcp/127.0.0.1/20000) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        break
    fi
    sleep 1
done

# Run the consumer; its exit code is the smoke-test result.
rc=0
go run ./consumer || rc=$?
exit "${rc}"
