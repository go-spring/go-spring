#!/usr/bin/env bash
#
# Regenerates the goctl-owned files (types + routes) from greet.api. Run this
# after editing greet.api. Requires goctl:
#
#   go install github.com/zeromicro/go-zero/tools/goctl@latest
#
# Only the two "DO NOT EDIT" files come from goctl:
#   internal/types/types.go       (request/response structs)
#   internal/handler/routes.go    (route table)
#
# All other internal/* files (handler entry, svc, logic) are hand-written and
# Go-Spring-owned, so they survive re-generation.
#
# We run goctl inside a scratch workspace whose parent module is "greetapi"
# (goctl generates imports rooted at the parent module + --dir subpath), then
# rewrite "greetapi/gen/internal/..." → "greetapi/internal/..." so the
# generated files sit at project root.
#
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

(cd "$tmp" && go mod init greetapi >/dev/null 2>&1)
cp greet.api "$tmp/"
(cd "$tmp" && goctl api go -api greet.api -dir gen --style gozero)

# Only take types.go and routes.go from the goctl output; everything else is
# hand-written and lives under internal/{handler,svc,logic} already.
cp "$tmp/gen/internal/types/types.go"   internal/types/types.go
cp "$tmp/gen/internal/handler/routes.go" internal/handler/routes.go

# Rewrite the goctl-embedded module path back to our project layout.
for f in internal/types/types.go internal/handler/routes.go; do
    sed -i.bak 's|greetapi/gen/internal|greetapi/internal|g' "$f"
    rm -f "${f}.bak"
done

# Re-apply the Apache License header — goctl strips it.
for f in internal/types/types.go internal/handler/routes.go; do
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

echo "Regenerated: internal/types/types.go, internal/handler/routes.go"
