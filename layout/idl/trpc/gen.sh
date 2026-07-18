#!/usr/bin/env bash
#
# Regenerates the tRPC stubs under pb/ from service.proto.
#
# Requires (versions match the committed generated code):
#   trpc-cmdline    latest    go install trpc.group/trpc-go/trpc-cmdline/trpc@latest
#
# --rpconly emits only the RPC stubs (messages + service desc + client proxy),
# not a full project scaffold. Run this after editing service.proto.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

mkdir -p pb
trpc create --rpconly --protofile service.proto -o ./pb
rm -f pb/go.mod
