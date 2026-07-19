#!/usr/bin/env bash
#
# Smoke test for starter-oauth2-server. The authorization server signs with a
# shared HMAC secret, so there is no external identity provider: the example
# drives the full authorization_code + PKCE flow in-process (authorize -> code
# -> token), has the resource server validate the issued token, then exercises
# refresh_token, client_credentials, a PKCE mismatch, /jwks, and the CORS/CSRF
# filter chain — exiting non-zero on any failure.
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
