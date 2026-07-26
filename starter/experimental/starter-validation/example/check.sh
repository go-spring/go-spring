#!/usr/bin/env bash
#
# Smoke test for starter-validation. The example is purely in-process — it
# exercises config-binding and web-path validation + i18n without any server or
# external service. Exits non-zero on failure.
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