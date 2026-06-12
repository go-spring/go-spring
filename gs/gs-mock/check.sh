#!/usr/bin/env bash
set -euo pipefail

export GOEXPERIMENT=jsonv2

# Print separator
print_separator() {
    echo "=================================================="
}

# Check if command exists
command_exists() {
    if ! command -v "$1" &> /dev/null; then
        return 1
    fi
    return 0
}

# Install modernize
install_modernize() {
    echo "modernize not found, installing..."
    go install golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest
    # Add GOPATH/bin to PATH if needed
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

# Show help
show_help() {
    echo "Usage: ./check.sh [options]"
    echo ""
    echo "Runs code fixes, modernization, and tests"
    echo ""
    echo "Options:"
    echo "  -h, --help    Show this help message"
    exit 0
}

# Parse arguments
if [ $# -gt 0 ]; then
    case "$1" in
        -h|--help)
            show_help
            ;;
    esac
fi

# Check dependencies
if ! command_exists go; then
    echo "Error: go is not installed"
    exit 1
fi

# Check and install modernize if needed
if ! command_exists modernize; then
    install_modernize
fi

print_separator
echo "Step 1/3: Running go fix..."
print_separator
go fix ./...

print_separator
echo "Step 2/3: Running modernize..."
print_separator
modernize -test ./...

print_separator
echo "Step 3/3: Running tests..."
print_separator
go test -gcflags="all=-N -l" -count=1 ./...

print_separator
echo "✅  All checks passed!"
print_separator