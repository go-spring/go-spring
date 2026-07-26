#!/usr/bin/env bash
#
# Smoke test for starter-discovery-k8s.
#
# Full end-to-end discovery (resolving a Deployment's Pods, observing scale
# changes) requires a live Kubernetes cluster, which this script cannot provide.
# So the automated check is two-part:
#
#   1. Unit tests — the real verification. A fake DNS resolver and the client-go
#      fake clientset exercise both modes (resolve, port selection, ready/zone
#      metadata, watch-on-scale) without a cluster.
#   2. Boot the example — proves the starter wires into a Go-Spring app and
#      registers its backend. Outside a cluster the resolve call warns and the
#      app self-terminates, so a clean exit means the wiring is sound.
#
# For true in-cluster verification, apply example/deploy/*.yaml (see README).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# 1. Unit tests at the module root. -gcflags is required by the assert package's
#    mockey dependency.
echo "== unit tests =="
( cd .. && go test -gcflags="all=-N -l" ./... )

# 2. Boot the example; it resolves once, then SIGTERMs itself.
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
