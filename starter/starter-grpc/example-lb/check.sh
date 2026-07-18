#!/usr/bin/env bash
#
# Smoke test for the gRPC client-side load-balancing adapter built on
# go-spring.org/stdlib/loadbalance. The example starts three in-process Echo
# backends and drives one balanced gRPC client, asserting even distribution,
# breaker eviction + recovery, and instance-kill drop. It self-asserts and exits
# non-zero on failure, so no external services (or docker) are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
