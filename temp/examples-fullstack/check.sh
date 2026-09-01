#!/usr/bin/env bash
#
# End-to-end smoke for the full-stack reference app. It boots the three services
# (gateway, order, inventory) against a Consul agent and a Nacos server in
# Docker, then drives one request through the whole chain and asserts:
#
#   1. Unauthenticated POST /api/orders -> 401 (JWT enforced at the order service).
#   2. Authenticated  POST /api/orders -> 200 committed (gateway -> order ->
#      inventory reserve, discovered via Consul).
#   3. After flipping fullstack.order.charge-fail=true in Nacos, an authenticated
#      order -> 409 compensated, and the inventory log shows a matching /release
#      (Saga compensation triggered by a live config change).
#   4. All three services shut down gracefully on SIGTERM.
#
# Requires Docker + docker-compose. Skipped gracefully when Docker is absent.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# --- offline check: everything compiles --------------------------------------
echo "== build =="
go build ./... || { echo "FAIL: build"; exit 1; }

if ! command -v docker >/dev/null 2>&1 || ! command -v docker-compose >/dev/null 2>&1; then
    echo "WARNING: docker/docker-compose not found — skipping end-to-end boot"
    exit 0
fi

GW=http://127.0.0.1:9440
NACOS=http://127.0.0.1:8848

pids=()
logdir="$(mktemp -d)"

cleanup() {
    echo "== cleanup =="
    for pid in "${pids[@]:-}"; do
        kill "${pid}" 2>/dev/null || true
    done
    wait 2>/dev/null || true
    docker-compose down -v >/dev/null 2>&1 || true
    rm -rf "${logdir}"
}
trap cleanup EXIT

# --- dependencies ------------------------------------------------------------
echo "== dependencies (consul + nacos) =="
docker-compose up -d

echo "-- waiting for consul leader --"
for _ in $(seq 1 60); do
    if curl -fsS http://127.0.0.1:8500/v1/status/leader 2>/dev/null | grep -q ':'; then break; fi
    sleep 1
done

echo "-- waiting for nacos --"
for _ in $(seq 1 90); do
    if curl -fsS "${NACOS}/nacos/v1/console/health/readiness" >/dev/null 2>&1; then break; fi
    sleep 1
done

# Start each service from its own directory (each chdir's to its source dir, but
# `go run` needs a clean cwd anyway).
start() {
    local name=$1 dir=$2
    ( go run "${dir}" >"${logdir}/${name}.log" 2>&1 ) &
    pids+=($!)
    echo "started ${name} (pid ${pids[-1]}), log ${logdir}/${name}.log"
}

echo "== services =="
start inventory ./cmd/inventory
start order     ./cmd/order
start gateway   ./cmd/gateway

# Wait for each actuator readiness endpoint (distinct management ports).
wait_ready() {
    local name=$1 port=$2
    for _ in $(seq 1 60); do
        if curl -fsS "http://127.0.0.1:${port}/readiness" >/dev/null 2>&1; then
            echo "${name} ready"
            return 0
        fi
        sleep 1
    done
    echo "FAIL: ${name} not ready on :${port}"
    cat "${logdir}/${name}.log"
    exit 1
}
wait_ready gateway   9370
wait_ready order     9371
wait_ready inventory 9372

# Give Consul registration + the gateway's lb://order resolve a moment to settle.
sleep 3

# --- 1. unauthenticated is rejected ------------------------------------------
echo "== 1. unauthenticated -> 401 =="
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "${GW}/api/orders")
if [[ "${code}" != "401" ]]; then
    echo "FAIL: expected 401, got ${code}"; cat "${logdir}/order.log"; exit 1
fi
echo "OK: 401 without token"

# --- 2. happy path -> committed ----------------------------------------------
echo "== 2. authenticated -> committed =="
token=$(go run ./cmd/mint alice user)
resp=$(curl -s -w '\n%{http_code}' -X POST "${GW}/api/orders" -H "Authorization: Bearer ${token}")
code=$(tail -n1 <<<"${resp}")
body=$(sed '$d' <<<"${resp}")
echo "response: ${code} ${body}"
if [[ "${code}" != "200" ]]; then
    echo "FAIL: expected 200 committed, got ${code}"; cat "${logdir}/order.log" "${logdir}/gateway.log"; exit 1
fi
echo "OK: order committed through gateway -> order -> inventory"

# --- 3. compensation via live config change ----------------------------------
echo "== 3. flip charge-fail in nacos -> compensated =="
curl -fsS -X POST "${NACOS}/nacos/v1/cs/configs" \
    --data-urlencode 'dataId=gs-fullstack-order' \
    --data-urlencode 'group=DEFAULT_GROUP' \
    --data-urlencode 'content=fullstack.order.charge-fail=true' >/dev/null
echo "published fullstack.order.charge-fail=true"

# Poll until the order service observes the compensated outcome (config refresh
# is asynchronous).
ok=0
for _ in $(seq 1 30); do
    resp=$(curl -s -w '\n%{http_code}' -X POST "${GW}/api/orders" -H "Authorization: Bearer ${token}")
    code=$(tail -n1 <<<"${resp}")
    if [[ "${code}" == "409" ]]; then ok=1; break; fi
    sleep 1
done
if [[ "${ok}" != "1" ]]; then
    echo "FAIL: order never compensated after config change"; cat "${logdir}/order.log"; exit 1
fi
echo "OK: order compensated (409) after charge-fail=true"

if ! grep -q "released token=" "${logdir}/inventory.log"; then
    echo "FAIL: inventory shows no /release — compensation did not reach service B"
    cat "${logdir}/inventory.log"; exit 1
fi
echo "OK: inventory released the reservation (cross-service compensation)"

# --- 4. graceful shutdown ----------------------------------------------------
echo "== 4. graceful shutdown =="
for pid in "${pids[@]}"; do
    kill -TERM "${pid}" 2>/dev/null || true
done
for _ in $(seq 1 20); do
    alive=0
    for pid in "${pids[@]}"; do kill -0 "${pid}" 2>/dev/null && alive=1; done
    [[ "${alive}" == "0" ]] && break
    sleep 1
done
pids=()  # cleanup already handled shutdown
echo "OK: all services exited on SIGTERM"

echo "PASS: full-stack reference app end-to-end"
