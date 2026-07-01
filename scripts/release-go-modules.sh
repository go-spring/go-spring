#!/usr/bin/env bash
#
# release.sh — go-spring monorepo serial release driver.
#
# Releases modules in dependency order:
#     stdlib  →  log  →  spring  →  starter-*  →  layout + examples (no tag)
#
# Each round:
#   1. update versions of dependent go-spring.org/* modules in every go.mod
#   2. cd into the module, run `go mod tidy`
#   3. commit go.mod + go.sum
#   4. create the module's subdir tag (e.g. stdlib/v0.1.5)
#   5. push commit, then push tag
#
# Every destructive step pauses for explicit confirmation.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
log_step()    { echo -e "\n${BOLD}${BLUE}==> $*${NC}"; }
log_success() { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error()   { echo -e "${RED}[ERR]${NC} $*"; }

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

# ---------------------------------------------------------------------------
# Proxy / GOPROXY setup
# ---------------------------------------------------------------------------
# `go mod tidy` must reach origin to resolve freshly-pushed tags. Set
# GOPROXY=direct so tidy talks to git directly (avoids proxy.golang.org cache
# lag), then route git HTTPS through the local proxy.
export GOPROXY=direct
export GOSUMDB=off
export https_proxy="${https_proxy:-http://127.0.0.1:7897}"
export http_proxy="${http_proxy:-http://127.0.0.1:7897}"
export all_proxy="${all_proxy:-socks5://127.0.0.1:7897}"

# ---------------------------------------------------------------------------
# Module table.
#
# Each entry:  <module-dir>:<tag-prefix>:<version-var>
#
# tag-prefix is what we'll prepend before the version when tagging
# (subdirectory tag convention). Empty version-var means "no tag in this
# round" (used for layout/examples).
# ---------------------------------------------------------------------------
STDLIB_DIR="stdlib"
LOG_DIR="log"
SPRING_DIR="spring"

STARTER_DIRS=(
    "starter/starter-gin"
    "starter/starter-echo"      # remove this line if the dir doesn't exist
    "starter/starter-grpc"
    "starter/starter-go-redis"
    "starter/starter-redigo"
    "starter/starter-gorm-mysql"
    "starter/starter-pprof"
    "starter/starter-thrift"
)
# Filter out non-existent starter directories so the script stays valid
# when the starter list changes.
EXISTING_STARTERS=()
for d in "${STARTER_DIRS[@]}"; do
    [[ -d "${REPO_ROOT}/${d}" ]] && EXISTING_STARTERS+=("${d}")
done

LAYOUT_DIRS=(
    "layout"
    "examples/bookman"
    "examples/chatAI"
    "examples/miniapi"
    "examples/startup"
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# confirm <prompt>  — ask the user to press y / Y / yes to proceed.
confirm() {
    local prompt="$1"
    local ans
    while true; do
        read -r -p "$(echo -e "${YELLOW}${prompt} [y/N/q] ${NC}")" ans
        case "${ans}" in
            y|Y|yes|YES) return 0 ;;
            q|Q|quit)    log_warn "aborted by user"; exit 130 ;;
            *)           return 1 ;;
        esac
    done
}

# pause <prompt>  — wait for ENTER; used between major phases.
pause() {
    local msg="${1:-press ENTER to continue, or Ctrl-C to abort}"
    read -r -p "$(echo -e "${YELLOW}${msg}${NC} ")" _
}

# ask_version <var-name> <module-label>  — prompt for a version string.
ask_version() {
    local __var="$1"
    local label="$2"
    local current="${3:-}"
    local input
    read -r -p "$(echo -e "${BOLD}new version for ${label}${NC} (current: ${current:-?}) > ")" input
    if [[ -z "${input}" ]]; then
        log_error "version cannot be empty"
        exit 2
    fi
    if [[ ! "${input}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
        log_error "version '${input}' does not look like semver (expected vX.Y.Z)"
        exit 2
    fi
    printf -v "${__var}" '%s' "${input}"
}

# bump_dep <module-path> <new-version>
# Rewrites every go.mod in the repo to pin <module-path> at <new-version>.
# Matches both direct and indirect require lines.
bump_dep() {
    local mod="$1"
    local ver="$2"
    log_info "bump ${mod} → ${ver} across all go.mod"
    # find -print0 / xargs to handle paths safely
    find "${REPO_ROOT}" -name go.mod -not -path '*/.*' -print0 \
        | while IFS= read -r -d '' f; do
            # Replace lines like:  <mod> vX.Y.Z [// indirect]
            # Keep trailing comments intact.
            if grep -q "${mod} v" "${f}"; then
                # Use a perl one-liner for portable in-place edit.
                perl -pi -e "s{(${mod}) v[0-9A-Za-z.+\\-]+}{\$1 ${ver}}g" "${f}"
            fi
          done
}

# show_status — short git status; used at every confirmation gate.
show_status() {
    log_info "git status:"
    git status -s
}

# tidy_dir <dir>
tidy_dir() {
    local d="$1"
    log_info "go mod tidy in ${d}"
    ( cd "${REPO_ROOT}/${d}" && go mod tidy )
}

# Commit + tag + push for a single module round.
# Args:
#   $1 module dir   (e.g. stdlib)
#   $2 tag prefix   (e.g. stdlib)
#   $3 version      (e.g. v0.1.5)
#   $4 commit msg subject
release_single() {
    local dir="$1"
    local tag_prefix="$2"
    local ver="$3"
    local subject="$4"
    local tag="${tag_prefix}/${ver}"

    log_step "Round: ${dir} (tag ${tag})"

    tidy_dir "${dir}"
    show_status

    if ! confirm "stage and commit changes in ${dir}?"; then
        log_warn "skipping ${dir}"
        return
    fi
    git add "${dir}/go.mod" "${dir}/go.sum"
    if git diff --cached --quiet; then
        log_info "${dir}: nothing to commit — will tag current HEAD"
    else
        git commit -m "${subject}"
        log_success "committed ${dir}"
    fi

    if ! confirm "create tag ${tag}?"; then
        log_warn "tag skipped — release_single will not push"
        return
    fi
    git tag "${tag}"
    log_success "tagged ${tag}"

    if ! confirm "push commit AND tag ${tag} to origin?"; then
        log_warn "push skipped — remember to push manually before next round"
        return
    fi
    git push origin main
    git push origin "${tag}"
    log_success "pushed ${dir} + ${tag}"
}

# Commit + multi-tag + push for the starter batch.
# Args: $1 version, then a list of starter dirs.
release_starters() {
    local ver="$1"; shift
    local dirs=("$@")

    log_step "Round: starter batch (version ${ver})"

    for d in "${dirs[@]}"; do
        tidy_dir "${d}"
    done
    show_status

    if ! confirm "stage and commit ALL starter go.mod/go.sum changes?"; then
        log_warn "skipping starter batch"
        return
    fi
    for d in "${dirs[@]}"; do
        git add "${d}/go.mod" "${d}/go.sum"
    done
    git commit -m "chore(starter): bump dependencies to release ${ver}"

    # Create one tag per starter on the same commit.
    local tags=()
    for d in "${dirs[@]}"; do
        # Tag uses the subdir path, e.g. starter/starter-gin/v1.3.3
        local t="${d}/${ver}"
        tags+=("${t}")
    done

    log_info "tags to create:"
    printf '  %s\n' "${tags[@]}"
    if ! confirm "create all ${#tags[@]} starter tags?"; then
        log_warn "tags skipped"
        return
    fi
    for t in "${tags[@]}"; do
        git tag "${t}"
    done
    log_success "tagged ${#tags[@]} starters"

    if ! confirm "push commit AND all starter tags to origin?"; then
        log_warn "push skipped"
        return
    fi
    git push origin main
    for t in "${tags[@]}"; do
        git push origin "${t}"
    done
    log_success "starter batch released"
}

# Commit + push (no tag) for layout + examples.
release_layout_examples() {
    local dirs=("$@")

    log_step "Final round: layout + examples (no tag)"

    for d in "${dirs[@]}"; do
        tidy_dir "${d}"
    done
    show_status

    log_warn "review the diff carefully — tidy may legitimately remove unused direct deps"
    pause "press ENTER once the diff has been reviewed"

    if ! confirm "stage and commit layout + examples go.mod/go.sum?"; then
        log_warn "skipping final round"
        return
    fi
    for d in "${dirs[@]}"; do
        [[ -f "${d}/go.mod" ]] && git add "${d}/go.mod" || true
        [[ -f "${d}/go.sum" ]] && git add "${d}/go.sum" || true
    done
    git commit -m "chore: refresh go.sum for layout and examples"

    if ! confirm "push final commit to origin?"; then
        log_warn "push skipped"
        return
    fi
    git push origin main
    log_success "layout + examples released"
}

# ---------------------------------------------------------------------------
# Main flow
# ---------------------------------------------------------------------------

log_step "go-spring release driver"
log_info "repo root: ${REPO_ROOT}"
log_info "GOPROXY=${GOPROXY}  GOSUMDB=${GOSUMDB}  https_proxy=${https_proxy}"

# 0. Sanity checks ----------------------------------------------------------
if [[ -n "$(git status --porcelain)" ]]; then
    log_error "working tree is not clean — commit or stash before running"
    git status -s
    exit 1
fi
if [[ "$(git rev-parse --abbrev-ref HEAD)" != "main" ]]; then
    log_warn "current branch is not main"
    confirm "continue anyway?" || exit 1
fi
# Ensure local HEAD is in sync with origin — un-pushed commits would ride
# along with the first release push, and un-pulled commits would break tidy.
log_info "fetching origin to compare local vs remote HEAD"
git fetch origin --quiet
LOCAL_HEAD="$(git rev-parse HEAD)"
REMOTE_HEAD="$(git rev-parse origin/main)"
if [[ "${LOCAL_HEAD}" != "${REMOTE_HEAD}" ]]; then
    AHEAD="$(git rev-list --count origin/main..HEAD)"
    BEHIND="$(git rev-list --count HEAD..origin/main)"
    log_error "local main is out of sync with origin/main (ahead ${AHEAD}, behind ${BEHIND})"
    git log --oneline --graph --decorate --boundary origin/main...HEAD | head -20
    log_error "push or reset first, then re-run this script"
    exit 1
fi

# 1. Collect versions -------------------------------------------------------
log_step "Step 1 / collect target versions"
ask_version STDLIB_VER  "stdlib"      "$(grep -m1 'go-spring.org/stdlib v' "${REPO_ROOT}/log/go.mod" | awk '{print $2}' || true)"
ask_version LOG_VER     "log"         "$(grep -m1 'go-spring.org/log v'    "${REPO_ROOT}/spring/go.mod" | awk '{print $2}' || true)"
ask_version SPRING_VER  "spring"      "$(grep -m1 'go-spring.org/spring v' "${REPO_ROOT}/${EXISTING_STARTERS[0]}/go.mod" | awk '{print $2}' || true)"
STARTER_VER="${SPRING_VER}"

# Cross-check: spring/gs/gs.go carries an embedded Version constant that must
# match SPRING_VER, otherwise the compiled binary will advertise a stale tag.
GS_FILE="${REPO_ROOT}/spring/gs/gs.go"
if [[ -f "${GS_FILE}" ]]; then
    GS_VER="$(grep -m1 -E 'Version[[:space:]]*=[[:space:]]*"go-spring@v[0-9A-Za-z.+\-]+"' "${GS_FILE}" \
        | sed -E 's/.*go-spring@(v[0-9A-Za-z.+\-]+).*/\1/')"
    if [[ -z "${GS_VER}" ]]; then
        log_warn "could not parse Version constant in ${GS_FILE}"
    elif [[ "${GS_VER}" != "${SPRING_VER}" ]]; then
        log_error "spring/gs/gs.go Version=${GS_VER} does not match SPRING_VER=${SPRING_VER}"
        log_error "update ${GS_FILE} manually, then re-run this script"
        exit 1
    else
        log_info "spring/gs/gs.go Version matches SPRING_VER (${SPRING_VER})"
    fi
fi

cat <<EOF

${BOLD}Target versions${NC}
  stdlib    : ${STDLIB_VER}
  log       : ${LOG_VER}
  spring    : ${SPRING_VER}
  starter-* : ${STARTER_VER}
EOF
confirm "proceed with these versions?" || exit 1

# 2. Bulk version bump ------------------------------------------------------
log_step "Step 2 / bump go.mod versions in-place"
bump_dep "go-spring.org/stdlib"  "${STDLIB_VER}"
bump_dep "go-spring.org/log"     "${LOG_VER}"
bump_dep "go-spring.org/spring"  "${SPRING_VER}"
for s in "${EXISTING_STARTERS[@]}"; do
    name="$(basename "${s}")"
    bump_dep "go-spring.org/${name}" "${STARTER_VER}"
done

show_status
log_warn "this is a non-committed bulk edit — every subsequent round will pick up its own slice"
pause "press ENTER to start releases"

# 3. stdlib -----------------------------------------------------------------
release_single "${STDLIB_DIR}" "stdlib" "${STDLIB_VER}" \
    "chore(stdlib): release ${STDLIB_VER}"

# 4. log --------------------------------------------------------------------
release_single "${LOG_DIR}" "log" "${LOG_VER}" \
    "chore(log): release ${LOG_VER}"

# 5. spring -----------------------------------------------------------------
release_single "${SPRING_DIR}" "spring" "${SPRING_VER}" \
    "chore(spring): release ${SPRING_VER}"

# 6. starter batch ----------------------------------------------------------
release_starters "${STARTER_VER}" "${EXISTING_STARTERS[@]}"

# 7. layout + examples ------------------------------------------------------
release_layout_examples "${LAYOUT_DIRS[@]}"

log_step "all rounds complete"
log_success "release flow finished"
