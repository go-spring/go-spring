#!/usr/bin/env bash
#
# WebSocket in goframe rides on a normal *ghttp.Server, and the provider's
# /echo handler is a hand-written r.WebSocket() upgrade loop — there is no
# IDL and no code generator involved. Nothing to regenerate.
#
# This script is intentionally a no-op so that every protocol subproject
# under contrib/goframe exposes the same regeneration entry point (compare
# ../../grpc/scripts/gen-code.sh, which does drive protoc, and ../../http/scripts/gen-code.sh, which runs
# `gf gen ctrl`). If you add a new upgrade route to internal/server/server.go
# you edit that file directly; nothing here needs to change.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "goframe WebSocket has no IDL codegen — nothing to do."
