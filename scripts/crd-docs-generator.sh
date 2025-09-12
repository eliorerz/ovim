#!/bin/bash

# CRD Documentation Generator
# Generates comprehensive documentation for OVIM CRDs
# Usage: ./scripts/crd-docs-generator.sh [options]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CRD_DOCS_DIR="$PROJECT_ROOT/docs/crds"
KUBECTL_CMD="${KUBECTL_CMD:-kubectl}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    cat << EOF
CRD Documentation Generator

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -h, --help          Show this help message
    -o, --output DIR    Output directory (default: docs/crds)
    -k, --kubectl CMD   kubectl command to use (default: kubectl)

DESCRIPTION:
    Generates comprehensive documentation for OVIM CRDs including:
    - API reference from live cluster (if available)
    - Operations guide
    - Usage examples

EXAMPLES:
    $0                                  # Generate all docs
    $0 -o /tmp/docs                    # Generate to custom directory
    $0 -k oc                           # Use OpenShift CLI

EOF
}

generate_crd_docs() {
    local output_dir="$1"
    
    log_info "Creating documentation directory: $output_dir"
    mkdir -p "$output_dir"
    
    # Try to generate from live cluster first
    if command -v "$KUBECTL_CMD" >/dev/null 2>&1; then
        log_info "Attempting to generate docs from live cluster..."
        
        for crd in organizations virtualdatacenters catalogs; do
            local doc_file="$output_dir/${crd}.md"
            log_info "Generating documentation for $crd.ovim.io..."
            
            if $KUBECTL_CMD explain "${crd}.ovim.io" --recursive > "$doc_file" 2>/dev/null; then
                log_info "Successfully generated $doc_file from cluster"
            else
                log_warn "Could not generate from cluster, creating placeholder for $crd"
                cat > "$doc_file" << EOF
# ${crd^} CRD

## Overview
This is the ${crd^} Custom Resource Definition for OVIM.

## Installation Required
To see the full API documentation, install the CRDs first:
\`\`\`bash
make crd-install
\`\`\`

Then regenerate docs:
\`\`\`bash
make crd-docs
\`\`\`

## Reference
See config/crd/${crd%s}.yaml for the full CRD specification.
EOF
            fi
        done
    else
        log_warn "$KUBECTL_CMD not found, generating placeholder documentation"
        for crd in organizations virtualdatacenters catalogs; do
            local doc_file="$output_dir/${crd}.md"
            cat > "$doc_file" << EOF
# ${crd^} CRD

## Installation Required
Install kubectl/oc and apply CRDs to generate full documentation.

## Reference
See config/crd/${crd%s}.yaml for the full CRD specification.
EOF
        done
    fi
    
    log_info "CRD documentation generated successfully in $output_dir"
}

main() {
    local output_dir="$CRD_DOCS_DIR"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -o|--output)
                output_dir="$2"
                shift 2
                ;;
            -k|--kubectl)
                KUBECTL_CMD="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    generate_crd_docs "$output_dir"
}

main "$@"