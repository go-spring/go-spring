#!/usr/bin/env bash
#
# Regenerates greet.pb.go and greet.triple.go under proto/ from the protobuf
# IDL. Run this after editing proto/greet.proto. Requires:
#
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install github.com/dubbogo/protoc-gen-go-triple/v3@latest
#
# The generated Triple stubs speak the Triple protocol (protobuf-over-HTTP/2,
# gRPC-wire-compatible); the transport is fixed at generation time.
#
# NOTE: on a go1.26 toolchain whose runtime.Version() carries an experiment
# suffix (e.g. go1.26.1-X:jsonv2), protoc-gen-go-triple v3.0.3 panics parsing
# the version. Rebuild it from source with the version string truncated to its
# numeric part.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

protoc --proto_path=proto \
  --go_out=paths=source_relative:./proto \
  --go-triple_out=paths=source_relative:./proto \
  proto/greet.proto
