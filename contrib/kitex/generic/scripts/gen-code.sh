#!/usr/bin/env bash
#
# Regenerates the Kitex stubs under kitex_gen/ from the Thrift IDL. Run this
# after editing idl/echo.thrift. Requires the kitex + thriftgo generators:
#
#   go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
#   go install github.com/cloudwego/thriftgo@latest
#
# NOTE: The generated code here is used ONLY by the provider. The consumer
# never imports kitex_gen — it parses idl/echo.thrift at runtime via
# generic.NewThriftFileProvider and speaks JSON <-> Thrift over the wire (see
# consumer/main.go). Regenerating this directory does not affect the consumer.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

kitex -module go-spring.org/kitex/generic -service echo idl/echo.thrift
