#!/usr/bin/env bash
# versions.sh - Go-Spring BOM governance entry (MAINTAINER-ONLY).
#
# This is NOT part of the gs user toolkit. It wraps the bomtool maintainer
# binary to check/diff/align the workspace's third-party dependency versions
# against versions.yaml at the repo root. Kept here so version governance lives
# in scripts/ and never leaks into the commands users see via `gs --help`.
#
# Usage:
#   ./scripts/versions.sh check            report drift; non-zero exit on drift
#   ./scripts/versions.sh diff             per-dependency deviation detail
#   ./scripts/versions.sh apply <module>   align one module's go.mod to baseline
#
# See gs/gs/VERSIONS.md for the governance model.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT/gs/gs"
exec go run ./cmd/bomtool "$@"
