#!/usr/bin/env bash
#
# Regenerates the Kitex stubs under echo/ from the Thrift IDL. Run this
# after editing echo.thrift. Requires the kitex + thriftgo generators:
#
#   go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
#   go install github.com/cloudwego/thriftgo@latest
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

kitex -module go-spring.org/kitex/thrift -gen-path . -service echo echo.thrift
