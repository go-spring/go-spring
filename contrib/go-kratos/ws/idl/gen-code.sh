#!/usr/bin/env bash
#
# Regenerates the Kratos stubs under idl/helloworld/v1/ from the .proto files.
# Run this after editing any idl/helloworld/v1/*.proto. Requires the kratos CLI
# plus the protoc plugins it drives:
#
#   go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#   go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
#   go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

kratos proto client helloworld/v1/greeter.proto
kratos proto client helloworld/v1/error_reason.proto
