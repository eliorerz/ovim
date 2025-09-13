#!/bin/bash

# OVIM Undeployment Script
# Clean removal of OVIM components from Kubernetes cluster
# Usage: ./scripts/undeploy.sh [--namespace NAMESPACE] [--force] [--help]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default configuration
DEFAULT_NAMESPACE="${OVIM_NAMESPACE:-ovim-system}"
DEFAULT_KUBECTL="${KUBECTL_CMD:-kubectl}"

# Configuration variables
NAMESPACE="$DEFAULT_NAMESPACE"
KUBECTL_CMD="$DEFAULT_KUBECTL"
FORCE="false"
KEEP_NAMESPACE="false"
KEEP_CRDS="false"
DRY_RUN="false"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

show_help() {
    cat << EOF
OVIM Undeployment Script

USAGE:
    ./scripts/undeploy.sh [OPTIONS]

OPTIONS:
    --namespace NAMESPACE    Kubernetes namespace to clean (default: $DEFAULT_NAMESPACE)
    --kubectl CMD           kubectl command to use (default: $DEFAULT_KUBECTL)
    --force                 Skip confirmation prompts
    --keep-namespace        Don't delete the namespace
    --keep-crds             Don't delete CRDs (keeps custom resources)
    --dry-run               Show what would be deleted without removing
    --help                  Show this help message

ENVIRONMENT VARIABLES:
    OVIM_NAMESPACE          Default namespace (default: ovim-system)
    KUBECTL_CMD             kubectl command to use

EXAMPLES:
    # Remove OVIM from default namespace (with confirmation)
    ./scripts/undeploy.sh

    # Force removal without prompts
    ./scripts/undeploy.sh --force

    # Dry run to see what would be removed
    ./scripts/undeploy.sh --dry-run

    # Remove from custom namespace but keep CRDs
    ./scripts/undeploy.sh --namespace my-ovim --keep-crds

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --kubectl)
            KUBECTL_CMD="$2"
            shift 2
            ;;
        --force)
            FORCE="true"
            shift
            ;;
        --keep-namespace)
            KEEP_NAMESPACE="true"
            shift
            ;;
        --keep-crds)
            KEEP_CRDS="true"
            shift
            ;;
        --dry-run)
            DRY_RUN="true"
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

# Confirm deletion unless forced
confirm_deletion() {
    if [ "$FORCE" = "true" ] || [ "$DRY_RUN" = "true" ]; then
        return
    fi
    
    echo ""
    log_warn "This will remove OVIM components from namespace '$NAMESPACE'"
    
    if [ "$KEEP_CRDS" = "false" ]; then
        log_warn "This will also DELETE ALL CUSTOM RESOURCES (Organizations, VDCs, Catalogs)"
    fi
    
    echo ""
    read -p "Are you sure you want to continue? (type 'yes' to confirm): " confirm
    if [ "$confirm" != "yes" ]; then
        log_info "Undeployment cancelled"
        exit 0
    fi
}

# Check if kubectl is available and connected
check_kubectl() {
    log_step "Checking kubectl connectivity..."
    
    if ! command -v "$KUBECTL_CMD" &> /dev/null; then
        log_error "kubectl command not found: $KUBECTL_CMD"
        exit 1
    fi
    
    if ! $KUBECTL_CMD cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    
    log_info "kubectl connectivity verified"
}

# Remove controller deployment
remove_controller() {
    log_step "Removing OVIM controller..."
    
    local resources=(
        "deployment/ovim-controller"
        "service/ovim-controller-metrics"
    )
    
    for resource in "${resources[@]}"; do
        if $KUBECTL_CMD get "$resource" -n "$NAMESPACE" &> /dev/null; then
            if [ "$DRY_RUN" = "true" ]; then
                log_info "[DRY-RUN] Would delete: $resource"
            else
                log_info "Deleting $resource..."
                $KUBECTL_CMD delete "$resource" -n "$NAMESPACE" --ignore-not-found=true
            fi
        else
            log_info "$resource not found, skipping"
        fi
    done
    
    log_info "Controller components removed"
}

# Remove RBAC
remove_rbac() {
    log_step "Removing RBAC components..."
    
    local rbac_resources=(
        "clusterrolebinding/ovim-controller"
        "clusterrole/ovim-controller"
        "serviceaccount/ovim-controller"
    )
    
    for resource in "${rbac_resources[@]}"; do
        # Handle cluster-scoped and namespaced resources differently
        if [[ "$resource" == clusterrole* ]] || [[ "$resource" == clusterrolebinding* ]]; then
            if $KUBECTL_CMD get "$resource" &> /dev/null; then
                if [ "$DRY_RUN" = "true" ]; then
                    log_info "[DRY-RUN] Would delete: $resource"
                else
                    log_info "Deleting $resource..."
                    $KUBECTL_CMD delete "$resource" --ignore-not-found=true
                fi
            else
                log_info "$resource not found, skipping"
            fi
        else
            if $KUBECTL_CMD get "$resource" -n "$NAMESPACE" &> /dev/null; then
                if [ "$DRY_RUN" = "true" ]; then
                    log_info "[DRY-RUN] Would delete: $resource"
                else
                    log_info "Deleting $resource..."
                    $KUBECTL_CMD delete "$resource" -n "$NAMESPACE" --ignore-not-found=true
                fi
            else
                log_info "$resource not found, skipping"
            fi
        fi
    done
    
    log_info "RBAC components removed"
}

# Remove custom resources
remove_custom_resources() {
    if [ "$KEEP_CRDS" = "true" ]; then
        log_info "Skipping custom resource removal (--keep-crds specified)"
        return
    fi
    
    log_step "Removing custom resources..."
    
    # List all custom resources across all namespaces
    local cr_types=("organizations" "virtualdatacenters" "catalogs")
    
    for cr_type in "${cr_types[@]}"; do
        log_info "Checking for $cr_type..."
        
        if $KUBECTL_CMD get "$cr_type" --all-namespaces &> /dev/null; then
            local resources
            resources=$($KUBECTL_CMD get "$cr_type" --all-namespaces -o name 2>/dev/null || true)
            
            if [ -n "$resources" ]; then
                if [ "$DRY_RUN" = "true" ]; then
                    log_info "[DRY-RUN] Would delete $cr_type:"
                    echo "$resources" | sed 's/^/  /'
                else
                    log_warn "Deleting all $cr_type resources..."
                    echo "$resources" | while read -r resource; do
                        if [ -n "$resource" ]; then
                            log_info "Deleting $resource..."
                            $KUBECTL_CMD delete "$resource" --all-namespaces --ignore-not-found=true || true
                        fi
                    done
                fi
            else
                log_info "No $cr_type resources found"
            fi
        else
            log_info "CRD $cr_type not found or not accessible"
        fi
    done
    
    # Wait for custom resources to be deleted
    if [ "$DRY_RUN" = "false" ]; then
        log_info "Waiting for custom resources to be deleted..."
        sleep 5
    fi
    
    log_info "Custom resources removed"
}

# Remove CRDs
remove_crds() {
    if [ "$KEEP_CRDS" = "true" ]; then
        log_info "Skipping CRD removal (--keep-crds specified)"
        return
    fi
    
    log_step "Removing Custom Resource Definitions..."
    
    local crds=(
        "organizations.ovim.io"
        "virtualdatacenters.ovim.io"
        "catalogs.ovim.io"
    )
    
    for crd in "${crds[@]}"; do
        if $KUBECTL_CMD get crd "$crd" &> /dev/null; then
            if [ "$DRY_RUN" = "true" ]; then
                log_info "[DRY-RUN] Would delete CRD: $crd"
            else
                log_info "Deleting CRD: $crd..."
                $KUBECTL_CMD delete crd "$crd" --ignore-not-found=true
            fi
        else
            log_info "CRD $crd not found, skipping"
        fi
    done
    
    log_info "CRDs removed"
}

# Remove namespace
remove_namespace() {
    if [ "$KEEP_NAMESPACE" = "true" ]; then
        log_info "Skipping namespace removal (--keep-namespace specified)"
        return
    fi
    
    log_step "Removing namespace '$NAMESPACE'..."
    
    if $KUBECTL_CMD get namespace "$NAMESPACE" &> /dev/null; then
        if [ "$DRY_RUN" = "true" ]; then
            log_info "[DRY-RUN] Would delete namespace: $NAMESPACE"
        else
            log_info "Deleting namespace $NAMESPACE..."
            $KUBECTL_CMD delete namespace "$NAMESPACE" --ignore-not-found=true
            
            # Wait for namespace to be deleted
            log_info "Waiting for namespace to be deleted..."
            timeout_seconds=60
            while $KUBECTL_CMD get namespace "$NAMESPACE" &> /dev/null && [ $timeout_seconds -gt 0 ]; do
                sleep 2
                timeout_seconds=$((timeout_seconds - 2))
            done
            
            if $KUBECTL_CMD get namespace "$NAMESPACE" &> /dev/null; then
                log_warn "Namespace $NAMESPACE is still being deleted. This may take a few more minutes."
            else
                log_info "Namespace $NAMESPACE deleted successfully"
            fi
        fi
    else
        log_info "Namespace $NAMESPACE not found, skipping"
    fi
}

# Verify removal
verify_removal() {
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Skipping removal verification"
        return
    fi
    
    log_step "Verifying removal..."
    
    local remaining_resources=()
    
    # Check for remaining deployments
    if $KUBECTL_CMD get deployment ovim-controller -n "$NAMESPACE" &> /dev/null; then
        remaining_resources+=("deployment/ovim-controller")
    fi
    
    # Check for remaining RBAC
    if $KUBECTL_CMD get clusterrole ovim-controller &> /dev/null; then
        remaining_resources+=("clusterrole/ovim-controller")
    fi
    
    # Check for remaining CRDs (if not keeping them)
    if [ "$KEEP_CRDS" = "false" ]; then
        if $KUBECTL_CMD get crd organizations.ovim.io &> /dev/null; then
            remaining_resources+=("crd/organizations.ovim.io")
        fi
    fi
    
    # Check for remaining namespace (if not keeping it)
    if [ "$KEEP_NAMESPACE" = "false" ]; then
        if $KUBECTL_CMD get namespace "$NAMESPACE" &> /dev/null; then
            remaining_resources+=("namespace/$NAMESPACE")
        fi
    fi
    
    if [ ${#remaining_resources[@]} -eq 0 ]; then
        log_info "✓ All OVIM components removed successfully"
    else
        log_warn "Some resources are still being deleted:"
        for resource in "${remaining_resources[@]}"; do
            echo "  - $resource"
        done
        log_info "This is normal and they should be removed shortly"
    fi
}

# Main undeployment function
main() {
    log_info "Starting OVIM undeployment from namespace '$NAMESPACE'..."
    
    if [ "$DRY_RUN" = "true" ]; then
        log_warn "DRY-RUN mode enabled - no changes will be applied"
    fi
    
    check_kubectl
    confirm_deletion
    remove_controller
    remove_rbac
    remove_custom_resources
    remove_crds
    remove_namespace
    verify_removal
    
    log_info "OVIM undeployment completed!"
    
    if [ "$DRY_RUN" = "false" ]; then
        echo ""
        log_info "OVIM has been removed from your cluster."
        if [ "$KEEP_CRDS" = "true" ] || [ "$KEEP_NAMESPACE" = "true" ]; then
            log_info "Some components were preserved as requested."
        fi
    fi
}

# Run main function
main "$@"