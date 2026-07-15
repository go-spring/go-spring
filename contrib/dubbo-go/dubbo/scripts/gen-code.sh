#!/usr/bin/env bash
#
# The classic Dubbo protocol (TCP + Hessian2) has no protobuf/thrift IDL and
# no code generator in dubbo-go v3: services are plain Go structs whose
# exported method signatures are reflected over at registration time and
# marshalled with Hessian2 on the wire. The "IDL" is proto/greet.go — edit
# that file by hand.
#
# This script is intentionally a no-op so that every protocol subproject
# under contrib/dubbo-go exposes the same regeneration entry point (compare
# ../../triple/scripts/gen-code.sh, which does drive protoc). If your service uses non-
# primitive types you may need to add hessian.RegisterPOJO calls; add them to
# proto/greet.go, not here.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "classic Dubbo/Hessian2 has no IDL codegen — nothing to do."
