#!/usr/bin/env bash
#
# Smoke test for starter-registry-zookeeper. Two parts:
#
#   1. Unit tests — the offline verification (id/path derivation). The
#      stdlib/discovery Registrar seam itself is covered in that package.
#   2. End-to-end boot — starts a ZooKeeper node in Docker, boots the example
#      (which registers, lists the znodes back, prints the instance, then
#      SIGTERMs itself so the deregister-on-shutdown path runs), and checks the
#      example saw its own registration. Skipped gracefully without Docker.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# 1. Unit tests at the module root. -gcflags is required by the assert package's
#    mockey dependency.
echo "== unit tests =="
( cd .. && go test -gcflags="all=-N -l" ./... )

# 2. End-to-end boot against a real ZooKeeper node.
if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping example boot"
    exit 0
fi

name="registry-zookeeper-smoke"
trap 'docker rm -f "${name}" >/dev/null 2>&1 || true' EXIT
docker rm -f "${name}" >/dev/null 2>&1 || true
docker run -d --rm --name "${name}" -p 2181:2181 zookeeper:3.9 >/dev/null

# Wait for ZooKeeper to answer the "ruok" four-letter command with "imok" (up to 60s).
for _ in $(seq 1 60); do
    if [ "$( (echo ruok; sleep 1) | nc 127.0.0.1 2181 2>/dev/null)" = "imok" ]; then
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
if ! grep -q "registered node=" "${out}"; then
    echo "FAIL: example did not observe its own registration in ZooKeeper"
    rm -f "${out}"
    exit 1
fi
rm -f "${out}"
exit "${rc}"
