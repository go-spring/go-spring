#!/usr/bin/env bash
#
# Smoke test for starter-rocketmq. Brings up a local RocketMQ (name server +
# broker) via docker compose, runs the example (which self-asserts and exits
# non-zero on failure), then tears the containers down. Skipped gracefully
# when docker is unavailable.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

if ! command -v docker >/dev/null 2>&1; then
    echo "WARNING: docker not found — skipping"
    exit 0
fi

# Prefer the compose v2 plugin, fall back to the standalone docker-compose.
if docker compose version >/dev/null 2>&1; then
    compose() { docker compose -p gs-rocketmq-example "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
    compose() { docker-compose -p gs-rocketmq-example "$@"; }
else
    echo "WARNING: docker compose not available — skipping"
    exit 0
fi

trap 'compose down -v >/dev/null 2>&1 || true' EXIT
compose up -d

# Wait for the name server port (up to 90s; the JVM boot is slow).
namesrv_ready=false
echo "== waiting for the service to be ready =="
for _ in $(seq 1 90); do
    if (exec 3<>/dev/tcp/127.0.0.1/9876) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        namesrv_ready=true
        break
    fi
    sleep 1
done
if [ "${namesrv_ready}" != true ]; then
    echo "ERROR: rocketmq name server did not open 9876 in 90s"
    exit 1
fi

# Wait for the broker JVM to boot (up to 90s).
for _ in $(seq 1 90); do
    if (exec 3<>/dev/tcp/127.0.0.1/10911) 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        break
    fi
    sleep 1
done

# Create the topic from inside the broker container. mqadmin exits 0 even on
# failure, so gate on the "success" line in its output. Running the command
# inside the broker matters: broker.conf advertises brokerIP1=127.0.0.1 for
# host-side clients, so a side-car container resolving the broker through the
# name server would dial itself.
topic_created=false
for _ in $(seq 1 60); do
    if docker exec rmqbroker sh mqadmin updateTopic -n namesrv:9876 -c DefaultCluster -t hello 2>/dev/null | grep -q success; then
        topic_created=true
        break
    fi
    sleep 2
done
if [ "${topic_created}" != true ]; then
    echo "ERROR: could not create topic hello in 120s"
    exit 1
fi

# Gate on the topic being visible through the name server — the exact view
# the client's Subscribe will query. The route reaches the name server on the
# broker's next registration heartbeat (up to 60s).
topics_ready=false
for _ in $(seq 1 30); do
    if docker exec rmqbroker sh mqadmin topicList -n namesrv:9876 2>/dev/null | grep -qw hello; then
        topics_ready=true
        break
    fi
    sleep 2
done
if [ "${topics_ready}" != true ]; then
    echo "ERROR: topic hello not visible on the name server in 60s"
    exit 1
fi

echo "== example boot =="
go run . > smoke.out 2>&1 &
pid=$!
( sleep 90; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!

rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

# gs.Run returns (exit code 0) even when bean wiring fails at startup, so gate
# on the example's success marker rather than the exit code alone.
if [ "${rc}" -ne 0 ] || ! grep -q "Response from server:" smoke.out; then
    echo "== FAILED ==" >&2
    cat smoke.out >&2 || true
    rm -f smoke.out
    exit 1
fi
echo "== OK =="
rm -f smoke.out
exit 0
