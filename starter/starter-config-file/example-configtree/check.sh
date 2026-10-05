#!/usr/bin/env bash
#
# Smoke test for starter-config-file (configtree). Runs the example, which lays
# down a Kubernetes-style Secret mount (flat scalar key files with the ..data
# symlink swap), binds gs.Dync fields to values from it, rewrites the mount
# atomically, and asserts the bound fields hot-reload. It exits non-zero on
# failure. No external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "== example boot =="
go run . &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
if [ "${rc}" -ne 0 ]; then
    echo "== FAILED ==" >&2
    exit "${rc}"
fi
echo "== OK =="
exit 0
