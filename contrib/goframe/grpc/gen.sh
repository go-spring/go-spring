#!/usr/bin/env bash
#
# Regenerates the gRPC stubs under pbgen/echo/ from the protobuf IDL. Run this
# after editing idl/echo.proto. Requires protoc plus the Go plugins:
#
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#
# The go_package option in echo.proto pins the output package to
# go-spring.org/goframe/grpc/pbgen/echo, so protoc writes to pbgen/echo/ under
# the module root regardless of --proto_path layout.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

protoc \
    --proto_path=idl \
    --go_out=. \
    --go_opt=module=go-spring.org/goframe/grpc \
    --go-grpc_out=. \
    --go-grpc_opt=module=go-spring.org/goframe/grpc \
    idl/echo.proto
