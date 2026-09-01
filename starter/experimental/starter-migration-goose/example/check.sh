#!/usr/bin/env bash
#
# Smoke test for the starter-migration-goose example. Runs the app, which
# applies the goose SQL migrations to an in-memory sqlite database at startup
# and self-asserts startup apply and idempotency. Exits non-zero on failure.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run . 2>&1)"
echo "${out}"
grep -q "startup apply OK" <<<"${out}"
grep -q "idempotency OK" <<<"${out}"
