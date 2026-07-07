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

has_findings=0

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ---------------------------------------------------------------------------
# Starter example smoke tests
#
# Every starter/*/example is smoke-tested. A starter with a unique setup (e.g.
# one that needs a backing service) ships its own executable check.sh, which
# owns its whole procedure and is trusted by its exit code. Starters without a
# check.sh use the default runner below, which builds the example and runs it
# under a watchdog; each example self-asserts and exits non-zero on failure.
# ---------------------------------------------------------------------------
SMOKE_TIMEOUT=30

# run_smoke_test <starter-name>
# Builds the example, runs it under a watchdog, and asserts a clean exit.
run_smoke_test() {
    local dir="$1"
    local example_dir="${REPO_ROOT}/${dir}/example"

    local bin log rc=0
    bin="$(mktemp)"
    log="$(mktemp)"
    if ! (cd "${example_dir}" && go build -o "${bin}" .) 2>"${log}"; then
        echo "FAIL ${dir}: build error"
        cat "${log}"
        has_findings=1
        rm -f "${bin}" "${log}"
        return
    fi

    "${bin}" >"${log}" 2>&1 &
    local pid=$!
    ( sleep "${SMOKE_TIMEOUT}"; kill -9 "${pid}" 2>/dev/null ) &
    local watchdog=$!
    wait "${pid}" 2>/dev/null || rc=$?
    kill "${watchdog}" 2>/dev/null || true
    wait "${watchdog}" 2>/dev/null || true

    if [ "${rc}" -ne 0 ]; then
        echo "FAIL ${dir}: exited ${rc}"
        cat "${log}"
        has_findings=1
    else
        echo "OK ${dir}"
    fi
    rm -f "${bin}" "${log}"
}

run_smoke_tests() {
    print_separator
    echo "Starter example smoke tests"
    print_separator
    local example_dir dir
    for example_dir in "${REPO_ROOT}"/starter/*/example; do
        [ -d "${example_dir}" ] || continue
        dir="$(dirname "${example_dir}")"
        dir="${dir#"${REPO_ROOT}"/}"
        if [ -x "${example_dir}/check.sh" ]; then
            echo "Running ${dir} (custom check.sh)..."
            if "${example_dir}/check.sh"; then
                echo "OK ${dir}"
            else
                echo "FAIL ${dir}: check.sh exited non-zero"
                has_findings=1
            fi
        else
            run_smoke_test "${dir}"
        fi
    done
}

run_in_module() {
    local module_dir="$1"

    print_separator
    echo "Module: ${module_dir}"
    print_separator

    echo "Checking error construction..."
    local findings
    findings=$(find "${module_dir}" -type f -name '*.go' \
        ! -path '*/vendor/*' \
        ! -path '*/errutil/*' \
        -exec grep -Hn 'fmt\.Errorf' {} \;
    find "${module_dir}" -type f -name '*.go' \
        ! -path '*/vendor/*' \
        ! -path '*/errutil/*' \
        -exec grep -Hn 'errors\.New' {} \;)
    if [ -n "${findings}" ]; then
        echo "${findings}"
        has_findings=1
    fi

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
    ! -path './gs/gs-http-gen/*' \
    ! -path './temp/*' \
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

run_smoke_tests

print_separator
if [ "${has_findings}" -ne 0 ]; then
    echo "Findings detected in error construction checks. Please fix them."
    print_separator
    exit 1
fi
echo "All checks passed!"
print_separator
