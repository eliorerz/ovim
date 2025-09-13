#!/bin/bash

# OVIM Deployment Script
# Comprehensive deployment for OVIM controllers, CRDs, and all dependencies
# Usage: ./scripts/deploy.sh [--namespace NAMESPACE] [--image-tag TAG] [--dry-run] [--help]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default deployment configuration
DEFAULT_NAMESPACE="${OVIM_NAMESPACE:-ovim-system}"
DEFAULT_IMAGE_TAG="${OVIM_IMAGE_TAG:-latest}"
DEFAULT_KUBECTL="${KUBECTL_CMD:-kubectl}"
DEFAULT_CONTROLLER_IMAGE="${OVIM_CONTROLLER_IMAGE:-ovim-controller}"
DEFAULT_SERVER_IMAGE="${OVIM_SERVER_IMAGE:-ovim-server}"

# Configuration variables (can be overridden)
NAMESPACE="$DEFAULT_NAMESPACE"
IMAGE_TAG="$DEFAULT_IMAGE_TAG"
KUBECTL_CMD="$DEFAULT_KUBECTL"
CONTROLLER_IMAGE="$DEFAULT_CONTROLLER_IMAGE"
SERVER_IMAGE="$DEFAULT_SERVER_IMAGE"
DRY_RUN="false"
SKIP_BUILD="false"
SKIP_CRD="false"
SKIP_RBAC="false"
SKIP_CONTROLLER="false"
SKIP_DB_MIGRATION="false"
VERBOSE="false"

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
OVIM Deployment Script

USAGE:
    ./scripts/deploy.sh [OPTIONS]

OPTIONS:
    --namespace NAMESPACE       Kubernetes namespace for deployment (default: $DEFAULT_NAMESPACE)
    --image-tag TAG            Docker image tag (default: $DEFAULT_IMAGE_TAG)
    --controller-image IMAGE   Controller image name (default: $DEFAULT_CONTROLLER_IMAGE)
    --server-image IMAGE       Server image name (default: $DEFAULT_SERVER_IMAGE)
    --kubectl CMD              kubectl command to use (default: $DEFAULT_KUBECTL)
    --dry-run                  Show what would be deployed without applying
    --skip-build               Skip building binaries
    --skip-crd                 Skip CRD installation
    --skip-rbac                Skip RBAC setup
    --skip-controller          Skip controller deployment
    --skip-db-migration        Skip database migration
    --verbose                  Enable verbose output
    --help                     Show this help message

ENVIRONMENT VARIABLES:
    OVIM_NAMESPACE             Default namespace (default: ovim-system)
    OVIM_IMAGE_TAG             Default image tag (default: latest)
    OVIM_CONTROLLER_IMAGE      Controller image name
    OVIM_SERVER_IMAGE          Server image name
    KUBECTL_CMD                kubectl command to use
    DATABASE_URL               Database connection string (required for migration)

EXAMPLES:
    # Deploy to default namespace
    ./scripts/deploy.sh

    # Deploy to custom namespace with specific tag
    ./scripts/deploy.sh --namespace my-ovim --image-tag v1.0.0

    # Dry run to see what would be deployed
    ./scripts/deploy.sh --dry-run

    # Skip building and only deploy manifests
    ./scripts/deploy.sh --skip-build --image-tag existing-tag

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
        --controller-image)
            CONTROLLER_IMAGE="$2"
            shift 2
            ;;
        --server-image)
            SERVER_IMAGE="$2"
            shift 2
            ;;
        --kubectl)
            KUBECTL_CMD="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        --skip-build)
            SKIP_BUILD="true"
            shift
            ;;
        --skip-crd)
            SKIP_CRD="true"
            shift
            ;;
        --skip-rbac)
            SKIP_RBAC="true"
            shift
            ;;
        --skip-controller)
            SKIP_CONTROLLER="true"
            shift
            ;;
        --skip-db-migration)
            SKIP_DB_MIGRATION="true"
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

# Validate dependencies
check_dependencies() {
    log_step "Checking dependencies..."
    
    local missing_deps=()
    
    if ! command -v "$KUBECTL_CMD" &> /dev/null; then
        missing_deps+=("$KUBECTL_CMD")
    fi
    
    if ! command -v go &> /dev/null; then
        missing_deps+=("go")
    fi
    
    if [ "$SKIP_BUILD" = "false" ] && ! command -v make &> /dev/null; then
        missing_deps+=("make")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        exit 1
    fi
    
    # Check kubectl connectivity
    if ! $KUBECTL_CMD cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    
    log_info "All dependencies satisfied"
}

# Create namespace if it doesn't exist
create_namespace() {
    log_step "Creating namespace '$NAMESPACE'..."
    
    if $KUBECTL_CMD get namespace "$NAMESPACE" &> /dev/null; then
        log_info "Namespace '$NAMESPACE' already exists"
        return
    fi
    
    local namespace_yaml=$(cat << EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $NAMESPACE
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: system
    app.kubernetes.io/managed-by: ovim-deployment
EOF
)
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would create namespace:"
        echo "$namespace_yaml"
        return
    fi
    
    echo "$namespace_yaml" | $KUBECTL_CMD apply -f -
    log_info "Namespace '$NAMESPACE' created"
}

# Build binaries
build_binaries() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_info "Skipping binary build (--skip-build specified)"
        return
    fi
    
    log_step "Building OVIM binaries..."
    
    cd "$PROJECT_ROOT"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would run: make build build-controller"
        return
    fi
    
    make build build-controller
    log_info "Binaries built successfully"
}

# Generate and install CRDs
install_crds() {
    if [ "$SKIP_CRD" = "true" ]; then
        log_info "Skipping CRD installation (--skip-crd specified)"
        return
    fi
    
    log_step "Installing Custom Resource Definitions..."
    
    cd "$PROJECT_ROOT"
    
    # Generate CRDs if controller-gen is available
    if command -v controller-gen &> /dev/null; then
        log_debug "Generating CRDs with controller-gen"
        if [ "$DRY_RUN" = "false" ]; then
            make manifests
        fi
    else
        log_warn "controller-gen not found, using pre-generated CRDs"
    fi
    
    local crd_files=(
        "config/crd/organization.yaml"
        "config/crd/virtualdatacenter.yaml"
        "config/crd/catalog.yaml"
    )
    
    for crd_file in "${crd_files[@]}"; do
        if [ ! -f "$crd_file" ]; then
            log_error "CRD file not found: $crd_file"
            exit 1
        fi
        
        if [ "$DRY_RUN" = "true" ]; then
            log_info "[DRY-RUN] Would apply CRD: $crd_file"
            continue
        fi
        
        log_debug "Applying CRD: $crd_file"
        $KUBECTL_CMD apply -f "$crd_file"
    done
    
    # Wait for CRDs to be established
    if [ "$DRY_RUN" = "false" ]; then
        log_debug "Waiting for CRDs to be established..."
        $KUBECTL_CMD wait --for condition=established --timeout=60s crd/organizations.ovim.io
        $KUBECTL_CMD wait --for condition=established --timeout=60s crd/virtualdatacenters.ovim.io
        $KUBECTL_CMD wait --for condition=established --timeout=60s crd/catalogs.ovim.io
    fi
    
    log_info "CRDs installed successfully"
}

# Setup RBAC
setup_rbac() {
    if [ "$SKIP_RBAC" = "true" ]; then
        log_info "Skipping RBAC setup (--skip-rbac specified)"
        return
    fi
    
    log_step "Setting up RBAC..."
    
    local rbac_yaml=$(cat << EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ovim-controller
  namespace: $NAMESPACE
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: controller
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ovim-controller
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: controller
rules:
- apiGroups: ["ovim.io"]
  resources: ["organizations", "virtualdatacenters", "catalogs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["ovim.io"]
  resources: ["organizations/status", "virtualdatacenters/status", "catalogs/status"]
  verbs: ["get", "update", "patch"]
- apiGroups: ["ovim.io"]
  resources: ["organizations/finalizers", "virtualdatacenters/finalizers", "catalogs/finalizers"]
  verbs: ["update"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["resourcequotas", "limitranges"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["rolebindings", "clusterrolebindings"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ovim-controller
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ovim-controller
subjects:
- kind: ServiceAccount
  name: ovim-controller
  namespace: $NAMESPACE
EOF
)
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would apply RBAC:"
        echo "$rbac_yaml"
        return
    fi
    
    echo "$rbac_yaml" | $KUBECTL_CMD apply -f -
    log_info "RBAC configured successfully"
}

# Deploy controller
deploy_controller() {
    if [ "$SKIP_CONTROLLER" = "true" ]; then
        log_info "Skipping controller deployment (--skip-controller specified)"
        return
    fi
    
    log_step "Deploying OVIM controller..."
    
    local controller_yaml=$(cat << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ovim-controller
  namespace: $NAMESPACE
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: controller
    app.kubernetes.io/version: "$IMAGE_TAG"
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: ovim
      app.kubernetes.io/component: controller
  template:
    metadata:
      labels:
        app.kubernetes.io/name: ovim
        app.kubernetes.io/component: controller
        app.kubernetes.io/version: "$IMAGE_TAG"
    spec:
      serviceAccountName: ovim-controller
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: controller
        image: $CONTROLLER_IMAGE:$IMAGE_TAG
        command:
        - ovim_controller
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          runAsNonRoot: true
          seccompProfile:
            type: RuntimeDefault
        args:
        - --metrics-bind-address=:8080
        - --health-probe-bind-address=:8081
        - --leader-elect=true
        env:
        - name: OVIM_ENVIRONMENT
          value: "production"
        - name: OVIM_LOG_LEVEL
          value: "info"
        ports:
        - containerPort: 8080
          name: metrics
          protocol: TCP
        - containerPort: 8081
          name: health
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: health
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: health
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: ovim-controller-metrics
  namespace: $NAMESPACE
  labels:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: controller
spec:
  selector:
    app.kubernetes.io/name: ovim
    app.kubernetes.io/component: controller
  ports:
  - name: metrics
    port: 8080
    targetPort: metrics
    protocol: TCP
EOF
)
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would deploy controller:"
        echo "$controller_yaml"
        return
    fi
    
    echo "$controller_yaml" | $KUBECTL_CMD apply -f -
    log_info "Controller deployed successfully"
}

# Run database migration
run_db_migration() {
    if [ "$SKIP_DB_MIGRATION" = "true" ]; then
        log_info "Skipping database migration (--skip-db-migration specified)"
        return
    fi
    
    log_step "Running database migration..."
    
    if [ -z "${DATABASE_URL:-}" ]; then
        log_warn "DATABASE_URL not set, skipping database migration"
        log_warn "Run 'make db-migrate' manually to apply database changes"
        return
    fi
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Would run database migration"
        return
    fi
    
    cd "$PROJECT_ROOT"
    make db-migrate
    log_info "Database migration completed"
}

# Verify deployment
verify_deployment() {
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY-RUN] Skipping deployment verification"
        return
    fi
    
    log_step "Verifying deployment..."
    
    # Check CRDs
    if [ "$SKIP_CRD" = "false" ]; then
        log_debug "Checking CRDs..."
        $KUBECTL_CMD get crd organizations.ovim.io virtualdatacenters.ovim.io catalogs.ovim.io > /dev/null
        log_info "✓ CRDs are installed"
    fi
    
    # Check RBAC
    if [ "$SKIP_RBAC" = "false" ]; then
        log_debug "Checking RBAC..."
        $KUBECTL_CMD get serviceaccount ovim-controller -n "$NAMESPACE" > /dev/null
        $KUBECTL_CMD get clusterrole ovim-controller > /dev/null
        $KUBECTL_CMD get clusterrolebinding ovim-controller > /dev/null
        log_info "✓ RBAC is configured"
    fi
    
    # Check controller deployment
    if [ "$SKIP_CONTROLLER" = "false" ]; then
        log_debug "Checking controller deployment..."
        $KUBECTL_CMD get deployment ovim-controller -n "$NAMESPACE" > /dev/null
        
        # Wait for deployment to be ready
        log_debug "Waiting for controller to be ready..."
        $KUBECTL_CMD wait --for=condition=available --timeout=300s deployment/ovim-controller -n "$NAMESPACE"
        log_info "✓ Controller is running"
    fi
    
    log_info "Deployment verification completed successfully"
}

# Show deployment status
show_status() {
    log_step "Deployment Status:"
    echo ""
    echo "Namespace: $NAMESPACE"
    echo "Image Tag: $IMAGE_TAG"
    echo "Controller Image: $CONTROLLER_IMAGE:$IMAGE_TAG"
    echo ""
    
    if [ "$DRY_RUN" = "false" ]; then
        echo "CRDs:"
        $KUBECTL_CMD get crd | grep ovim.io || echo "  No OVIM CRDs found"
        echo ""
        
        echo "Controller Deployment:"
        $KUBECTL_CMD get deployment,pod,service -n "$NAMESPACE" -l app.kubernetes.io/name=ovim || echo "  No OVIM resources found"
    fi
}

# Main deployment function
main() {
    log_info "Starting OVIM deployment..."
    log_info "Namespace: $NAMESPACE, Image Tag: $IMAGE_TAG"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_warn "DRY-RUN mode enabled - no changes will be applied"
    fi
    
    check_dependencies
    create_namespace
    build_binaries
    install_crds
    setup_rbac
    deploy_controller
    run_db_migration
    verify_deployment
    show_status
    
    log_info "OVIM deployment completed successfully!"
    
    if [ "$DRY_RUN" = "false" ]; then
        echo ""
        log_info "Next steps:"
        echo "  1. Check controller logs: $KUBECTL_CMD logs -f deployment/ovim-controller -n $NAMESPACE"
        echo "  2. Create sample resources: $KUBECTL_CMD apply -f config/samples/"
        echo "  3. Check CRD status: $KUBECTL_CMD get organizations,virtualdatacenters,catalogs --all-namespaces"
    fi
}

# Run main function
main "$@"