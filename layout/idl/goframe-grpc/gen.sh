#!/usr/bin/env bash
#
# Regenerates the goframe gRPC stubs under pb/ from service.proto.
#
# Requires (versions match the committed generated code):
#   protoc              v5.27.0
#   protoc-gen-go       v1.34.2   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   protoc-gen-go-grpc  v1.5.1    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#
# Run this after editing service.proto.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

mkdir -p pb
protoc --proto_path=. \
  --go_out=paths=source_relative:./pb \
  --go-grpc_out=paths=source_relative:./pb \
  service.proto
