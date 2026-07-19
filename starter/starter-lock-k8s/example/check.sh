#!/usr/bin/env bash
#
# Smoke test for starter-lock-k8s.
#
# Leader election over a Lease requires a live Kubernetes control plane, which
# this script cannot provide. So the automated check is two-part:
#
#   1. Unit tests — the real verification. The client-go fake clientset exercises
#      the full acquire / contend / renew / release / election logic against
#      in-memory Lease objects, without a cluster.
#   2. Boot the example — proves the starter wires into a Go-Spring app. Outside
#      a cluster the example detects no reachable API server, skips the Locker,
#      and self-terminates, so a clean exit means the wiring is sound.
#
# For true in-cluster verification, apply example/deploy/*.yaml (see README):
# run replicas > 1 and confirm exactly one Pod logs "became leader".
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# 1. Unit tests at the module root. -gcflags is required by the assert package's
#    mockey dependency.
echo "== unit tests =="
( cd .. && go test -gcflags="all=-N -l" ./... )

# 2. Boot the example; outside a cluster it self-terminates cleanly.
echo "== example boot =="
go run . &
pid=$!
( sleep 30; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
