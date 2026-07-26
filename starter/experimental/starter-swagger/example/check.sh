#!/usr/bin/env bash
#
# Smoke test for starter-swagger. The starter serves a Swagger UI shell plus a
# generated OpenAPI document over HTTP with no external service, so this just
# runs the example, which self-asserts (fetches the UI + spec) and exits
# non-zero on failure.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run . &
pid=$!
( sleep 30; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!
rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true
exit "${rc}"
