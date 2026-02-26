#!/usr/bin/env bash

# Script to check for updates to GitHub binary tools
# Usage: ./hack/check-github-tool-versions.sh <makefile-path> <updates-file>
#
# This script reads tool versions from the Makefile and checks for updates
# using the gh CLI. Results are appended to the updates file.

set -euo pipefail

MAKEFILE="${1:-Makefile}"
UPDATES_FILE="${2:-updates.txt}"

echo "Checking GitHub binary tools for updates..."

# Function to check GitHub release version
check_github_version() {
  local var_name=$1
  local repo=$2
  local current_version=$3
  
  echo "Checking $var_name (current: $current_version)..."
  
  # Get latest release tag using the gh CLI
  latest=$(gh release view -R "$repo" --json tagName --template '{{ .tagName }}{{ "\n" }}' 2>/dev/null || echo "")
  
  if [ -z "$latest" ]; then
    echo "  Warning: Could not fetch latest release for $repo"
    return
  fi
  
  # Compare versions
  if [ "$current_version" != "$latest" ]; then
    echo "  ✓ Update available: $current_version → $latest"
    echo "$var_name|$current_version|$latest|https://github.com/$repo" >> "$UPDATES_FILE"
  else
    echo "  Already up-to-date"
  fi
}

# Read current versions from Makefile
KIND_VERSION=$(grep '^KIND_VERSION' "$MAKEFILE" | awk -F'= ' '{print $2}')
YTT_VERSION=$(grep '^YTT_VERSION' "$MAKEFILE" | awk -F'= ' '{print $2}')
CMCTL_VERSION=$(grep '^CMCTL_VERSION' "$MAKEFILE" | awk -F'= ' '{print $2}')

# Check each GitHub binary tool
check_github_version "KIND_VERSION" "kubernetes-sigs/kind" "$KIND_VERSION"
check_github_version "YTT_VERSION" "carvel-dev/ytt" "$YTT_VERSION"
check_github_version "CMCTL_VERSION" "cert-manager/cmctl" "$CMCTL_VERSION"

echo "GitHub tools check complete"
