#!/usr/bin/env bash
#
# Regenerates the goframe TCP payload stubs under pb/ from payload.proto.
#
# Requires (versions match the committed generated code):
#   protoc         v5.27.0
#   protoc-gen-go  v1.34.2   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#
# Run this after editing payload.proto.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

mkdir -p pb
protoc --proto_path=. \
  --go_out=paths=source_relative:./pb \
  payload.proto
