#!/usr/bin/env bash
#
# Smoke test for starter-registry-etcd. Two parts:
#
#   1. Unit tests — the offline verification (id/key derivation, TTL rounding).
#      The stdlib/discovery Registrar seam itself is covered in that package.
#   2. End-to-end boot — starts an etcd node in Docker, boots the example (which
#      registers, reads the keys back, prints the instance, then SIGTERMs itself
#      so the deregister-on-shutdown path runs), and checks the example saw its
#      own registration. Skipped gracefully without Docker.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# 1. Unit tests at the module root. -gcflags is required by the assert package's
#    mockey dependency.
echo "== unit tests =="
( cd .. && go test -gcflags="all=-N -l" ./... )

# 2. End-to-end boot against a real etcd node.
if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping example boot"
    exit 0
fi

name="registry-etcd-smoke"
trap 'docker rm -f "${name}" >/dev/null 2>&1 || true' EXIT
docker rm -f "${name}" >/dev/null 2>&1 || true
docker run -d --rm --name "${name}" -p 2379:2379 \
    quay.io/coreos/etcd:v3.5.13 \
    /usr/local/bin/etcd \
    --name node1 \
    --advertise-client-urls http://0.0.0.0:2379 \
    --listen-client-urls http://0.0.0.0:2379 >/dev/null

# Wait for etcd to answer health (up to 60s).
for _ in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:2379/health" 2>/dev/null | grep -q "true"; then
        break
    fi
    sleep 1
done

echo "== example boot =="
out="$(mktemp)"
MOCKEY_CHECK_GCFLAGS=false go run . >"${out}" 2>&1 &
pid=$!
( sleep 40; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

cat "${out}"
if ! grep -q "registered key=" "${out}"; then
    echo "FAIL: example did not observe its own registration in etcd"
    rm -f "${out}"
    exit 1
fi
rm -f "${out}"
exit "${rc}"
