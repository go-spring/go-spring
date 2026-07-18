#!/usr/bin/env bash
#
# Regenerates the Kitex stubs under kitex_gen/ from service.proto for the
# protobuf/grpc target.
#
# Requires (versions match the committed generated code):
#   kitex v0.16.3   go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
#   protoc          https://github.com/protocolbuffers/protobuf/releases
#
# Run this after editing service.proto. -module must match the project module.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

kitex -module GS_PROJECT_MODULE -type protobuf service.proto
