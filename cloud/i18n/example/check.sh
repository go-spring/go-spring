#!/usr/bin/env bash
#
# Smoke test for the i18n example. It resolves messages per locale, checks the
# fallback order and the missing-key sentinel, and exercises the validation
# pairing, self-asserting every expectation. No external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run .)"
echo "${out}"
grep -q "i18n example ok" <<<"${out}"
