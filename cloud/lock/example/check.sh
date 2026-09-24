#!/usr/bin/env bash
#
# Smoke test for the lock example. It runs the starter-shaped demo app — the
# locker arrives via "spring.lock.instances.memory.demo" configuration, business code
# injects lock.Locker — and asserts every demo step completed: the critical
# section, the contended skip, and the election handover. The backend is the
# in-process MemoryLocker via starter-lock-memory; no external services.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run .)"
echo "${out}"
grep -q "demo: critical section done" <<<"${out}"
grep -q "demo: contended TryAcquire correctly skipped" <<<"${out}"
grep -q "demo: leadership handed over" <<<"${out}"
