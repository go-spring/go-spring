#!/usr/bin/env bash
#
# gofmt-all.sh - format every Go source file in the monorepo with gofmt.
#
# By default files are rewritten in place. Use --check to instead report files
# whose formatting differs from gofmt's and exit non-zero (CI gate).
#
# Excludes vendored code, test fixtures and scratch directories so that upstream
# code and intentional testdata formatting are left untouched:
#   .git/  vendor/  node_modules/  testdata/  temp/
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

print_separator() {
    echo "=================================================="
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

show_help() {
    cat <<EOF
Usage: ./scripts/gofmt-all.sh [options]

Runs gofmt across every Go file in the repository.

Options:
  -l, --check    List files whose formatting differs from gofmt's and exit 1
                  if any are found. Files are not modified.
  -h, --help     Show this help message

By default (no flags) files are rewritten in place.
Excludes: .git/, vendor/, node_modules/, testdata/, temp/
EOF
}

check_mode=0
if [ $# -gt 0 ]; then
    case "$1" in
        -l|--check) check_mode=1 ;;
        -h|--help)  show_help; exit 0 ;;
        *) echo "Error: unexpected argument $1" >&2; show_help >&2; exit 1 ;;
    esac
fi

if ! command_exists gofmt; then
    echo "Error: gofmt is not installed (ships with the Go toolchain)" >&2
    exit 1
fi

# Collect Go files, skipping vendored code, test fixtures and scratch dirs.
go_files=()
while IFS= read -r f; do
    go_files+=("$f")
done < <(find "$REPO_ROOT" \
    -type f -name '*.go' \
    ! -path '*/.git/*' \
    ! -path '*/vendor/*' \
    ! -path '*/node_modules/*' \
    ! -path '*/testdata/*' \
    ! -path '*/temp/*')

if [ "${#go_files[@]}" -eq 0 ]; then
    echo "No Go files found."
    exit 0
fi

print_separator
if [ "$check_mode" -eq 1 ]; then
    echo "Checking gofmt on ${#go_files[@]} files..."
    print_separator
    output="$(gofmt -l "${go_files[@]}" 2>&1)" || true
    if [ -n "$output" ]; then
        echo "The following files are not gofmt-ed (or failed to parse):"
        echo "$output"
        echo ""
        echo "Run './scripts/gofmt-all.sh' to fix."
        exit 1
    fi
    echo "All Go files are gofmt-ed."
    exit 0
fi

echo "Formatting ${#go_files[@]} files in place..."
print_separator
gofmt -w "${go_files[@]}"
echo "Done."
