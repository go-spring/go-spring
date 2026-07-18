#!/usr/bin/env bash
#
# Regenerates the kratos-ws payload stubs under pb/ from service.proto.
#
# kratos does not have a first-party WebSocket transport, so this IDL only
# describes the on-the-wire envelope + payload types. No rpc service is
# declared; the connection loop lives in kratoswssvr and picks a payload type
# based on the Envelope.type field.
#
# Requires (versions match the committed generated code):
#   protoc         v5.27.0
#   protoc-gen-go  v1.34.2   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#
# Run this after editing service.proto.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

mkdir -p pb
protoc --proto_path=. \
  --go_out=paths=source_relative:./pb \
  service.proto
