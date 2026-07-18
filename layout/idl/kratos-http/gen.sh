#!/usr/bin/env bash
#
# Regenerates the kratos-HTTP stubs under pb/ from service.proto.
#
# Requires (versions match the committed generated code):
#   protoc                v5.27.0
#   protoc-gen-go         v1.34.2   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   protoc-gen-go-http    v2.7.3    go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
#
# Run this after editing service.proto.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

mkdir -p pb
protoc --proto_path=. \
  --go_out=paths=source_relative:./pb \
  --go-http_out=paths=source_relative:./pb \
  service.proto
