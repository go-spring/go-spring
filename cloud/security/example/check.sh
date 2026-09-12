#!/usr/bin/env bash
#
# Smoke test for the security example. It drives a hand-written net/http shell
# over a fixed token table and self-asserts the TokenValidator seam, context
# propagation, the route gate, the method-level Require decorator, and the
# shared parsing/CSRF helpers; it exits non-zero on any mismatch. No external
# services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run .)"
echo "${out}"
grep -q "security example ok" <<<"${out}"
