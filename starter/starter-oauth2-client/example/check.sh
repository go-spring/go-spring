#!/usr/bin/env bash
#
# Smoke test for starter-oauth2-client. The example spins up an in-process
# OAuth2 token endpoint and a protected resource server, then uses the
# starter-provided *http.Client to call the resource with an auto-fetched
# bearer token. The example self-asserts and exits non-zero on failure, so no
# external services (or docker) are required.
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
