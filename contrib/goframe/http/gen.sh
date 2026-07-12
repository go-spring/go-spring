#!/usr/bin/env bash
#
# Regenerates the goframe controller stubs under internal/controller/ from the
# api/*/v*/ request/response types. Run this after editing anything under api/.
# Requires the gf CLI:
#
#   go install github.com/gogf/gf/cmd/gf/v2@latest
#
# This is what `make ctrl` invokes; the hack/*.mk files carry the same command.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

gf gen ctrl
