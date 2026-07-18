#!/usr/bin/env bash
#
# Regenerates the Apache Thrift stubs under gen/ from service.thrift.
#
# Requires (versions match the committed generated code):
#   thriftgo v0.4.5   go install github.com/cloudwego/thriftgo@latest
#
# Run this after editing service.thrift.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

thriftgo -g go -o . service.thrift
