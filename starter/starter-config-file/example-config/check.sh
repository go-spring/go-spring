#!/usr/bin/env bash
#
# Smoke test for starter-config-file. Runs the example, which lays down a
# Kubernetes-style ConfigMap mount (with the ..data symlink swap), binds a
# gs.Dync field to a value from it, rewrites the mount atomically, and asserts
# the bound field hot-reloads. It exits non-zero on failure. No external
# services are required.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run . &
pid=$!
( sleep 40; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
