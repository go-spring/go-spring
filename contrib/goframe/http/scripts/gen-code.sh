#!/usr/bin/env bash
#
# The goframe HTTP provider's Hello handler is hand-written in
# provider/handler.go (the g.Meta request type drives route registration) —
# there is no IDL and no code generator involved. Nothing to regenerate.
#
# This script is intentionally a no-op so that every protocol subproject under
# contrib/goframe exposes the same regeneration entry point (compare
# ../../grpc/scripts/gen-code.sh, which does drive protoc). If you add a new
# route you edit provider/{server,handler}.go directly; nothing here changes.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "goframe HTTP has no IDL codegen — nothing to do."
