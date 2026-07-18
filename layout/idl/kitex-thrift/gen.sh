#!/usr/bin/env bash
#
# Regenerates the Kitex stubs under kitex_gen/ from service.thrift.
#
# Requires (versions match the committed generated code):
#   kitex    v0.16.3   go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
#   thriftgo v0.4.5    go install github.com/cloudwego/thriftgo@latest
#
# Run this after editing service.thrift. -module must match the project module.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

kitex -module GS_PROJECT_MODULE -type thrift service.thrift
