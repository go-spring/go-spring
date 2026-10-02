#!/usr/bin/env bash
#
# Smoke test for the cache otel example. It runs the self-asserting demo — an
# in-process TracerProvider with a span recorder, a cache built over the
# package's Memory backend, and a layer of the application's own below
# cache.New — and asserts the marker the demo prints once every step passed,
# including that each operation's span carries both the framework's attributes
# and the layer's. No external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run .)"
echo "${out}"
grep -q "cache example-otel ok" <<<"${out}"
