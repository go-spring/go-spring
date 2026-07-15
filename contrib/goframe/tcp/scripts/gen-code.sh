#!/usr/bin/env bash
#
# Raw TCP in goframe (gtcp) has no IDL and no code generator: the wire
# protocol is whatever bytes the handler in internal/server/server.go reads
# and writes. Nothing to regenerate.
#
# This script is intentionally a no-op so that every protocol subproject
# under contrib/goframe exposes the same regeneration entry point (compare
# ../../grpc/scripts/gen-code.sh, which does drive protoc, and ../../http/scripts/gen-code.sh, which runs
# `gf gen ctrl`). If you change the wire format, edit the handler directly;
# nothing here needs to change.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "goframe raw-TCP has no IDL codegen — nothing to do."
