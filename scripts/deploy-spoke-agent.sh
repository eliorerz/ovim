#!/bin/bash

# OVIM Spoke Agent Deployment Script
# This script automatically deploys a spoke agent to any cluster with proper configuration

set -euo pipefail

# Default values
HUB_EXTERNAL_ENDPOINT="https://192.168.111.20:32443"
HUB_INTERNAL_ENDPOINT="https://ovim-server.ovim-system.svc.cluster.local:8443"
TEMPLATE_FILE="config/spoke-agent/ovim-spoke-agent-template.yaml"
DRY_RUN=false
VERBOSE=false

# Function to print usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Deploy OVIM spoke agent to any Kubernetes cluster with automatic configuration.

OPTIONS:
    -k, --kubeconfig PATH     Kubeconfig file path (default: uses current context)
    -e, --hub-endpoint URL    Hub endpoint URL (auto-detected: internal for hub cluster, external for remote)
    -d, --dry-run            Print the configuration without applying
    -v, --verbose            Enable verbose output
    -h, --help               Show this help message

EXAMPLES:
    # Deploy to current cluster context
    $0

    # Deploy to specific cluster
    $0 -k /path/to/kubeconfig

    # Deploy with custom hub endpoint
    $0 -e https://my-hub.example.com

    # Dry run to see configuration
    $0 --dry-run

EOF
}

# Function to log messages
log() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >&2
    fi
}

# Function to detect cluster name from current context
detect_cluster_name() {
    local kubeconfig_arg=""
    if [[ -n "${KUBECONFIG_FILE:-}" ]]; then
        kubeconfig_arg="--kubeconfig=$KUBECONFIG_FILE"
    fi

    # Try to get cluster name from current context
    local current_context
    current_context=$(kubectl $kubeconfig_arg config current-context 2>/dev/null || echo "")

    if [[ -z "$current_context" ]]; then
        echo "ERROR: No current kubernetes context found" >&2
        return 1
    fi

    # Get cluster name from context
    local cluster_name
    cluster_name=$(kubectl $kubeconfig_arg config view -o jsonpath="{.contexts[?(@.name=='$current_context')].context.cluster}" 2>/dev/null || echo "")

    if [[ -z "$cluster_name" ]]; then
        echo "ERROR: Could not determine cluster name from context" >&2
        return 1
    fi

    echo "$cluster_name"
}

# Function to detect if this is the hub cluster
is_hub_cluster() {
    local kubeconfig_arg=""
    if [[ -n "${KUBECONFIG_FILE:-}" ]]; then
        kubeconfig_arg="--kubeconfig=$KUBECONFIG_FILE"
    fi

    # Check if ovim-server service exists in ovim-system namespace
    if kubectl $kubeconfig_arg get service ovim-server -n ovim-system >/dev/null 2>&1; then
        return 0  # true - this is the hub cluster
    else
        return 1  # false - this is a remote cluster
    fi
}

# Function to check if ovim-system namespace exists, create if not
ensure_namespace() {
    local kubeconfig_arg=""
    if [[ -n "${KUBECONFIG_FILE:-}" ]]; then
        kubeconfig_arg="--kubeconfig=$KUBECONFIG_FILE"
    fi

    if ! kubectl $kubeconfig_arg get namespace ovim-system >/dev/null 2>&1; then
        log "Creating ovim-system namespace..."
        kubectl $kubeconfig_arg create namespace ovim-system
    fi
}

# Function to ensure ServiceAccount exists
ensure_service_account() {
    local kubeconfig_arg=""
    if [[ -n "${KUBECONFIG_FILE:-}" ]]; then
        kubeconfig_arg="--kubeconfig=$KUBECONFIG_FILE"
    fi

    if ! kubectl $kubeconfig_arg get serviceaccount ovim-spoke-agent -n ovim-system >/dev/null 2>&1; then
        log "Creating ovim-spoke-agent service account..."
        cat << EOF | kubectl $kubeconfig_arg apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ovim-spoke-agent
  namespace: ovim-system
  labels:
    app.kubernetes.io/name: ovim-spoke-agent
    app.kubernetes.io/component: spoke-agent
    app.kubernetes.io/part-of: ovim
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ovim-spoke-agent
  labels:
    app.kubernetes.io/name: ovim-spoke-agent
    app.kubernetes.io/component: spoke-agent
    app.kubernetes.io/part-of: ovim
rules:
- apiGroups: [""]
  resources: ["namespaces", "nodes", "pods", "services", "persistentvolumes", "persistentvolumeclaims"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines", "virtualmachineinstances"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["cdi.kubevirt.io"]
  resources: ["datavolumes"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["nodes", "pods"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ovim-spoke-agent
  labels:
    app.kubernetes.io/name: ovim-spoke-agent
    app.kubernetes.io/component: spoke-agent
    app.kubernetes.io/part-of: ovim
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ovim-spoke-agent
subjects:
- kind: ServiceAccount
  name: ovim-spoke-agent
  namespace: ovim-system
EOF
    fi
}

# Function to generate spoke agent configuration
generate_config() {
    local cluster_name="$1"
    local zone_id="$2"
    local hub_endpoint="$3"
    local hub_protocol="$4"
    local hub_tls_enabled="$5"
    local hub_tls_skip_verify="$6"

    # Substitute variables in template
    sed -e "s|\${CLUSTER_ID}|$cluster_name|g" \
        -e "s|\${ZONE_ID}|$zone_id|g" \
        -e "s|\${HUB_ENDPOINT}|$hub_endpoint|g" \
        -e "s|\${HUB_PROTOCOL}|$hub_protocol|g" \
        -e "s|\${HUB_TLS_ENABLED}|$hub_tls_enabled|g" \
        -e "s|\${HUB_TLS_SKIP_VERIFY}|$hub_tls_skip_verify|g" \
        "$TEMPLATE_FILE"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -k|--kubeconfig)
            KUBECONFIG_FILE="$2"
            shift 2
            ;;
        -e|--hub-endpoint)
            HUB_ENDPOINT_OVERRIDE="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "ERROR: Unknown option: $1" >&2
            usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    log "Starting OVIM spoke agent deployment..."

    # Check if template file exists
    if [[ ! -f "$TEMPLATE_FILE" ]]; then
        echo "ERROR: Template file not found: $TEMPLATE_FILE" >&2
        echo "Please run this script from the ovim project root directory." >&2
        exit 1
    fi

    # Detect cluster name
    log "Detecting cluster name..."
    CLUSTER_NAME=$(detect_cluster_name)
    log "Detected cluster name: $CLUSTER_NAME"

    # Use cluster name as zone ID (ACM managed clusters have same name as zone)
    ZONE_ID="$CLUSTER_NAME"

    # Determine hub endpoint and connection settings
    if [[ -n "${HUB_ENDPOINT_OVERRIDE:-}" ]]; then
        HUB_ENDPOINT="$HUB_ENDPOINT_OVERRIDE"
        HUB_PROTOCOL="http"
        HUB_TLS_ENABLED="true"
        HUB_TLS_SKIP_VERIFY="true"
        log "Using override hub endpoint: $HUB_ENDPOINT"
    elif is_hub_cluster; then
        HUB_ENDPOINT="$HUB_INTERNAL_ENDPOINT"
        HUB_PROTOCOL="http"
        HUB_TLS_ENABLED="true"
        HUB_TLS_SKIP_VERIFY="true"
        log "Detected hub cluster, using internal endpoint: $HUB_ENDPOINT"
    else
        HUB_ENDPOINT="$HUB_EXTERNAL_ENDPOINT"
        HUB_PROTOCOL="http"
        HUB_TLS_ENABLED="true"
        HUB_TLS_SKIP_VERIFY="true"
        log "Detected remote cluster, using external endpoint: $HUB_ENDPOINT"
    fi

    # Generate configuration
    log "Generating spoke agent configuration..."
    CONFIG=$(generate_config "$CLUSTER_NAME" "$ZONE_ID" "$HUB_ENDPOINT" "$HUB_PROTOCOL" "$HUB_TLS_ENABLED" "$HUB_TLS_SKIP_VERIFY")

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "# Generated configuration for cluster: $CLUSTER_NAME"
        echo "# Zone ID: $ZONE_ID"
        echo "# Hub endpoint: $HUB_ENDPOINT"
        echo "# Protocol: $HUB_PROTOCOL"
        echo ""
        echo "$CONFIG"
        exit 0
    fi

    # Set up kubeconfig argument if provided
    local kubeconfig_arg=""
    if [[ -n "${KUBECONFIG_FILE:-}" ]]; then
        kubeconfig_arg="--kubeconfig=$KUBECONFIG_FILE"
        export KUBECONFIG="$KUBECONFIG_FILE"
    fi

    # Ensure prerequisites
    log "Ensuring ovim-system namespace exists..."
    ensure_namespace

    log "Ensuring service account and RBAC exists..."
    ensure_service_account

    # Deploy spoke agent
    log "Deploying spoke agent to cluster: $CLUSTER_NAME"
    echo "$CONFIG" | kubectl $kubeconfig_arg apply -f -

    # Wait for deployment to be ready
    log "Waiting for spoke agent deployment to be ready..."
    kubectl $kubeconfig_arg wait --for=condition=available --timeout=300s deployment/ovim-spoke-agent -n ovim-system

    log "Spoke agent deployed successfully!"
    log "Cluster: $CLUSTER_NAME"
    log "Zone ID: $ZONE_ID"
    log "Hub endpoint: $HUB_ENDPOINT"

    # Show pod status
    echo ""
    echo "Spoke agent pod status:"
    kubectl $kubeconfig_arg get pods -n ovim-system -l app.kubernetes.io/name=ovim-spoke-agent
}

# Run main function
main "$@"