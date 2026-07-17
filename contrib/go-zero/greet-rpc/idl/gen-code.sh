#!/usr/bin/env bash
#
# Regenerates the gRPC stubs under idl/ from idl/greet.proto. Run this after
# editing idl/greet.proto. Requires goctl and the protoc plugins:
#
#   go install github.com/zeromicro/go-zero/tools/goctl@latest
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#
# We use `goctl rpc protoc` because this is the go-zero example. --zrpc_out
# points at a throwaway tmp directory: goctl insists on scaffolding an
# internal/ + go.mod project tree there, but we only want the .pb.go stubs.
# The Go-Spring provider/consumer stay hand-written and unaffected.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# Remove previously generated stubs but keep the hand-maintained greet.proto,
# which lives alongside them under idl/ (mirroring dubbo-go/triple).
rm -f greet.pb.go greet_grpc.pb.go

tmp_zrpc="$(mktemp -d)"
trap 'rm -rf "$tmp_zrpc"' EXIT

goctl rpc protoc greet.proto \
    --go_out=. \
    --go-grpc_out=. \
    --zrpc_out="$tmp_zrpc"

# protoc-gen-go honours the go_package option ("greetrpc/idl;greet") and drops
# the stubs under greetrpc/idl/. Flatten them back to idl/.
mv greetrpc/idl/*.go .
rm -rf greetrpc
