#!/bin/bash

# CRD Installation Script for OVIM Org->VDC Architecture
# This script installs the Organization, VirtualDataCenter, and Catalog CRDs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBECTL_CMD="${KUBECTL_CMD:-kubectl}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

check_kubectl() {
    if ! command -v $KUBECTL_CMD &> /dev/null; then
        log_error "kubectl command not found. Please install kubectl or set KUBECTL_CMD environment variable."
        exit 1
    fi
    
    log_info "Using kubectl command: $KUBECTL_CMD"
}

check_cluster_connection() {
    log_info "Checking cluster connection..."
    if ! $KUBECTL_CMD cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubectl configuration."
        exit 1
    fi
    log_success "Connected to Kubernetes cluster"
}

install_crd() {
    local crd_file=$1
    local crd_name=$2
    
    log_info "Installing $crd_name CRD..."
    
    if $KUBECTL_CMD apply -f "$SCRIPT_DIR/$crd_file"; then
        log_success "$crd_name CRD installed successfully"
    else
        log_error "Failed to install $crd_name CRD"
        return 1
    fi
}

wait_for_crd() {
    local crd_name=$1
    log_info "Waiting for $crd_name CRD to be ready..."
    
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if $KUBECTL_CMD get crd "$crd_name" &> /dev/null; then
            log_success "$crd_name CRD is ready"
            return 0
        fi
        
        attempt=$((attempt + 1))
        log_info "Waiting... ($attempt/$max_attempts)"
        sleep 2
    done
    
    log_error "$crd_name CRD is not ready after $max_attempts attempts"
    return 1
}

validate_crd_installation() {
    local crd_name=$1
    local kind=$2
    
    log_info "Validating $kind CRD installation..."
    
    # Check if CRD exists and is ready
    if $KUBECTL_CMD get crd "$crd_name" -o jsonpath='{.status.conditions[?(@.type=="Established")].status}' | grep -q "True"; then
        log_success "$kind CRD is properly established"
    else
        log_error "$kind CRD is not properly established"
        return 1
    fi
    
    # Try to list the custom resources (should not fail even if empty)
    if $KUBECTL_CMD get "$kind" --all-namespaces &> /dev/null; then
        log_success "$kind custom resource is accessible"
    else
        log_error "Cannot access $kind custom resources"
        return 1
    fi
}

show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --dry-run         Show what would be installed without actually installing"
    echo "  --uninstall       Uninstall the CRDs (WARNING: This will delete all custom resources)"
    echo "  --validate-only   Only validate existing CRD installation"
    echo "  --help           Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  KUBECTL_CMD      kubectl command to use (default: kubectl)"
    echo ""
    echo "Examples:"
    echo "  $0                    # Install all CRDs"
    echo "  $0 --dry-run          # Show what would be installed"
    echo "  $0 --validate-only    # Validate existing installation"
    echo "  KUBECTL_CMD=oc $0     # Use OpenShift oc command"
}

uninstall_crds() {
    log_warning "Uninstalling OVIM CRDs..."
    log_warning "This will delete ALL Organization, VDC, and Catalog custom resources!"
    
    read -p "Are you sure you want to continue? (type 'yes' to confirm): " confirmation
    if [ "$confirmation" != "yes" ]; then
        log_info "Uninstall cancelled"
        exit 0
    fi
    
    # Delete custom resources first
    log_info "Deleting all custom resources..."
    $KUBECTL_CMD delete organizations.ovim.io --all --all-namespaces || true
    $KUBECTL_CMD delete virtualdatacenters.ovim.io --all --all-namespaces || true
    $KUBECTL_CMD delete catalogs.ovim.io --all --all-namespaces || true
    
    # Delete CRDs
    log_info "Deleting CRDs..."
    $KUBECTL_CMD delete crd organizations.ovim.io || true
    $KUBECTL_CMD delete crd virtualdatacenters.ovim.io || true
    $KUBECTL_CMD delete crd catalogs.ovim.io || true
    
    log_success "OVIM CRDs uninstalled"
}

validate_only() {
    log_info "Validating OVIM CRD installation..."
    
    local all_valid=true
    
    validate_crd_installation "organizations.ovim.io" "Organization" || all_valid=false
    validate_crd_installation "virtualdatacenters.ovim.io" "VirtualDataCenter" || all_valid=false
    validate_crd_installation "catalogs.ovim.io" "Catalog" || all_valid=false
    
    if [ "$all_valid" = true ]; then
        log_success "All OVIM CRDs are properly installed and accessible"
    else
        log_error "Some OVIM CRDs are not properly installed"
        exit 1
    fi
}

main() {
    local dry_run=false
    local uninstall=false
    local validate_only_mode=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                dry_run=true
                shift
                ;;
            --uninstall)
                uninstall=true
                shift
                ;;
            --validate-only)
                validate_only_mode=true
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Banner
    echo "=============================================="
    echo "OVIM CRD Installation Script"
    echo "Org->VDC Architecture with Flat Namespaces"
    echo "=============================================="
    echo ""
    
    check_kubectl
    check_cluster_connection
    
    if [ "$validate_only_mode" = true ]; then
        validate_only
        exit 0
    fi
    
    if [ "$uninstall" = true ]; then
        uninstall_crds
        exit 0
    fi
    
    if [ "$dry_run" = true ]; then
        log_info "DRY RUN: The following CRDs would be installed:"
        echo "  - Organization CRD (organizations.ovim.io)"
        echo "  - VirtualDataCenter CRD (virtualdatacenters.ovim.io)"
        echo "  - Catalog CRD (catalogs.ovim.io)"
        echo ""
        log_info "Files that would be applied:"
        echo "  - $SCRIPT_DIR/organization.yaml"
        echo "  - $SCRIPT_DIR/virtualdatacenter.yaml"
        echo "  - $SCRIPT_DIR/catalog.yaml"
        exit 0
    fi
    
    log_info "Installing OVIM CRDs..."
    
    # Install CRDs in order
    install_crd "organization.yaml" "Organization"
    wait_for_crd "organizations.ovim.io"
    
    install_crd "virtualdatacenter.yaml" "VirtualDataCenter"
    wait_for_crd "virtualdatacenters.ovim.io"
    
    install_crd "catalog.yaml" "Catalog"
    wait_for_crd "catalogs.ovim.io"
    
    log_info "Validating installation..."
    validate_only
    
    echo ""
    log_success "All OVIM CRDs installed successfully!"
    echo ""
    echo "Next steps:"
    echo "1. Deploy the OVIM controllers to manage these CRDs"
    echo "2. Run the database migration scripts"
    echo "3. Update your application to use the new CRD-based models"
    echo ""
    echo "You can now create Organization, VDC, and Catalog resources:"
    echo "  $KUBECTL_CMD get organizations"
    echo "  $KUBECTL_CMD get virtualdatacenters"
    echo "  $KUBECTL_CMD get catalogs"
}

# Run main function with all arguments
main "$@"