#!/bin/bash

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

DRY_RUN=false
LIST_MODE=false
WORKFLOW_DIRS=()

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    cat << EOF
GitHub Actions Version Updater

Usage: $0 [OPTIONS] [WORKFLOW_DIR...]

Options:
    --dry-run       Show what would be updated without making changes
    --list          List all actions found in workflows
    --help          Show this help message

Examples:
    $0                                      # Update all subproject workflows
    $0 --dry-run                           # Preview updates without changes
    $0 --list                              # List all GitHub Actions found
    $0 stdlib/.github/workflows            # Update one workflow directory
    $0 stdlib/.github/workflows log/.github/workflows

EOF
}

check_dependencies() {
    log_info "Checking dependencies..."

    if ! command -v jq &> /dev/null; then
        log_error "jq is not installed, please install jq first"
        log_info "macOS: brew install jq"
        log_info "Ubuntu: sudo apt-get install jq"
        exit 1
    fi

    if ! command -v curl &> /dev/null; then
        log_error "curl is not installed, please install curl first"
        exit 1
    fi

    log_success "Dependencies OK"
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --list)
                LIST_MODE=true
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            --*)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
            *)
                WORKFLOW_DIRS+=("$1")
                shift
                ;;
        esac
    done
}

find_default_workflow_dirs() {
    local dirs=()

    if [[ -d ".github/workflows" ]]; then
        dirs+=(".github/workflows")
    fi

    while IFS= read -r dir; do
        dirs+=("$dir")
    done < <(find . -path "*/node_modules" -prune -o -type d -path "*/.github/workflows" -print | sed 's|^./||' | sort)

    printf '%s\n' "${dirs[@]}" | awk 'NF && !seen[$0]++'
}

resolve_workflow_dirs() {
    if [[ ${#WORKFLOW_DIRS[@]} -eq 0 ]]; then
        while IFS= read -r dir; do
            WORKFLOW_DIRS+=("$dir")
        done < <(find_default_workflow_dirs)
    fi

    if [[ ${#WORKFLOW_DIRS[@]} -eq 0 ]]; then
        log_error "No workflow directories found"
        exit 1
    fi

    local dir
    for dir in "${WORKFLOW_DIRS[@]}"; do
        if [[ ! -d "$dir" ]]; then
            log_error "Workflow directory not found: $dir"
            exit 1
        fi
    done
}

extract_actions() {
    local file=$1

    grep "uses:" "$file" 2>/dev/null | \
        grep -E '@[a-zA-Z0-9_.-]+' | \
        sed -E 's/.*uses: ([^@]+)@([a-zA-Z0-9_.-]+).*/\1@\2/' | \
        sort -u
}

get_workflow_files() {
    local dir=$1

    find "$dir" \( -name "*.yml" -o -name "*.yaml" \) -type f 2>/dev/null | sort
}

get_all_actions() {
    local all_actions=""
    local dir file file_actions

    for dir in "${WORKFLOW_DIRS[@]}"; do
        while IFS= read -r file; do
            file_actions=$(extract_actions "$file")
            if [[ -n "$file_actions" ]]; then
                all_actions+="$file_actions\n"
            fi
        done < <(get_workflow_files "$dir")
    done

    echo -e "$all_actions" | sort -u | grep -v '^$' || true
}

list_actions() {
    log_info "Finding GitHub Actions..."
    local actions_found
    actions_found=$(get_all_actions)

    if [[ -z "$actions_found" ]]; then
        log_warning "No GitHub Actions found"
        return
    fi

    local count
    count=$(echo "$actions_found" | wc -l | tr -d ' ')
    log_success "Found $count actions:"
    echo "$actions_found" | sed 's/^/  - /'
}

get_latest_release() {
    local repo=$1
    local response

    response=$(curl -s --max-time 10 "https://api.github.com/repos/${repo}/releases/latest" 2>/dev/null)

    if [[ $? -eq 0 ]] && [[ $(echo "$response" | jq -r '.message') != "Not Found" ]]; then
        local latest_tag
        latest_tag=$(echo "$response" | jq -r '.tag_name')
        local prerelease
        prerelease=$(echo "$response" | jq -r '.prerelease')

        if [[ "$prerelease" == "false" ]] && [[ -n "$latest_tag" ]] && [[ "$latest_tag" != "null" ]]; then
            echo "$latest_tag"
            return 0
        fi
    fi

    response=$(curl -s --max-time 10 "https://api.github.com/repos/${repo}/releases?per_page=10" 2>/dev/null)

    if [[ $? -eq 0 ]]; then
        local latest_stable
        latest_stable=$(echo "$response" | jq -r '.[] | select(.prerelease == false) | .tag_name' | head -n 1)
        if [[ -n "$latest_stable" ]] && [[ "$latest_stable" != "null" ]]; then
            echo "$latest_stable"
            return 0
        fi
    fi

    response=$(curl -s --max-time 10 "https://api.github.com/repos/${repo}/tags" 2>/dev/null | jq -r '.[0].name')
    if [[ "$response" != "null" ]] && [[ -n "$response" ]]; then
        echo "$response"
        return 0
    fi

    return 1
}

replace_action_version() {
    local file=$1
    local action=$2
    local current_version=$3
    local latest_version=$4

    python3 - "$file" "$action" "$current_version" "$latest_version" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
action = sys.argv[2]
current_version = sys.argv[3]
latest_version = sys.argv[4]
text = path.read_text()
text = text.replace(f"uses: {action}@{current_version}", f"uses: {action}@{latest_version}")
path.write_text(text)
PY
}

update_workflows() {
    local updated_count=0
    local workflow_files=""
    local dir

    for dir in "${WORKFLOW_DIRS[@]}"; do
        workflow_files+="$(get_workflow_files "$dir")"$'\n'
    done
    workflow_files=$(echo "$workflow_files" | grep -v '^$' || true)

    if [[ -z "$workflow_files" ]]; then
        log_warning "No workflow files found"
        return
    fi

    local file_count
    file_count=$(echo "$workflow_files" | wc -l | tr -d ' ')
    log_info "Processing $file_count workflow files..."

    while IFS= read -r file; do
        log_info "Processing: $file"
        local file_updated=false

        while IFS= read -r line || [[ -n "$line" ]]; do
            if [[ $line =~ uses:\ ([^@]+)@([^#[:space:]]+) ]]; then
                local action="${BASH_REMATCH[1]}"
                local current_version="${BASH_REMATCH[2]}"

                log_info "  Checking: $action@$current_version"

                local latest_version
                latest_version=$(get_latest_release "$action" 2>/dev/null)

                if [[ $? -eq 0 ]] && [[ -n "$latest_version" ]]; then
                    if [[ "$current_version" != "$latest_version" ]]; then
                        if [[ $DRY_RUN == true ]]; then
                            log_info "  [DRY-RUN] Would update: $action $current_version -> $latest_version"
                        else
                            log_success "  Updating: $action $current_version -> $latest_version"
                            replace_action_version "$file" "$action" "$current_version" "$latest_version"
                            file_updated=true
                        fi
                    else
                        log_info "  Already latest: $action@$latest_version"
                    fi
                else
                    log_warning "  Cannot fetch latest version for $action"
                fi
            fi
        done < "$file"

        if [[ $file_updated == true ]]; then
            ((updated_count++))
        fi
    done <<< "$workflow_files"

    if [[ $DRY_RUN == true ]]; then
        log_info "Dry run completed. No files were modified."
    else
        log_success "Update completed! $updated_count files were updated."
        log_info "Remember to review changes with 'git diff' and commit if satisfied."
    fi
}

main() {
    parse_arguments "$@"

    log_info "GitHub Actions Version Updater"

    check_dependencies
    resolve_workflow_dirs

    if [[ $LIST_MODE == true ]]; then
        list_actions
        exit 0
    fi

    update_workflows
}

main "$@"
