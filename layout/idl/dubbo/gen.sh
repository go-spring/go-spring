#!/usr/bin/env bash
#
# Regenerates the Dubbo (triple) stubs under triple/ from service.proto.
#
# Requires (versions match the committed generated code):
#   protoc              v5.27.0
#   protoc-gen-go       v1.34.2   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   protoc-gen-triple   v3.0.0    go install dubbo.apache.org/dubbo-go/v3/protocol/triple/triple_protocol/cmd/protoc-gen-triple@latest
#
# Run this after editing service.proto.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

mkdir -p triple
protoc --proto_path=. \
  --go_out=paths=source_relative:./triple \
  --go-triple_out=paths=source_relative:./triple \
  service.proto
