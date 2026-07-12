#!/usr/bin/env bash
#
# Regenerates the Kitex stubs under kitex_gen/ from the protobuf IDL. Run this
# after editing idl/echo.proto. Requires the kitex generator:
#
#   go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
#
# The generated code speaks both protobuf transports (KitexProtobuf and gRPC);
# the transport is chosen per client at call time, not at generation time.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

kitex -module go-spring.org/kitex/protobuf -service echo idl/echo.proto
