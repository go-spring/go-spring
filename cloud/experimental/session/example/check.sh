#!/usr/bin/env bash
#
# Smoke test for the session example. It brings up the example's gs application,
# which drives the Manager middleware over the in-process Memory store and
# self-asserts lazy allocation, login id rotation, typed attributes, logout
# invalidation, and idle expiry; it exits non-zero on any mismatch. No external
# services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(mktemp)"
go run . >"${out}" 2>&1 &
pid=$!
# The example stops itself once its assertions pass; the watchdog only fires if
# it hangs.
( sleep 60; kill -9 "${pid}" 2>/dev/null ) &
watchdog=$!

rc=0
wait "${pid}" 2>/dev/null || rc=$?
kill "${watchdog}" 2>/dev/null || true
wait "${watchdog}" 2>/dev/null || true

cat "${out}"
# gs.Run returns (exit code 0) even when bean wiring fails at startup, so gate
# on the example's success marker rather than the exit code alone.
if [ "${rc}" -ne 0 ] || ! grep -q "session example ok" "${out}"; then
    echo "smoke test FAILED" >&2
    rm -f "${out}"
    exit 1
fi
rm -f "${out}"
exit 0
