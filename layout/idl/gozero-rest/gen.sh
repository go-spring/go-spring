#!/usr/bin/env bash
#
# Regenerates the go-zero REST stubs (handler/, types/) from service.api.
#
# Requires (versions match the committed generated code):
#   goctl v1.10.1   go install github.com/zeromicro/go-zero/tools/goctl@latest
#
# Run this after editing service.api.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

goctl api go --api service.api --dir . --style go_zero
