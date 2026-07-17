#!/usr/bin/env bash
#
# REST in dubbo-go v3 has no protobuf/thrift IDL and no code generator: the
# service surface is a plain Go struct plus a hand-written RestServiceConfig
# map (see provider/handler.go and consumer/main.go) that pins every method
# to a (verb, path, param-source) tuple. The "IDL" is greet.go — edit
# that file by hand.
#
# This script is intentionally a no-op so that every protocol subproject
# under contrib/dubbo-go exposes the same regeneration entry point (compare
# ../../triple/idl/gen-code.sh, which does drive protoc).
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "REST has no IDL codegen — nothing to do."
