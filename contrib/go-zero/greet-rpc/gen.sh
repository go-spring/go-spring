#!/usr/bin/env bash
#
# Regenerates the gRPC stubs under pb/ from greet.proto. Run this after editing
# greet.proto. Requires goctl and the protoc plugins:
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

rm -rf pb
mkdir -p pb

tmp_zrpc="$(mktemp -d)"
trap 'rm -rf "$tmp_zrpc"' EXIT

goctl rpc protoc greet.proto \
    --go_out=./pb \
    --go-grpc_out=./pb \
    --zrpc_out="$tmp_zrpc"

# protoc-gen-go honours the go_package option ("greetrpc/pb;pb") and drops the
# stubs under pb/greetrpc/pb/. Flatten them back to pb/.
mv pb/greetrpc/pb/*.go pb/
rm -rf pb/greetrpc
