#!/usr/bin/env bash
#
# Smoke test for starter-registry-nacos. Two parts:
#
#   1. Unit tests — the offline verification (addr fail-fast). The
#      stdlib/discovery Registrar seam itself is covered in that package.
#   2. End-to-end boot — starts a Nacos standalone server via docker compose,
#      boots the example (which registers, reads the naming service back, prints
#      the instance, then SIGTERMs itself so the deregister-on-shutdown path
#      runs), and checks the example saw its own registration. Skipped gracefully
#      without docker.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# 1. Unit tests at the module root. -gcflags is required by the assert package's
#    mockey dependency.
echo "== unit tests =="
( cd .. && go test -gcflags="all=-N -l" ./... )

# 2. End-to-end boot against a real Nacos server.
if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping example boot"
    exit 0
fi

# Prefer the compose v2 plugin, fall back to the standalone docker-compose.
if docker compose version >/dev/null 2>&1; then
    compose() { docker compose "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose "$@"; }
else
    echo "WARNING: docker compose not available — skipping example boot"
    exit 0
fi

trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d

# Wait for Nacos to report readiness (up to 120s; first boot is slow).
for _ in $(seq 1 120); do
    if curl -fsS "http://127.0.0.1:8848/nacos/v1/console/health/readiness" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

echo "== example boot =="
out="$(mktemp)"
MOCKEY_CHECK_GCFLAGS=false go run . >"${out}" 2>&1 &
pid=$!
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

cat "${out}"
if ! grep -q "registered addr=" "${out}"; then
    echo "FAIL: example did not observe its own registration in Nacos"
    rm -f "${out}"
    exit 1
fi
rm -f "${out}"
exit "${rc}"