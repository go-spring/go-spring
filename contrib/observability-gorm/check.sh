#!/usr/bin/env bash
#
# Smoke test for the unified-observability GORM bridge. Proves the whole promise
# of starter-otel in one run: importing starter-otel + starter-gorm-mysql and
# configuring ${spring.observability} once is enough to get GORM query spans and
# connection-pool metrics at the collector — no per-component instrumentation.
#
# Flow: bring up mysql + an OTel collector (debug exporter -> stdout), run the
# demo app on the host (it does a few DB statements at startup then returns),
# SIGTERM it so graceful shutdown flushes buffered spans/metrics, then grep the
# collector logs for db spans (db.system.name=mysql, db.query.text) and pool
# metrics (go.sql.connections_*).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
ROOT="$(pwd)"

echo "=== observability-gorm smoke test ==="

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

# Prefer the compose v2 plugin, fall back to the standalone docker-compose.
if docker compose version >/dev/null 2>&1; then
    compose() { (cd "${ROOT}" && docker compose "$@"); }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { (cd "${ROOT}" && docker-compose "$@"); }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

app_pid=""
app_bin=""
cleanup() {
    [ -n "${app_pid}" ] && kill "${app_pid}" 2>/dev/null || true
    [ -n "${app_bin}" ] && rm -rf "$(dirname "${app_bin}")" 2>/dev/null || true
    compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

compose up -d

# wait_tcp HOST PORT [TRIES] — returns 0 once the port accepts a connection.
wait_tcp() {
    local host="$1" port="$2" tries="${3:-30}"
    for _ in $(seq 1 "${tries}"); do
        if (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; then
            exec 3>&- 3<&- 2>/dev/null || true
            return 0
        fi
        sleep 1
    done
    return 1
}

# The collector's OTLP ingress must be up before the app exports.
wait_tcp 127.0.0.1 4317 || { echo "OTLP ingress (:4317) not ready"; exit 1; }

# MySQL must accept connections AND report healthy (initialized) before the app
# opens its pool; a bare TCP check races the container's init phase.
echo "waiting for mysql to become healthy..."
for _ in $(seq 1 60); do
    status="$(compose ps mysql --format '{{.Health}}' 2>/dev/null || true)"
    [ "${status}" = "healthy" ] && break
    sleep 2
done
if [ "${status:-}" != "healthy" ]; then
    echo "FAIL: mysql did not become healthy"
    compose ps
    exit 1
fi
echo "OK: mysql healthy, collector listening"

# Build the app to a temp binary and run it directly. It does its DB work at
# startup, prints a confirmation line, then blocks on signal (gs.Run).
app_bin="$(mktemp -d)/app"
go build -o "${app_bin}" .
app_out="$(mktemp)"
"${app_bin}" >"${app_out}" 2>&1 &
app_pid=$!

# Assertion 1: the Runner actually reached the DB and read its rows back.
for _ in $(seq 1 30); do
    grep -q "inserted and read back" "${app_out}" && break
    kill -0 "${app_pid}" 2>/dev/null || { echo "FAIL: app exited early"; cat "${app_out}"; exit 1; }
    sleep 1
done
if ! grep -q "inserted and read back 5 widgets" "${app_out}"; then
    echo "FAIL: app did not complete its DB work"
    cat "${app_out}"
    exit 1
fi
echo "OK: app ran DB statements against mysql"

# Signal graceful shutdown so the batch span processor / periodic reader flush
# their buffers to the collector, then wait for the process to exit.
kill "${app_pid}" 2>/dev/null || true
wait "${app_pid}" 2>/dev/null || true
app_pid=""
rm -f "${app_out}"

# Give the collector a moment to print the flushed batch to stdout.
sleep 3
logs="$(compose logs otel-collector 2>/dev/null || true)"

# Assertion 2: a GORM CLIENT span reached the collector with the db semconv
# attributes. The otel plugin tags spans with db.system.name and db.query.text.
if ! grep -q "db.system.name" <<<"${logs}"; then
    echo "FAIL: collector did not receive db.system.name attribute"
    echo "${logs}" | tail -40
    exit 1
fi
if ! grep -qi "mysql" <<<"${logs}"; then
    echo "FAIL: collector received no mysql-tagged spans"
    exit 1
fi
echo "OK: collector received GORM db spans (db.system.name / mysql)"

# Assertion 3: connection-pool metrics landed too (proves the metrics pillar of
# the bridge, not just tracing).
if ! grep -q "go.sql.connections" <<<"${logs}"; then
    echo "FAIL: collector did not receive go.sql.connections_* pool metrics"
    echo "${logs}" | grep -i "metric\|gauge\|sum" | tail -20
    exit 1
fi
echo "OK: collector received go.sql.connections_* pool metrics"

echo "smoke test passed"
