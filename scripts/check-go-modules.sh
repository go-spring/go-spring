#!/usr/bin/env bash
set -euo pipefail

export GOEXPERIMENT=jsonv2

print_separator() {
    echo "=================================================="
}

command_exists() {
    if ! command -v "$1" &> /dev/null; then
        return 1
    fi
    return 0
}

install_modernize() {
    echo "modernize not found, installing..."
    go install golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest
    if ! command_exists modernize; then
        export PATH="$PATH:$(go env GOPATH)/bin"
    fi
    if ! command_exists modernize; then
        echo "Error: Failed to install modernize. Please install it manually:"
        echo "  go install golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest"
        exit 1
    fi
    echo "modernize installed successfully"
}

show_help() {
    echo "Usage: ./scripts/check-go-modules.sh [options]"
    echo ""
    echo "Runs error-construction checks, code fixes, modernization, and tests for every Go module"
    echo ""
    echo "Options:"
    echo "  -h, --help    Show this help message"
}

run_in_module() {
    local module_dir="$1"

    print_separator
    echo "Module: ${module_dir}"
    print_separator

    echo "Checking error construction..."
    find "${module_dir}" -type f -name '*.go' \
        ! -path '*/vendor/*' \
        ! -path '*/errutil/*' \
        -exec grep -Hn 'fmt\.Errorf' {} \;
    find "${module_dir}" -type f -name '*.go' \
        ! -path '*/vendor/*' \
        ! -path '*/errutil/*' \
        -exec grep -Hn 'errors\.New' {} \;

    echo "Running go fix..."
    (cd "${module_dir}" && go fix ./...)

    echo "Running modernize..."
    (cd "${module_dir}" && modernize -test ./...)

    echo "Running tests..."
    (cd "${module_dir}" && go test -gcflags="all=-N -l" -count=1 ./...)
}

if [ $# -gt 0 ]; then
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Error: unexpected argument $1"
            show_help
            exit 1
            ;;
    esac
fi

if ! command_exists go; then
    echo "Error: go is not installed"
    exit 1
fi

if ! command_exists modernize; then
    install_modernize
fi

module_dirs=$(find . -name go.mod \
    ! -path '*/vendor/*' \
    ! -path '*/node_modules/*' \
    ! -path './gs/*' \
    ! -path './misc/*' \
    ! -path './website/*' \
    -exec dirname {} \; | sort)

if [ -z "${module_dirs}" ]; then
    echo "Error: no go.mod found"
    exit 1
fi

while IFS= read -r module_dir; do
    run_in_module "${module_dir}"
done <<EOF
${module_dirs}
EOF

print_separator
echo "All checks passed!"
print_separator
