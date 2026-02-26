# Tool Version Check Scripts

This directory contains scripts for checking development tool version updates.

## Scripts

### check-go-tool-versions.sh

Checks for updates to Go-installable tools by querying Go module versions.

**Usage:**
```bash
./hack/check-go-tool-versions.sh [makefile-path] [updates-file]
```

**Default values:**
- `makefile-path`: `Makefile`
- `updates-file`: `updates.txt`

**Example:**
```bash
# Check for Go tool updates
./hack/check-go-tool-versions.sh

# Check with custom output file
./hack/check-go-tool-versions.sh Makefile my-updates.txt
```

**Checked tools:**
- kustomize
- controller-gen
- golangci-lint
- crd-ref-docs
- counterfeiter
- ginkgo
- yj
- govulncheck

### check-github-tool-versions.sh

Checks for updates to binary tools by querying the GitHub Releases API.

**Usage:**
```bash
./hack/check-github-tool-versions.sh [makefile-path] [updates-file]
```

**Default values:**
- `makefile-path`: `Makefile`
- `updates-file`: `updates.txt`

**Example:**
```bash
# Check for GitHub binary tool updates
./hack/check-github-tool-versions.sh

# Check and append to existing updates file
./hack/check-github-tool-versions.sh Makefile updates.txt
```

**Checked tools:**
- kind
- ytt
- cmctl

## Output Format

Both scripts append results to the updates file in the format:
```
VAR_NAME|current_version|new_version|repository_url
```

Example:
```
KUSTOMIZE_VERSION|v5.7.1|v5.8.0|https://github.com/kubernetes-sigs/kustomize
KIND_VERSION|v0.30.0|v0.31.0|https://github.com/kubernetes-sigs/kind
```

## Local Testing

To test the scripts locally:

```bash
# Create a temporary updates file
rm -f updates.txt

# Check Go tools
./hack/check-go-tool-versions.sh Makefile updates.txt

# Check GitHub tools (appends to the same file)
./hack/check-github-tool-versions.sh Makefile updates.txt

# View results
cat updates.txt
```

## Requirements

- **For Go tools:** Go must be installed and accessible (no `jq` required)
- **For GitHub tools:** `gh` CLI must be installed and authenticated
- Both scripts require `grep`, `awk`, `sed` (standard on most Unix systems)

## Integration

These scripts are used by the `.github/workflows/update-tool-versions.yml` workflow to automatically check for tool updates and create pull requests.
