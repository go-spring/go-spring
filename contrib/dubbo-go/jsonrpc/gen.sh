#!/usr/bin/env bash
#
# JSON-RPC in dubbo-go v3 has no protobuf/thrift IDL and no code generator:
# the service surface is a plain Go struct whose exported method signatures
# are reflected over at registration time and JSON-serialized on the wire.
# The "IDL" is proto/greet.go — edit that file by hand.
#
# This script is intentionally a no-op so that every protocol subproject
# under contrib/dubbo-go exposes the same regeneration entry point (compare
# ../triple/gen.sh, which does drive protoc).
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "JSON-RPC has no IDL codegen — nothing to do."
