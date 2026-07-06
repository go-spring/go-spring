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
# Each entry: <module-dir>|<infra host:port>
# Each example self-asserts and exits non-zero on failure, so we only check the
# exit code. An empty infra field means the example needs no external service;
# otherwise the port is probed first and the test is skipped (with a warning)
# when the service is unreachable.
# ---------------------------------------------------------------------------
STARTER_SMOKE=(
    "starter/starter-gin|"
    "starter/starter-grpc|"
    "starter/starter-thrift|"
    "starter/starter-websocket|"
    "starter/starter-pprof|"
    "starter/starter-go-redis|127.0.0.1:6379"
    "starter/starter-redigo|127.0.0.1:6379"
    "starter/starter-gorm-mysql|127.0.0.1:3306"
)

SMOKE_TIMEOUT=30

# probe_tcp <host:port> — return 0 if a TCP connection can be opened.
probe_tcp() {
    local host="${1%%:*}"
    local port="${1##*:}"
    (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null || return 1
    exec 3>&- 3<&- 2>/dev/null || true
    return 0
}

# run_smoke_test <module-dir> <infra host:port>
# Builds the example, runs it under a watchdog, and asserts a clean exit. Each
# example self-asserts internally and exits non-zero on failure.
run_smoke_test() {
    local dir="$1" infra="$2"
    local example_dir="${REPO_ROOT}/${dir}/example"

    if [ ! -d "${example_dir}" ]; then
        echo "SKIP ${dir}: no example directory"
        return
    fi
    if [ -n "${infra}" ] && ! probe_tcp "${infra}"; then
        echo "WARNING: ${dir}: ${infra} not reachable — skipping smoke test"
        return
    fi

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
    for entry in "${STARTER_SMOKE[@]}"; do
        local dir infra
        IFS='|' read -r dir infra <<<"${entry}"
        if [ -d "${REPO_ROOT}/${dir}" ]; then
            run_smoke_test "${dir}" "${infra}"
        else
            echo "SKIP ${dir}: not present"
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
