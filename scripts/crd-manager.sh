#!/bin/bash

# CRD Manager
# Manages OVIM CRD installation, validation, and lifecycle
# Usage: ./scripts/crd-manager.sh [command] [options]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CRD_DIR="$PROJECT_ROOT/config/crd"
KUBECTL_CMD="${KUBECTL_CMD:-kubectl}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

show_help() {
    cat << EOF
CRD Manager - OVIM Custom Resource Definition Management

USAGE:
    $0 COMMAND [OPTIONS]

COMMANDS:
    install             Install OVIM CRDs to the cluster
    uninstall          Uninstall OVIM CRDs from the cluster
    validate           Validate CRD installation
    status             Show status of OVIM CRDs
    dry-run            Show what would be installed without applying
    help               Show this help message

OPTIONS:
    -k, --kubectl CMD   kubectl command to use (default: kubectl)
    -n, --namespace NS  Namespace for namespaced operations
    -f, --force        Force operation (skip confirmations)
    -v, --verbose      Verbose output

EXAMPLES:
    $0 install                         # Install CRDs using kubectl
    $0 install -k oc                   # Install CRDs using oc (OpenShift)
    $0 validate -v                     # Validate installation with verbose output
    $0 status                          # Show CRD status
    $0 uninstall -f                    # Force uninstall without confirmation

DESCRIPTION:
    This script manages the lifecycle of OVIM Custom Resource Definitions:
    - Organization CRD (cluster-scoped)
    - VirtualDataCenter CRD (namespaced)
    - Catalog CRD (namespaced)

EOF
}

check_prerequisites() {
    log_debug "Checking prerequisites..."
    
    if ! command -v "$KUBECTL_CMD" >/dev/null 2>&1; then
        log_error "$KUBECTL_CMD command not found. Please install kubectl or oc."
        exit 1
    fi
    
    if ! "$KUBECTL_CMD" cluster-info >/dev/null 2>&1; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    
    if [[ ! -d "$CRD_DIR" ]]; then
        log_error "CRD directory not found: $CRD_DIR"
        exit 1
    fi
    
    log_debug "Prerequisites check passed"
}

get_crd_files() {
    local crd_files=()
    for crd in organization virtualdatacenter catalog; do
        local crd_file="$CRD_DIR/${crd}.yaml"
        if [[ -f "$crd_file" ]]; then
            crd_files+=("$crd_file")
        else
            log_warn "CRD file not found: $crd_file"
        fi
    done
    echo "${crd_files[@]}"
}

install_crds() {
    local dry_run="${1:-false}"
    local force="${2:-false}"
    
    log_info "Installing OVIM CRDs..."
    
    local crd_files
    read -ra crd_files <<< "$(get_crd_files)"
    
    if [[ ${#crd_files[@]} -eq 0 ]]; then
        log_error "No CRD files found to install"
        exit 1
    fi
    
    # Check if CRDs already exist
    local existing_crds=()
    for crd_name in organizations.ovim.io virtualdatacenters.ovim.io catalogs.ovim.io; do
        if "$KUBECTL_CMD" get crd "$crd_name" >/dev/null 2>&1; then
            existing_crds+=("$crd_name")
        fi
    done
    
    if [[ ${#existing_crds[@]} -gt 0 ]] && [[ "$force" != "true" ]]; then
        log_warn "The following CRDs already exist:"
        printf '  - %s\n' "${existing_crds[@]}"
        echo
        read -p "Do you want to update them? (y/N): " -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Installation cancelled"
            exit 0
        fi
    fi
    
    # Install CRDs
    for crd_file in "${crd_files[@]}"; do
        local crd_name
        crd_name=$(basename "$crd_file" .yaml)
        log_info "Installing $crd_name CRD..."
        
        if [[ "$dry_run" == "true" ]]; then
            log_info "DRY RUN: Would apply $crd_file"
            "$KUBECTL_CMD" apply -f "$crd_file" --dry-run=client
        else
            if "$KUBECTL_CMD" apply -f "$crd_file"; then
                log_info "Successfully installed $crd_name CRD"
            else
                log_error "Failed to install $crd_name CRD"
                exit 1
            fi
        fi
    done
    
    if [[ "$dry_run" != "true" ]]; then
        log_info "Waiting for CRDs to be established..."
        sleep 2
        validate_installation
        log_info "CRD installation completed successfully"
    fi
}

uninstall_crds() {
    local force="${1:-false}"
    
    log_warn "WARNING: This will remove all OVIM CRDs and their resources!"
    
    if [[ "$force" != "true" ]]; then
        echo
        read -p "Are you sure you want to continue? Type 'yes' to confirm: " -r
        if [[ "$REPLY" != "yes" ]]; then
            log_info "Uninstallation cancelled"
            exit 0
        fi
    fi
    
    # List existing resources before deletion
    log_info "Checking for existing OVIM resources..."
    for resource in organizations virtualdatacenters catalogs; do
        local count
        count=$("$KUBECTL_CMD" get "$resource" --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
        if [[ "$count" -gt 0 ]]; then
            log_warn "Found $count $resource resources - they will be deleted"
        fi
    done
    
    # Delete CRDs (this will cascade delete all resources)
    log_info "Removing OVIM CRDs..."
    for crd_name in organizations.ovim.io virtualdatacenters.ovim.io catalogs.ovim.io; do
        if "$KUBECTL_CMD" get crd "$crd_name" >/dev/null 2>&1; then
            log_info "Removing $crd_name..."
            if "$KUBECTL_CMD" delete crd "$crd_name"; then
                log_info "Successfully removed $crd_name"
            else
                log_warn "Failed to remove $crd_name"
            fi
        else
            log_debug "$crd_name not found, skipping"
        fi
    done
    
    log_info "CRD uninstallation completed"
}

validate_installation() {
    log_info "Validating CRD installation..."
    
    local all_valid=true
    
    # Check CRD existence and status
    for crd_name in organizations.ovim.io virtualdatacenters.ovim.io catalogs.ovim.io; do
        log_debug "Checking $crd_name..."
        
        if ! "$KUBECTL_CMD" get crd "$crd_name" >/dev/null 2>&1; then
            log_error "CRD $crd_name not found"
            all_valid=false
            continue
        fi
        
        # Check if CRD is established
        local established
        established=$("$KUBECTL_CMD" get crd "$crd_name" -o jsonpath='{.status.conditions[?(@.type=="Established")].status}' 2>/dev/null || echo "Unknown")
        
        if [[ "$established" == "True" ]]; then
            log_info "✓ $crd_name is established"
        else
            log_error "✗ $crd_name is not established (status: $established)"
            all_valid=false
        fi
        
        # Test resource creation (dry-run)
        local resource_type
        case "$crd_name" in
            organizations.ovim.io) resource_type="organizations" ;;
            virtualdatacenters.ovim.io) resource_type="virtualdatacenters" ;;
            catalogs.ovim.io) resource_type="catalogs" ;;
        esac
        
        if "$KUBECTL_CMD" get "$resource_type" --dry-run >/dev/null 2>&1; then
            log_debug "✓ $resource_type API is accessible"
        else
            log_warn "✗ $resource_type API may not be ready"
        fi
    done
    
    if [[ "$all_valid" == "true" ]]; then
        log_info "✓ All CRDs are valid and established"
        return 0
    else
        log_error "✗ CRD validation failed"
        return 1
    fi
}

show_status() {
    log_info "OVIM CRD Status:"
    echo
    
    # Check cluster connection
    if ! "$KUBECTL_CMD" cluster-info >/dev/null 2>&1; then
        log_error "Cannot connect to cluster"
        return 1
    fi
    
    local cluster_info
    cluster_info=$("$KUBECTL_CMD" cluster-info | head -1)
    echo "Cluster: $cluster_info"
    echo
    
    # CRD Status
    echo "CRD Status:"
    for crd_name in organizations.ovim.io virtualdatacenters.ovim.io catalogs.ovim.io; do
        if "$KUBECTL_CMD" get crd "$crd_name" >/dev/null 2>&1; then
            local version age established
            version=$("$KUBECTL_CMD" get crd "$crd_name" -o jsonpath='{.spec.versions[0].name}')
            age=$("$KUBECTL_CMD" get crd "$crd_name" -o jsonpath='{.metadata.creationTimestamp}')
            established=$("$KUBECTL_CMD" get crd "$crd_name" -o jsonpath='{.status.conditions[?(@.type=="Established")].status}')
            
            local status_icon="✓"
            [[ "$established" != "True" ]] && status_icon="✗"
            
            printf "  %s %-30s version=%s established=%s\n" "$status_icon" "$crd_name" "$version" "$established"
        else
            printf "  ✗ %-30s NOT FOUND\n" "$crd_name"
        fi
    done
    
    echo
    
    # Resource counts
    echo "Resource Counts:"
    for resource in organizations virtualdatacenters catalogs; do
        local count
        if count=$("$KUBECTL_CMD" get "$resource" --all-namespaces --no-headers 2>/dev/null | wc -l); then
            printf "  %-20s %d\n" "$resource" "$count"
        else
            printf "  %-20s N/A\n" "$resource"
        fi
    done
}

main() {
    local command=""
    local force="false"
    local verbose="false"
    local dry_run="false"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            install|uninstall|validate|status|dry-run|help)
                command="$1"
                shift
                ;;
            -k|--kubectl)
                KUBECTL_CMD="$2"
                shift 2
                ;;
            -f|--force)
                force="true"
                shift
                ;;
            -v|--verbose)
                verbose="true"
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    if [[ -z "$command" ]]; then
        log_error "No command specified"
        show_help
        exit 1
    fi
    
    if [[ "$verbose" == "true" ]]; then
        set -x
    fi
    
    case "$command" in
        install)
            check_prerequisites
            install_crds "$dry_run" "$force"
            ;;
        uninstall)
            check_prerequisites
            uninstall_crds "$force"
            ;;
        validate)
            check_prerequisites
            validate_installation
            ;;
        status)
            check_prerequisites
            show_status
            ;;
        dry-run)
            check_prerequisites
            install_crds "true" "$force"
            ;;
        help)
            show_help
            ;;
        *)
            log_error "Unknown command: $command"
            show_help
            exit 1
            ;;
    esac
}

main "$@"