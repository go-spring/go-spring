#!/usr/bin/env bash
#
# Regenerates the stdlib HTTP stubs under proto/ from the *.idl files
# (common.idl, ping.idl, order.idl, user.idl). meta.json carries the service
# name/version consumed by the generator.
#
# Requires:
#   gs-http-gen   go install go-spring.org/gs-http-gen@latest
#
# Run this after editing any *.idl file.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

gs-http-gen --server --output proto
