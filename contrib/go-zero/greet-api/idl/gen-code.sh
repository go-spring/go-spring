#!/usr/bin/env bash
#
# Regenerates the goctl-owned files (types + routes) from greet.api. Run this
# after editing greet.api. Requires goctl:
#
#   go install github.com/zeromicro/go-zero/tools/goctl@latest
#
# Only the two "DO NOT EDIT" files come from goctl:
#   idl/types.go         (request/response structs)  → package idl (shared DTOs)
#   provider/routes.go   (route table)               → package main (server side)
#
# All other files are hand-written and Go-Spring-owned, so they survive
# re-generation: the handler entry (provider/greethandler.go) and the
# ServiceContext + GreetLogic bean (provider/servicecontext.go, provider/logic.go).
#
# We run goctl inside a scratch workspace whose parent module is "greetapi"
# (goctl generates imports rooted at the parent module + --dir subpath), then
# rewrite "greetapi/gen/internal/..." → "greetapi/..." and re-home the two files
# into our flat idl/ + provider/ layout.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

(cd "$tmp" && go mod init greetapi >/dev/null 2>&1)
cp greet.api "$tmp/"
(cd "$tmp" && goctl api go -api greet.api -dir gen --style gozero)

# types.go holds the shared request/response DTOs; it lives in idl/ as package idl.
cp "$tmp/gen/internal/types/types.go" ./types.go
sed -i.bak -e 's|greetapi/gen/internal|greetapi|g' -e 's|^package types$|package idl|' ./types.go
rm -f ./types.go.bak

# routes.go is the server route table; it joins the flat provider package (main).
# ServiceContext now lives in that same package, so drop goctl's svc import and
# the svc. qualifier, and rename the package handler → main.
cp "$tmp/gen/internal/handler/routes.go" ../provider/routes.go
sed -i.bak \
    -e 's|greetapi/gen/internal|greetapi|g' \
    -e '\|"greetapi/svc"|d' \
    -e 's|\*svc\.ServiceContext|*ServiceContext|g' \
    -e 's|^package handler$|package main|' \
    ../provider/routes.go
rm -f ../provider/routes.go.bak

# Re-apply the Apache License header — goctl strips it.
for f in ./types.go ../provider/routes.go; do
    if ! head -1 "$f" | grep -q "Copyright"; then
        cat > "${f}.new" << 'EOF'
// Copyright 2025 The Go-Spring Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

EOF
        cat "$f" >> "${f}.new"
        mv "${f}.new" "$f"
    fi
done

echo "Regenerated: idl/types.go, provider/routes.go"
