#!/usr/bin/env bash
#
# Regenerates the goframe HTTP stubs under pb/ from service.proto.
#
# Requires (versions match the committed generated code):
#   protoc              v5.27.0
#   protoc-gen-go       v1.34.2   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   protoc-gen-go-http  v0.1.0    (scaffold; a real project supplies its own HTTP codegen)
#
# Run this after editing service.proto.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

mkdir -p pb
protoc --proto_path=. \
  --go_out=paths=source_relative:./pb \
  --go-http_out=paths=source_relative:./pb \
  service.proto
