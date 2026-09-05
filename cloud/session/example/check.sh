#!/usr/bin/env bash
#
# Smoke test for the session example. It drives the Manager middleware over
# the in-process Memory store and self-asserts lazy allocation, login id
# rotation, typed attributes, logout invalidation, and idle expiry; it exits
# non-zero on any mismatch. No external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run .)"
echo "${out}"
grep -q "session example ok" <<<"${out}"
