#!/usr/bin/env bash
#
# Smoke test for the loadtest example. It drives an open-loop ramp and a
# closed-loop run against an in-process HTTP server, self-asserts both
# verdicts (QPS floor, p99 ceiling, error classification), and exits non-zero
# on any failure. No external services are required.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

go run .
