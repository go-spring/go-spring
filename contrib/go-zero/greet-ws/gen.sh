#!/usr/bin/env bash
#
# go-zero's .api DSL (the one greet-api regenerates from) can only describe
# request/response HTTP endpoints; WebSocket has no counterpart in it, so
# `goctl api go` cannot scaffold either the WS route or the frame types.
# Everything under internal/ here is therefore hand-written:
#
#   internal/types/types.go        — WS frame payloads (JSON)
#   internal/handler/wshandler.go  — upgrade + read/write loop
#   internal/svc/servicecontext.go — injected Logic surface
#   internal/logic/greetlogic.go   — GreetLogic IoC bean
#
# This script is intentionally a no-op so every protocol subproject under
# contrib/go-zero exposes the same regeneration entry point (compare
# ../greet-api/gen.sh, which does drive goctl, and ../greet-rpc/gen.sh, which
# drives protoc). If you add a new WS route or field, edit the files above by
# hand.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "WebSocket has no .api / .proto IDL in go-zero — nothing to regenerate."
