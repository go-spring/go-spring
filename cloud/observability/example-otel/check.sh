#!/usr/bin/env bash
#
# Smoke test for the observability otel example. It runs the self-asserting demo
# — an in-process TracerProvider with a span recorder, and calls through the
# framework's observe-only executor — and asserts the marker the demo prints
# once every step passed, including that each operation's span carries both the
# framework's attributes and the layer's. No external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

out="$(go run .)"
echo "${out}"
grep -q "observability example-otel ok" <<<"${out}"
