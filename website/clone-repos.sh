#!/bin/bash

# Clone all repositories from go-spring organization
# Usage: ./clone-all-go-spring.sh [target_directory]

set -e

ORG="go-spring"
TARGET_DIR="${1:-$PWD}"
PER_PAGE=100

echo "Starting to clone all repositories from $ORG organization..."
echo "Target directory: $TARGET_DIR"
cd "$TARGET_DIR"

# Get all repositories (handle pagination)
page=1
all_repos=()

while true; do
    echo "Fetching repository list page $page..."
    repos=$(curl -s "https://api.github.com/orgs/$ORG/repos?per_page=$PER_PAGE&page=$page" | grep -o '"clone_url": *"[^"]*"' | cut -d'"' -f4)

    if [ -z "$repos" ]; then
        break
    fi

    all_repos+=($repos)
    page=$((page + 1))
done

total=${#all_repos[@]}
echo "Found $total repositories"

# Clone or pull each repository
index=1
for repo in "${all_repos[@]}"; do
    repo_name=$(basename "$repo" .git)
    echo "[$index/$total] Processing $repo_name..."

    if [ -d "$repo_name" ]; then
        echo "  Directory exists, running git pull..."
        (cd "$repo_name" && git pull)
    else
        echo "  Cloning..."
        git clone "$repo"
    fi

    index=$((index + 1))
done

echo ""
echo "Done! Processed $total repositories"
