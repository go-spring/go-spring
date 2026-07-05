#!/usr/bin/env bash

# Install the gs skill from the go-spring monorepo into the local Claude Code
# skills directory. Designed for remote use, e.g.:
#
#     curl -fsSL <raw-url>/install.sh | bash
#
# The skill files are fetched from the remote repo at the latest
# `skills/gs/vX.Y.Z` tag (overridable via GS_SKILL_REF), so the installed
# SKILL.md version matches a real release and stays in sync with the freshness
# check in gs/internal/claude/claude.go.

set -euo pipefail

repo_url="${GS_SKILL_REPO:-https://github.com/go-spring/go-spring.git}"
tag_prefix="skills/gs/"
dst_dir="${CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}/gs"

# Resolve the ref to install: an explicit GS_SKILL_REF wins (handy for testing
# a branch before any tag exists); otherwise pick the highest skills/gs/v* tag.
ref="${GS_SKILL_REF:-}"
if [ -z "$ref" ]; then
    latest=""
    while IFS= read -r line; do
        tag="${line#*refs/tags/}"
        case "$tag" in
            *"^{}") continue ;;                 # skip peeled annotated-tag refs
            "${tag_prefix}"v*) ;;
            *) continue ;;
        esac
        if [ -z "$latest" ] || \
            [ "$(printf '%s\n%s\n' "$latest" "$tag" | sort -V | tail -n1)" = "$tag" ]; then
            latest="$tag"
        fi
    done < <(git ls-remote --tags "$repo_url" "refs/tags/${tag_prefix}v*")
    if [ -z "$latest" ]; then
        echo "no ${tag_prefix}v* tag found on $repo_url" >&2
        exit 1
    fi
    ref="$latest"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Sparse, blobless clone: fetch only the skills/gs subtree at the target tag
# instead of the whole monorepo tree.
git clone --quiet --depth 1 --branch "$ref" \
    --filter=blob:none --sparse "$repo_url" "$tmp/repo"
git -C "$tmp/repo" sparse-checkout set --no-cone "${tag_prefix%/}"

src_dir="$tmp/repo/${tag_prefix%/}"
if [ ! -d "$src_dir" ]; then
    echo "skill dir ${tag_prefix%/} not present at ref $ref" >&2
    exit 1
fi

mkdir -p "$dst_dir"

# Mirror src -> dst, excluding install.sh itself.
# Use rsync when available for a proper mirror; fall back to cp.
if command -v rsync >/dev/null 2>&1; then
    rsync -a --delete --exclude 'install.sh' "$src_dir/" "$dst_dir/"
else
    find "$dst_dir" -mindepth 1 -delete
    (cd "$src_dir" && find . -type f ! -name install.sh -print0 | \
        while IFS= read -r -d '' f; do
            mkdir -p "$dst_dir/$(dirname "$f")"
            cp "$f" "$dst_dir/$f"
        done)
fi

echo "Installed gs skill ($ref) to $dst_dir"
