#!/bin/bash

# Simple script to update GitHub Actions versions in workflow files
# Usage: ./scripts/update-workflow-versions.sh [--dry-run] [--list]

set -euo pipefail

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Global variables
DRY_RUN=false
LIST_MODE=false

# Logging functions
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

# Display help information
show_help() {
    cat << EOF
GitHub Actions Version Updater

Usage: $0 [OPTIONS]

Options:
    --dry-run       Show what would be updated without making changes
    --list          List all actions found in workflows
    --help          Show this help message

Examples:
    $0              # Update all actions to latest versions
    $0 --dry-run    # Preview updates without changes
    $0 --list       # List all GitHub Actions found

EOF
}

# Check if required dependencies are installed
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

# Parse command line arguments
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
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Extract action information from workflow file
extract_actions() {
    local file=$1
    
    # Extract uses lines and parse action@version
    grep "uses:" "$file" 2>/dev/null | \
        grep -E '@[a-zA-Z0-9_.-]+' | \
        sed -E 's/.*uses: ([^@]+)@([a-zA-Z0-9_.-]+).*/\1@\2/' | \
        sort -u
}

# Get all actions from workflow files
get_all_actions() {
    local workflow_dir=".github/workflows"
    local all_actions=""

    for file in "$workflow_dir"/*.yml "$workflow_dir"/*.yaml; do
        if [[ -f "$file" ]]; then
            local file_actions
            file_actions=$(extract_actions "$file")
            if [[ -n "$file_actions" ]]; then
                all_actions+="$file_actions\n"
            fi
        fi
    done
    
    echo -e "$all_actions" | sort -u | grep -v '^$'
}

# List all actions found
list_actions() {
    log_info "Finding GitHub Actions..."
    local actions_found
    actions_found=$(get_all_actions)

    if [[ -z "$actions_found" ]]; then
        log_warning "No GitHub Actions found"
        return
    fi
    
    local count=$(echo "$actions_found" | wc -l | tr -d ' ')
    log_success "Found $count actions:"
    echo "$actions_found" | sed 's/^/  - /'
}

# Get the latest release tag
get_latest_release() {
    local repo=$1
    
    # Try to get the latest official release
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
    
    # Fallback to latest stable release
    response=$(curl -s --max-time 10 "https://api.github.com/repos/${repo}/releases?per_page=10" 2>/dev/null)
    
    if [[ $? -eq 0 ]]; then
        local latest_stable
        latest_stable=$(echo "$response" | jq -r '.[] | select(.prerelease == false) | .tag_name' | head -n 1)
        if [[ -n "$latest_stable" ]] && [[ "$latest_stable" != "null" ]]; then
            echo "$latest_stable"
            return 0
        fi
    fi
    
    # Last resort: latest tag
    response=$(curl -s --max-time 10 "https://api.github.com/repos/${repo}/tags" 2>/dev/null | jq -r '.[0].name')
    if [[ "$response" != "null" ]] && [[ -n "$response" ]]; then
        echo "$response"
        return 0
    fi
    
    return 1
}

# Update workflow files
update_workflows() {
    local workflow_dir=".github/workflows"
    local updated_count=0
    
    # Find all workflow files
    local workflow_files
    workflow_files=$(find "$workflow_dir" -name "*.yml" -o -name "*.yaml" 2>/dev/null)
    
    if [[ -z "$workflow_files" ]]; then
        log_warning "No workflow files found"
        return
    fi
    
    local file_count=$(echo "$workflow_files" | wc -l | tr -d ' ')
    log_info "Processing $file_count workflow files..."
    
    # Process each file
    echo "$workflow_files" | while read -r file; do
        log_info "Processing: $file"
        local file_updated=false
        
        # Read file line by line
        while IFS= read -r line || [[ -n "$line" ]]; do
            if [[ $line =~ uses:\ ([^@]+)@([^#[:space:]]+) ]]; then
                local action="${BASH_REMATCH[1]}"
                local current_version="${BASH_REMATCH[2]}"
                
                log_info "  Checking: $action@$current_version"
                
                # Get latest version
                local latest_version
                latest_version=$(get_latest_release "$action" 2>/dev/null)
                
                if [[ $? -eq 0 ]] && [[ -n "$latest_version" ]]; then
                    if [[ "$current_version" != "$latest_version" ]]; then
                        if [[ $DRY_RUN == true ]]; then
                            log_info "  [DRY-RUN] Would update: $action $current_version -> $latest_version"
                        else
                            log_success "  Updating: $action $current_version -> $latest_version"
                            sed -i '' "s|uses: ${action}@${current_version}|uses: ${action}@${latest_version}|g" "$file"
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
    done
    
    if [[ $DRY_RUN == true ]]; then
        log_info "Dry run completed. No files were modified."
    else
        log_success "Update completed! $updated_count files were updated."
        log_info "Remember to review changes with 'git diff' and commit if satisfied."
    fi
}

# Main function
main() {
    parse_arguments "$@"
    
    log_info "GitHub Actions Version Updater"
    
    # Check dependencies
    check_dependencies
    
    # Handle list mode
    if [[ $LIST_MODE == true ]]; then
        list_actions
        exit 0
    fi
    
    # Check workflow directory
    if [[ ! -d ".github/workflows" ]]; then
        log_error "Workflow directory not found: .github/workflows"
        exit 1
    fi
    
    # Update workflows
    update_workflows
}

# Execute main function
main "$@"