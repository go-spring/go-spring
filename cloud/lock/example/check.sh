#!/usr/bin/env bash
#
# Smoke test for the lock example. It exercises Acquire/TryAcquire contention,
# the fencing token, Lost() on lease expiry, and an election handover on the
# MemoryLocker, self-asserting every step; it exits non-zero on any mismatch.
# No external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run .)"
echo "${out}"
grep -q "lock example ok" <<<"${out}"
