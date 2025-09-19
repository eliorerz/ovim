#!/bin/bash

# OVIM Full Stack Deployment Script
# Deploys PostgreSQL, OVIM Controllers, UI, and Ingress
# Usage: ./scripts/deploy-stack.sh [OPTIONS]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default stack configuration
DEFAULT_NAMESPACE="${OVIM_NAMESPACE:-ovim-system}"
DEFAULT_IMAGE_TAG="${OVIM_IMAGE_TAG:-latest}"
DEFAULT_KUBECTL="${KUBECTL_CMD:-kubectl}"
DEFAULT_DB_STORAGE_SIZE="${OVIM_DB_STORAGE_SIZE:-10Gi}"
DEFAULT_UI_REPLICAS="${OVIM_UI_REPLICAS:-2}"
DEFAULT_DOMAIN="${OVIM_DOMAIN:-ovim.local}"
DEFAULT_CONTROLLER_IMAGE="${OVIM_CONTROLLER_IMAGE:-quay.io/eerez/ovim}"
DEFAULT_SERVER_IMAGE="${OVIM_SERVER_IMAGE:-quay.io/eerez/ovim}"
DEFAULT_UI_IMAGE="${OVIM_UI_IMAGE:-quay.io/eerez/ovim-ui}"

# Configuration variables
NAMESPACE="$DEFAULT_NAMESPACE"
IMAGE_TAG="$DEFAULT_IMAGE_TAG"
KUBECTL_CMD="$DEFAULT_KUBECTL"
DB_STORAGE_SIZE="$DEFAULT_DB_STORAGE_SIZE"
UI_REPLICAS="$DEFAULT_UI_REPLICAS"
DOMAIN="$DEFAULT_DOMAIN"
CONTROLLER_IMAGE="$DEFAULT_CONTROLLER_IMAGE"
SERVER_IMAGE="$DEFAULT_SERVER_IMAGE"
UI_IMAGE="$DEFAULT_UI_IMAGE"
USE_UNIQUE_TAG="false"
DRY_RUN="false"
SKIP_DATABASE="false"
SKIP_UI="false"
SKIP_INGRESS="false"
SKIP_CONTROLLER="false"
SKIP_SERVER="false"
SKIP_BUILD="false"
VERBOSE="false"
WAIT_FOR_READY="true"
BUILD_UI="true"

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

log_debug() {
    if [ "$VERBOSE" = "true" ]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

show_help() {
    cat << EOF
OVIM Full Stack Deployment Script

USAGE:
    ./scripts/deploy-stack.sh [OPTIONS]

OPTIONS:
    --namespace NAMESPACE       Kubernetes namespace (default: $DEFAULT_NAMESPACE)
    --image-tag TAG            Docker image tag (default: $DEFAULT_IMAGE_TAG)
    --domain DOMAIN            Ingress domain (default: $DEFAULT_DOMAIN)
    --db-storage-size SIZE     Database storage size (default: $DEFAULT_DB_STORAGE_SIZE)
    --ui-replicas NUM          UI replica count (default: $DEFAULT_UI_REPLICAS)
    --kubectl CMD              kubectl command (default: $DEFAULT_KUBECTL)
    --controller-image IMAGE   OVIM controller image (default: $DEFAULT_CONTROLLER_IMAGE)
    --server-image IMAGE       OVIM server image (default: $DEFAULT_SERVER_IMAGE)  
    --ui-image IMAGE           OVIM UI image (default: $DEFAULT_UI_IMAGE)
    --use-unique-tag           Use unique timestamp-git tag instead of latest
    --dry-run                  Show what would be deployed
    --skip-database            Skip PostgreSQL deployment
    --skip-ui                  Skip UI deployment
    --skip-ingress             Skip Ingress deployment
    --skip-controller          Skip OVIM controller deployment
    --skip-server              Skip OVIM server deployment
    --skip-build               Skip building binaries and UI
    --no-build-ui              Don't build UI (use existing image)
    --no-wait                  Don't wait for components to be ready
    --verbose                  Enable verbose output
    --help                     Show this help

ENVIRONMENT VARIABLES:
    OVIM_NAMESPACE             Default namespace
    OVIM_IMAGE_TAG             Default image tag
    OVIM_DOMAIN                Default domain
    OVIM_DB_STORAGE_SIZE       Default database storage size
    OVIM_UI_REPLICAS           Default UI replicas
    DATABASE_URL               Database URL (for migration)

EXAMPLES:
    # Deploy complete stack
    ./scripts/deploy-stack.sh

    # Deploy to custom domain
    ./scripts/deploy-stack.sh --domain ovim.example.com

    # Development deployment
    ./scripts/deploy-stack.sh --namespace ovim-dev --image-tag dev

    # Skip database (use external)
    ./scripts/deploy-stack.sh --skip-database

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --image-tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        --domain)
            DOMAIN="$2"
            shift 2
            ;;
        --db-storage-size)
            DB_STORAGE_SIZE="$2"
            shift 2
            ;;
        --ui-replicas)
            UI_REPLICAS="$2"
            shift 2
            ;;
        --kubectl)
            KUBECTL_CMD="$2"
            shift 2
            ;;
        --controller-image)
            CONTROLLER_IMAGE="$2"
            shift 2
            ;;
        --server-image)
            SERVER_IMAGE="$2"
            shift 2
            ;;
        --ui-image)
            UI_IMAGE="$2"
            shift 2
            ;;
        --use-unique-tag)
            USE_UNIQUE_TAG="true"
            shift
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        --skip-database)
            SKIP_DATABASE="true"
            shift
            ;;
        --skip-ui)
            SKIP_UI="true"
            shift
            ;;
        --skip-ingress)
            SKIP_INGRESS="true"
            shift
            ;;
        --skip-controller)
            SKIP_CONTROLLER="true"
            shift
            ;;
        --skip-server)
            SKIP_SERVER="true"
            shift
            ;;
        --skip-build)
            SKIP_BUILD="true"
            shift
            ;;
        --no-build-ui)
            BUILD_UI="false"
            shift
            ;;
        --no-wait)
            WAIT_FOR_READY="false"
            shift
            ;;
        --verbose)
            VERBOSE="true"
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

# Generate database password if not provided
generate_db_password() {
    if command -v openssl &> /dev/null; then
        openssl rand -base64 32 | tr -d "=+/" | cut -c1-25
    else
        # Fallback for systems without openssl
        LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 25
    fi
}

# Update manifests with configuration
update_manifests() {
    # Log to stderr so it doesn't interfere with return value
    log_step "Updating manifests with configuration..." >&2
    
    local temp_dir
    temp_dir=$(mktemp -d)
    
    # Copy manifests to temp directory for modification
    cp -r "$PROJECT_ROOT/config" "$temp_dir/"
    
    # Update namespace in all manifests
    find "$temp_dir/config" -name "*.yaml" -type f -exec sed -i "s/namespace: ovim-system/namespace: $NAMESPACE/g" {} \;
    
    # Update database storage size
    sed -i "s/storage: 10Gi/storage: $DB_STORAGE_SIZE/g" "$temp_dir/config/database/postgresql.yaml"
    
    # Update UI replicas
    sed -i "s/replicas: 2/replicas: $UI_REPLICAS/g" "$temp_dir/config/ui/ovim-ui.yaml"
    
    # Update ingress domain
    sed -i "s/ovim\\.local/$DOMAIN/g" "$temp_dir/config/ingress/ovim-ingress.yaml"
    
    # Update UI image reference
    sed -i "s|image: quay.io/eerez/ovim-ui:latest|image: $UI_IMAGE:$IMAGE_TAG|g" "$temp_dir/config/ui/ovim-ui.yaml"

    # Dynamically detect and update storage class for PostgreSQL
    log_debug "Detecting available storage classes..." >&2
    local default_storage_class
    default_storage_class=$($KUBECTL_CMD get storageclass -o jsonpath='{.items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")].metadata.name}' 2>/dev/null || echo "")

    if [ -z "$default_storage_class" ]; then
        # If no default storage class, get the first available one
        default_storage_class=$($KUBECTL_CMD get storageclass -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    fi

    if [ -n "$default_storage_class" ]; then
        log_debug "Using storage class: $default_storage_class" >&2
        # Add storageClassName to the PostgreSQL PVC template
        sed -i '/accessModes:/i\        storageClassName: '"$default_storage_class" "$temp_dir/config/database/postgresql.yaml"
    else
        log_warn "No storage class found. PostgreSQL PVC may fail to bind." >&2
    fi

    # Output temp directory path to stdout only
    printf "%s" "$temp_dir"
}

# Ensure namespace exists
ensure_namespace() {
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would ensure namespace $NAMESPACE exists"
        return
    fi
    
    log_debug "Ensuring namespace $NAMESPACE exists..."
    $KUBECTL_CMD create namespace "$NAMESPACE" --dry-run=client -o yaml | $KUBECTL_CMD apply -f - >/dev/null 2>&1
}

# Build components
build_components() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_info "Skipping build (--skip-build specified)"
        return
    fi
    
    log_step "Building OVIM components..."
    
    cd "$PROJECT_ROOT"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would build OVIM controller and server"
        if [ "$BUILD_UI" = "true" ] && [ "$SKIP_UI" = "false" ]; then
            log_info "[DRY-RUN] Would build UI"
        fi
        return
    fi
    
    # Build controller and server
    log_debug "Building OVIM binaries..."
    make build build-controller
    
    # Build UI if requested
    if [ "$BUILD_UI" = "true" ] && [ "$SKIP_UI" = "false" ]; then
        log_debug "Building UI..."
        cd "$PROJECT_ROOT/../ovim-ui"
        make container-build
        cd "$PROJECT_ROOT"
    fi
    
    log_info "Components built successfully"
}

# Deploy database
deploy_database() {
    if [ "$SKIP_DATABASE" = "true" ]; then
        log_info "Skipping database deployment (--skip-database specified)"
        return
    fi
    
    log_step "Deploying PostgreSQL database..."
    
    local manifests_dir="$1/config/database"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would deploy PostgreSQL from: $manifests_dir"
        return
    fi
    
    # Ensure namespace exists
    ensure_namespace
    
    # Deploy PostgreSQL
    $KUBECTL_CMD apply -f "$manifests_dir/"
    
    if [ "$WAIT_FOR_READY" = "true" ]; then
        log_debug "Waiting for PostgreSQL to be ready..."
        # Wait for StatefulSet to be ready instead of individual pods
        $KUBECTL_CMD wait --for=condition=ready statefulset/ovim-postgresql -n "$NAMESPACE" --timeout=300s || {
            log_warn "PostgreSQL StatefulSet not ready yet, but continuing deployment..."
        }
    fi
    
    log_info "PostgreSQL deployed successfully"
}

# Deploy OVIM controller
deploy_controller() {
    if [ "$SKIP_CONTROLLER" = "true" ]; then
        log_info "Skipping controller deployment (--skip-controller specified)"
        return
    fi
    
    log_step "Deploying OVIM controller..."
    
    # Use existing deploy.sh script for controller deployment
    local deploy_args=""
    if [ "$DRY_RUN" = "true" ]; then
        deploy_args="--dry-run"
    fi
    
    OVIM_NAMESPACE="$NAMESPACE" \
    OVIM_IMAGE_TAG="$IMAGE_TAG" \
    OVIM_CONTROLLER_IMAGE="$CONTROLLER_IMAGE" \
    OVIM_SERVER_IMAGE="$SERVER_IMAGE" \
    KUBECTL_CMD="$KUBECTL_CMD" \
    "$SCRIPT_DIR/deploy.sh" $deploy_args --skip-db-migration
    
    log_info "OVIM controller deployed successfully"
}

# Deploy UI
deploy_server() {
    if [ "$SKIP_SERVER" = "true" ]; then
        log_info "Skipping server deployment (--skip-server specified)"
        return
    fi
    
    log_step "Deploying OVIM server..."
    
    local manifests_dir="$1/config/server"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would deploy server from: $manifests_dir"
        return
    fi
    
    # Deploy server
    $KUBECTL_CMD apply -f "$manifests_dir/"
    
    if [ "$WAIT_FOR_READY" = "true" ]; then
        log_debug "Waiting for server to be ready..."
        $KUBECTL_CMD wait --for=condition=available deployment/ovim-server -n "$NAMESPACE" --timeout=300s
    fi
    
    log_info "Server deployed successfully"
}

deploy_ui() {
    if [ "$SKIP_UI" = "true" ]; then
        log_info "Skipping UI deployment (--skip-ui specified)"
        return
    fi
    
    log_step "Deploying OVIM UI..."
    
    # Check if server is available (unless skipped)
    if [ "$SKIP_SERVER" = "false" ]; then
        if ! $KUBECTL_CMD get deployment ovim-server -n "$NAMESPACE" &> /dev/null; then
            log_warn "OVIM server not found. UI may not function properly without backend."
            log_info "Consider deploying server first: make deploy-server"
        else
            log_debug "Server deployment found, proceeding with UI deployment"
        fi
    fi
    
    local manifests_dir="$1/config/ui"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would deploy UI from: $manifests_dir"
        return
    fi
    
    # Deploy UI
    $KUBECTL_CMD apply -f "$manifests_dir/"
    
    if [ "$WAIT_FOR_READY" = "true" ]; then
        log_debug "Waiting for UI to be ready..."
        $KUBECTL_CMD wait --for=condition=available deployment/ovim-ui -n "$NAMESPACE" --timeout=300s
    fi
    
    log_info "UI deployed successfully"
}

# Deploy ingress or routes based on platform
deploy_ingress() {
    if [ "$SKIP_INGRESS" = "true" ]; then
        log_info "Skipping ingress deployment (--skip-ingress specified)"
        return
    fi
    
    # Check if we're running on OpenShift
    if $KUBECTL_CMD api-resources --api-group=route.openshift.io &> /dev/null; then
        log_step "Deploying OpenShift Routes..."
        deploy_routes "$1"
    else
        log_step "Deploying Kubernetes Ingress..."
        deploy_k8s_ingress "$1"
    fi
}

# Deploy OpenShift Routes
deploy_routes() {
    local manifests_dir="$1/config/routes"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would deploy OpenShift Routes from: $manifests_dir"
        return
    fi
    
    # Check if routes directory exists
    if [ ! -d "$manifests_dir" ]; then
        log_warn "Routes directory not found: $manifests_dir"
        log_warn "Falling back to manual route creation..."
        
        # Create routes manually if manifests don't exist
        log_debug "Creating route for UI service..."
        $KUBECTL_CMD expose service ovim-ui --hostname="ovim-ui-$NAMESPACE.apps-crc.testing" -n "$NAMESPACE" || true
        
        log_debug "Creating route for server service..."
        $KUBECTL_CMD expose service ovim-server --hostname="ovim-server-$NAMESPACE.apps-crc.testing" -n "$NAMESPACE" || true
    else
        # Deploy routes from manifests
        $KUBECTL_CMD apply -f "$manifests_dir/"
    fi
    
    log_info "OpenShift Routes deployed successfully"
}

# Deploy Kubernetes Ingress (original function)
deploy_k8s_ingress() {
    local manifests_dir="$1/config/ingress"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would deploy Kubernetes Ingress from: $manifests_dir"
        return
    fi
    
    # Check if ingress controller is available
    if ! $KUBECTL_CMD get ingressclass nginx &> /dev/null; then
        log_warn "Nginx ingress controller not found. Ingress may not work properly."
        log_warn "Install with: kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml"
    fi
    
    # Deploy ingress
    $KUBECTL_CMD apply -f "$manifests_dir/"
    
    log_info "Kubernetes Ingress deployed successfully"
}

# Run database migration
run_migration() {
    if [ "$SKIP_DATABASE" = "true" ]; then
        log_info "Skipping database migration (database not deployed)"
        return
    fi
    
    log_step "Running database migration..."
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would run database migration"
        return
    fi
    
    # Port forward to database for migration
    log_debug "Setting up port forward for database migration..."
    $KUBECTL_CMD port-forward svc/ovim-postgresql 5432:5432 -n "$NAMESPACE" &
    local pf_pid=$!
    
    # Wait for port forward to be ready
    sleep 5
    
    # Run migration
    local db_url="postgres://ovim:ovim123@localhost:5432/ovim?sslmode=disable"
    DATABASE_URL="$db_url" make -C "$PROJECT_ROOT" db-migrate || {
        kill $pf_pid 2>/dev/null || true
        log_warn "Database migration failed. You may need to run it manually."
        return
    }
    
    # Clean up port forward
    kill $pf_pid 2>/dev/null || true
    
    log_info "Database migration completed"
}

# Verify deployment
verify_stack() {
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Skipping verification"
        return
    fi
    
    log_step "Verifying stack deployment..."
    
    local issues=0
    
    # Check namespace
    if ! $KUBECTL_CMD get namespace "$NAMESPACE" &> /dev/null; then
        log_error "Namespace $NAMESPACE not found"
        ((issues++))
    fi
    
    # Check database
    if [ "$SKIP_DATABASE" = "false" ]; then
        if ! $KUBECTL_CMD get statefulset ovim-postgresql -n "$NAMESPACE" &> /dev/null; then
            log_error "PostgreSQL StatefulSet not found"
            ((issues++))
        fi
    fi
    
    # Check controller
    if [ "$SKIP_CONTROLLER" = "false" ]; then
        if ! $KUBECTL_CMD get deployment ovim-controller -n "$NAMESPACE" &> /dev/null; then
            log_error "OVIM controller deployment not found"
            ((issues++))
        fi
    fi
    
    # Check UI
    if [ "$SKIP_UI" = "false" ]; then
        if ! $KUBECTL_CMD get deployment ovim-ui -n "$NAMESPACE" &> /dev/null; then
            log_error "UI deployment not found"
            ((issues++))
        fi
    fi
    
    # Check ingress or routes based on platform
    if [ "$SKIP_INGRESS" = "false" ]; then
        if $KUBECTL_CMD api-resources --api-group=route.openshift.io &> /dev/null; then
            # OpenShift - check for routes
            if ! $KUBECTL_CMD get route -n "$NAMESPACE" &> /dev/null; then
                log_error "OpenShift routes not found"
                ((issues++))
            fi
        else
            # Kubernetes - check for ingress
            if ! $KUBECTL_CMD get ingress ovim-ingress -n "$NAMESPACE" &> /dev/null; then
                log_error "Kubernetes ingress not found"
                ((issues++))
            fi
        fi
    fi
    
    if [ $issues -eq 0 ]; then
        log_info "✓ Stack verification completed successfully"
    else
        log_warn "Stack verification found $issues issues"
    fi
}

# Check network connectivity and ports
check_network_services() {
    if [ "$DRY_RUN" = "true" ]; then
        return
    fi
    
    log_step "Checking network services and connectivity..."
    
    local issues=0
    
    # Check if services are created and have endpoints
    log_debug "Checking services..."
    
    # Check UI service
    if ! $KUBECTL_CMD get service ovim-ui -n "$NAMESPACE" &> /dev/null; then
        log_warn "UI service not found"
        ((issues++))
    else
        # Check if service has endpoints
        local ui_endpoints
        ui_endpoints=$($KUBECTL_CMD get endpoints ovim-ui -n "$NAMESPACE" -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || echo "")
        if [ -z "$ui_endpoints" ]; then
            log_warn "UI service has no endpoints (pods may not be ready)"
            ((issues++))
        else
            log_debug "✓ UI service has endpoints: $ui_endpoints"
        fi
    fi
    
    # Check controller service
    if ! $KUBECTL_CMD get service ovim-server -n "$NAMESPACE" &> /dev/null; then
        log_warn "Controller service not found"
        ((issues++))
    else
        local controller_endpoints
        controller_endpoints=$($KUBECTL_CMD get endpoints ovim-server -n "$NAMESPACE" -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || echo "")
        if [ -z "$controller_endpoints" ]; then
            log_warn "Controller service has no endpoints (pods may not be ready)"
            ((issues++))
        else
            log_debug "✓ Controller service has endpoints: $controller_endpoints"
        fi
    fi
    
    # Check database service
    if [ "$SKIP_DATABASE" = "false" ]; then
        if ! $KUBECTL_CMD get service ovim-postgresql -n "$NAMESPACE" &> /dev/null; then
            log_warn "Database service not found"
            ((issues++))
        else
            local db_endpoints
            db_endpoints=$($KUBECTL_CMD get endpoints ovim-postgresql -n "$NAMESPACE" -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || echo "")
            if [ -z "$db_endpoints" ]; then
                log_warn "Database service has no endpoints (pod may not be ready)"
                ((issues++))
            else
                log_debug "✓ Database service has endpoints: $db_endpoints"
            fi
        fi
    fi
    
    # Check ingress or routes based on platform
    if [ "$SKIP_INGRESS" = "false" ]; then
        if $KUBECTL_CMD api-resources --api-group=route.openshift.io &> /dev/null; then
            # OpenShift - check for routes
            if ! $KUBECTL_CMD get route -n "$NAMESPACE" &> /dev/null; then
                log_warn "OpenShift routes not found"
                ((issues++))
            else
                log_debug "✓ OpenShift routes configured"
            fi
        else
            # Kubernetes - check for ingress
            if ! $KUBECTL_CMD get ingress ovim-ingress -n "$NAMESPACE" &> /dev/null; then
                log_warn "Kubernetes ingress not found"
                ((issues++))
            else
                log_debug "✓ Kubernetes ingress configured"
            fi
        fi
    fi
    
    if [ $issues -eq 0 ]; then
        log_info "✓ All network services are properly configured"
    else
        log_warn "Found $issues network service issues (may resolve as pods become ready)"
    fi
}

# Get access URLs and connection information
get_access_info() {
    local access_method=""
    local ui_url=""
    local api_url=""
    local notes=""
    
    # Check for OpenShift Routes first
    if [ "$SKIP_INGRESS" = "false" ] && $KUBECTL_CMD api-resources --api-group=route.openshift.io &> /dev/null; then
        # Check if routes exist
        local ui_route_host
        local server_route_host
        
        ui_route_host=$($KUBECTL_CMD get route ovim-ui -n "$NAMESPACE" -o jsonpath='{.spec.host}' 2>/dev/null || echo "")
        server_route_host=$($KUBECTL_CMD get route ovim-server -n "$NAMESPACE" -o jsonpath='{.spec.host}' 2>/dev/null || echo "")
        
        if [ -n "$ui_route_host" ] && [ -n "$server_route_host" ]; then
            access_method="openshift-routes"
            ui_url="https://$ui_route_host"
            api_url="https://$server_route_host"
            notes="Routes created automatically"
        else
            access_method="routes-pending"
            ui_url="https://ovim-ui-$NAMESPACE.apps-crc.testing (pending)"
            api_url="https://ovim-server-$NAMESPACE.apps-crc.testing (pending)"
            notes="Routes may still be creating. Check: oc get routes -n $NAMESPACE"
        fi
    # Check for Kubernetes ingress
    elif [ "$SKIP_INGRESS" = "false" ] && $KUBECTL_CMD get ingress ovim-ingress -n "$NAMESPACE" &> /dev/null; then
        local ingress_ip
        local ingress_hostname
        
        # Get ingress IP
        ingress_ip=$($KUBECTL_CMD get ingress ovim-ingress -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")
        
        # Get ingress hostname (for cloud providers that provide hostnames)
        if [ -z "$ingress_ip" ]; then
            ingress_hostname=$($KUBECTL_CMD get ingress ovim-ingress -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || echo "")
        fi
        
        if [ -n "$ingress_ip" ]; then
            access_method="ingress-ip"
            ui_url="https://$DOMAIN"
            api_url="https://$DOMAIN/api"
            notes="Add to /etc/hosts: $ingress_ip $DOMAIN"
        elif [ -n "$ingress_hostname" ]; then
            access_method="ingress-hostname"
            ui_url="https://$DOMAIN"
            api_url="https://$DOMAIN/api"
            notes="DNS points to: $ingress_hostname"
        else
            access_method="ingress-pending"
            ui_url="https://$DOMAIN (pending)"
            api_url="https://$DOMAIN/api (pending)"
            notes="Ingress IP pending. Check: kubectl get ingress -n $NAMESPACE"
        fi
    else
        # Fallback to NodePort or port-forwarding
        access_method="port-forward"
        ui_url="https://localhost:8443 (via port-forward)"
        api_url="https://localhost:8444 (via port-forward)"
        notes="Use: make stack-port-forward"
    fi
    
    echo "$access_method|$ui_url|$api_url|$notes"
}

# Show deployment info
show_deployment_info() {
    if [ "$DRY_RUN" = "true" ]; then
        return
    fi
    
    log_step "🎉 OVIM Stack Deployment Complete!"
    echo ""
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│                    Deployment Summary                       │"
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
    echo "📦 Configuration:"
    echo "   Namespace: $NAMESPACE"
    echo "   Domain: $DOMAIN"
    echo "   Image Tag: $IMAGE_TAG"
    echo "   Database Storage: $DB_STORAGE_SIZE"
    echo "   UI Replicas: $UI_REPLICAS"
    echo ""
    
    # Get access information
    local access_info
    access_info=$(get_access_info)
    local access_method
    local ui_url
    local api_url
    local notes
    
    IFS='|' read -r access_method ui_url api_url notes <<< "$access_info"
    
    echo "🌐 Access Information:"
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│  🖥️  OVIM Web UI:  $ui_url"
    echo "│  🔌 OVIM API:     $api_url"
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
    
    if [ "$access_method" = "ingress-ip" ] || [ "$access_method" = "ingress-hostname" ]; then
        echo "✅ Network Setup: READY"
        echo "   Your OVIM instance is accessible via ingress"
        if [ "$access_method" = "ingress-ip" ]; then
            echo "   $notes"
        fi
    elif [ "$access_method" = "ingress-pending" ]; then
        echo "⏳ Network Setup: PENDING"
        echo "   $notes"
        echo "   Alternative: Use port-forwarding below"
    else
        echo "🔧 Network Setup: PORT-FORWARDING REQUIRED"
        echo "   $notes"
    fi
    echo ""
    
    # Show port forwarding instructions
    echo "🔗 Local Access (Port Forwarding):"
    echo "   Run: make stack-port-forward"
    echo "   Then access:"
    echo "   • Web UI: https://localhost:8443"
    echo "   • API: https://localhost:8444"
    echo "   • Database: localhost:5432"
    echo ""
    
    # Show component status
    echo "📊 Component Status:"
    local ui_status="❓"
    local controller_status="❓"
    local db_status="❓"
    local ingress_status="❓"
    
    # Check UI
    if $KUBECTL_CMD get deployment ovim-ui -n "$NAMESPACE" &> /dev/null; then
        local ui_ready
        ui_ready=$($KUBECTL_CMD get deployment ovim-ui -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        local ui_total
        ui_total=$($KUBECTL_CMD get deployment ovim-ui -n "$NAMESPACE" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
        if [ "$ui_ready" = "$ui_total" ] && [ "$ui_ready" != "0" ]; then
            ui_status="✅ Running ($ui_ready/$ui_total)"
        else
            ui_status="⏳ Starting ($ui_ready/$ui_total)"
        fi
    else
        ui_status="❌ Not deployed"
    fi
    
    # Check controller
    if $KUBECTL_CMD get deployment ovim-controller -n "$NAMESPACE" &> /dev/null; then
        local controller_ready
        controller_ready=$($KUBECTL_CMD get deployment ovim-controller -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        if [ "$controller_ready" = "1" ]; then
            controller_status="✅ Running"
        else
            controller_status="⏳ Starting"
        fi
    else
        controller_status="❌ Not deployed"
    fi
    
    # Check server
    if [ "$SKIP_SERVER" = "false" ]; then
        if $KUBECTL_CMD get deployment ovim-server -n "$NAMESPACE" &> /dev/null; then
            local server_ready server_total
            server_ready=$($KUBECTL_CMD get deployment ovim-server -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
            server_total=$($KUBECTL_CMD get deployment ovim-server -n "$NAMESPACE" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
            if [ "$server_ready" = "$server_total" ] && [ "$server_ready" != "0" ]; then
                server_status="✅ Running ($server_ready/$server_total)"
            else
                server_status="⏳ Starting ($server_ready/$server_total)"
            fi
        else
            server_status="❌ Not deployed"
        fi
    else
        server_status="⏭️ Skipped"
    fi
    
    # Check database
    if [ "$SKIP_DATABASE" = "false" ]; then
        if $KUBECTL_CMD get statefulset ovim-postgresql -n "$NAMESPACE" &> /dev/null; then
            local db_ready
            db_ready=$($KUBECTL_CMD get statefulset ovim-postgresql -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
            if [ "$db_ready" = "1" ]; then
                db_status="✅ Running"
            else
                db_status="⏳ Starting"
            fi
        else
            db_status="❌ Not deployed"
        fi
    else
        db_status="⏭️ Skipped"
    fi
    
    # Check ingress
    if [ "$SKIP_INGRESS" = "false" ]; then
        if $KUBECTL_CMD get ingress ovim-ingress -n "$NAMESPACE" &> /dev/null; then
            ingress_status="✅ Configured"
        else
            ingress_status="❌ Not deployed"
        fi
    else
        ingress_status="⏭️ Skipped"
    fi
    
    echo "   • UI:         $ui_status"
    echo "   • Server:     $server_status"
    echo "   • Controller: $controller_status" 
    echo "   • Database:   $db_status"
    echo "   • Ingress:    $ingress_status"
    echo ""
    
    # Show next steps
    echo "🚀 Next Steps:"
    echo "   1. Wait for all components to be ready:"
    echo "      kubectl get pods -n $NAMESPACE -w"
    echo ""
    echo "   2. Check deployment status:"
    echo "      make stack-status"
    echo ""
    echo "   3. Deploy sample resources:"
    echo "      make deploy-samples"
    echo ""
    echo "   4. View logs:"
    echo "      make stack-logs"
    echo ""
    
    # Show troubleshooting info if there are issues
    local all_ready=true
    if [[ "$ui_status" == *"Starting"* ]] || [[ "$controller_status" == *"Starting"* ]] || [[ "$db_status" == *"Starting"* ]]; then
        all_ready=false
    fi
    
    if [ "$all_ready" = "false" ]; then
        echo "🔧 Troubleshooting:"
        echo "   • Check pod status: kubectl get pods -n $NAMESPACE"
        echo "   • View events: kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp'"
        echo "   • Check logs: kubectl logs -f deployment/ovim-controller -n $NAMESPACE"
        echo ""
    fi
    
    echo "📚 Documentation: $PROJECT_ROOT/DEPLOYMENT.md"
    echo ""
}

# Main deployment function
main() {
    log_info "Starting OVIM full stack deployment..."
    log_info "Namespace: $NAMESPACE, Domain: $DOMAIN, Image Tag: $IMAGE_TAG"
    
    # Handle unique tag option
    if [ "$USE_UNIQUE_TAG" = "true" ]; then
        if [ -n "${UNIQUE_TAG:-}" ]; then
            IMAGE_TAG="$UNIQUE_TAG"
            log_info "Using unique tag: $IMAGE_TAG"
        else
            # Generate unique tag if not provided
            local timestamp
            timestamp=$(date -u +'%Y%m%d-%H%M%S')
            local git_short
            git_short=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
            IMAGE_TAG="$timestamp-$git_short"
            log_info "Generated unique tag: $IMAGE_TAG"
        fi
    fi
    
    log_debug "Controller Image: $CONTROLLER_IMAGE:$IMAGE_TAG"
    log_debug "Server Image: $SERVER_IMAGE:$IMAGE_TAG"
    log_debug "UI Image: $UI_IMAGE:$IMAGE_TAG"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_warn "DRY-RUN mode enabled - no changes will be applied"
    fi
    
    # Create temporary manifests with updated configuration
    local manifests_dir
    manifests_dir=$(update_manifests)
    
    # Ensure cleanup of temp directory
    trap "rm -rf '$manifests_dir'" EXIT
    
    # Deploy components
    ensure_namespace
    build_components
    deploy_database "$manifests_dir"
    deploy_controller
    deploy_server "$manifests_dir"
    deploy_ui "$manifests_dir"
    deploy_ingress "$manifests_dir"
    run_migration
    verify_stack
    check_network_services
    show_deployment_info
    
    log_info "OVIM full stack deployment completed!"
}

# Run main function
main "$@"