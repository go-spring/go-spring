#!/usr/bin/env bash
#
# Smoke test for the validation example. It maps go-playground/validator
# failures onto validation.ValidationErrors and renders them in two locales,
# self-asserting both renderings; it exits non-zero on any mismatch. No
# external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run .)"
echo "${out}"
grep -q "validation example ok" <<<"${out}"
