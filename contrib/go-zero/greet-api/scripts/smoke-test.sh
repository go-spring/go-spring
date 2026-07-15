#!/usr/bin/env bash
#
# Smoke test for the go-zero REST example. Builds the provider, starts it,
# waits for the port to accept connections, runs the consumer (which calls
# the provider over HTTP and self-asserts, exiting non-zero on failure), and
# tears the provider down.
#
# No docker/etcd is involved: go-zero's rest.Server has no built-in service
# discovery, so there is nothing to register into — the consumer dials a
# fixed host:port. See README.md for the reasoning.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

provider_pid=""
provider_bin=""
cleanup() {
    [ -n "${provider_pid}" ] && kill "${provider_pid}" 2>/dev/null || true
    [ -n "${provider_bin}" ] && rm -rf "$(dirname "${provider_bin}")" 2>/dev/null || true
}
trap cleanup EXIT

# Build to a temp binary and run it directly — "go run" would leave the
# compiled child orphaned when killed (the signal hits the go-run wrapper,
# not its child), so the provider would keep holding the port.
provider_bin="$(mktemp -d)/provider"
go build -o "${provider_bin}" ./provider
"${provider_bin}" &
provider_pid=$!

# Wait for the provider's REST port to accept connections (up to 30s).
for _ in $(seq 1 30); do
    if (exec 3<>/dev/tcp/127.0.0.1/8888) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        break
    fi
    sleep 1
done

# Run the consumer; its exit code is the smoke-test result.
rc=0
go run ./consumer || rc=$?
exit "${rc}"
