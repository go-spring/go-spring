#!/usr/bin/env bash
#
# Smoke test for service-mesh mode built on go-spring.org/stdlib/discovery and
# go-spring.org/stdlib/loadbalance. The example runs the same client code twice —
# mesh off and mesh on — asserting that mesh off spreads requests across three
# real endpoints while mesh on degrades to a single stable endpoint and never
# resolves the discovery backend. It self-asserts and exits non-zero on failure,
# so no external services (or docker) are required.
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
