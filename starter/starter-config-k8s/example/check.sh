#!/usr/bin/env bash
#
# Smoke test for starter-config-k8s. Boots the example, which proves the starter
# wires into a Go-Spring app: the import is "optional:", so outside a cluster
# the read is skipped, the bound field shows its default, and the app
# self-terminates — a clean exit means the wiring is sound. Verifying a real
# ConfigMap read and its hot reload needs a live cluster: apply
# example/deploy/*.yaml (see README) and `kubectl edit configmap app-config`.
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
