#!/usr/bin/env bash
#
# Regenerates greet.pb.go and greet.trpc.go under idl/ from greet.proto using
# the tRPC command-line tool. Run this after editing greet.proto. Requires:
#
#   go install trpc.group/trpc-go/trpc-cmdline/trpc@latest
#
# --rpconly emits only the RPC stubs (messages + service desc + client proxy),
# not a full project scaffold. The trailing "mockgen" warning about go.mod is
# harmless — the .pb.go/.trpc.go files are generated before it. This example is
# one module, so no per-idl go.mod is kept.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

trpc create --rpconly --protofile greet.proto -o .
rm -f go.mod
