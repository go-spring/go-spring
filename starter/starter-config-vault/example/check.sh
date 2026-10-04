#!/usr/bin/env bash
#
# Smoke test for starter-config-vault. Brings up a dev-mode Vault via docker
# compose, runs the example (which self-asserts and exits non-zero on failure),
# then tears the container down. Skipped gracefully when docker is unavailable.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

# Prefer the compose v2 plugin, fall back to the standalone docker-compose.
if docker compose version >/dev/null 2>&1; then
    compose() { docker compose "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose "$@"; }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d

# Wait for the Vault health endpoint (a raw TCP probe accepts before Vault can
# serve). Dev-mode Vault boots quickly; allow up to 30s for the image to start.
echo "== waiting for the service to be ready =="
for _ in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:8200/v1/sys/health" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

export VAULT_TOKEN=root
# AES key for decrypting the demo.password=ENC(aes:...) property (see
# spring/conf/decrypt/aes). Base64 of the 16-byte key "1234567890123456";
# a real deployment supplies the key out of band (mounted Secret, Vault
# Agent sink) — it is inlined here only to keep the smoke test self-contained.
export GS_CONFIG_DECRYPT_AES_KEY="MTIzNDU2Nzg5MDEyMzQ1Ng=="
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
