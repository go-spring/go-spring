#!/usr/bin/env bash
#
# Regenerates the gRPC stubs under proto/ from proto/greet.proto. Run this after
# editing proto/greet.proto. Requires goctl and the protoc plugins:
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
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Remove previously generated stubs but keep the hand-maintained greet.proto,
# which now lives alongside them under proto/ (mirroring dubbo-go/triple).
rm -f proto/greet.pb.go proto/greet_grpc.pb.go

tmp_zrpc="$(mktemp -d)"
trap 'rm -rf "$tmp_zrpc"' EXIT

goctl rpc protoc proto/greet.proto \
    --go_out=./proto \
    --go-grpc_out=./proto \
    --zrpc_out="$tmp_zrpc"

# protoc-gen-go honours the go_package option ("greetrpc/proto;greet") and drops
# the stubs under proto/greetrpc/proto/. Flatten them back to proto/.
mv proto/greetrpc/proto/*.go proto/
rm -rf proto/greetrpc
