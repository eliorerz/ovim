#!/bin/bash

# OVIM Hub Networking Setup Script
# This script configures network access for spoke agents to reach the hub

set -euo pipefail

VERBOSE=false
DRY_RUN=false
NODEPORT=32443

# Function to print usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Configure network access for OVIM hub cluster to enable spoke agent connectivity.

OPTIONS:
    -p, --port PORT       NodePort to use for external access (default: 32443)
    -d, --dry-run        Print configuration without applying
    -v, --verbose        Enable verbose output
    -h, --help           Show this help message

DESCRIPTION:
    This script creates a NodePort service to expose the OVIM hub server externally,
    allowing spoke agents from remote clusters to connect. The NodePort service
    exposes the hub on all cluster nodes at the specified port.

EXAMPLES:
    # Setup with default port
    $0

    # Setup with custom port
    $0 --port 30443

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

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -p|--port)
            NODEPORT="$2"
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

# Function to generate NodePort service configuration
generate_nodeport_config() {
    cat << EOF
---
apiVersion: v1
kind: Service
metadata:
  name: ovim-server-external
  namespace: ovim-system
  labels:
    app.kubernetes.io/component: server
    app.kubernetes.io/name: ovim
    ovim.io/service-type: external
spec:
  type: NodePort
  ports:
  - name: https
    port: 8443
    protocol: TCP
    targetPort: 8443
    nodePort: $NODEPORT
  selector:
    app.kubernetes.io/component: server
    app.kubernetes.io/name: ovim
  sessionAffinity: None
EOF
}

# Function to get external endpoint
get_external_endpoint() {
    # Get the first master node IP as the external endpoint
    kubectl get nodes -l node-role.kubernetes.io/master -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || \
    kubectl get nodes -l node-role.kubernetes.io/control-plane -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || \
    kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null
}

# Main execution
main() {
    log "Starting OVIM hub networking setup..."

    # Check if we're in the hub cluster
    if ! kubectl get service ovim-server -n ovim-system >/dev/null 2>&1; then
        echo "ERROR: ovim-server service not found. This script must be run against the hub cluster." >&2
        exit 1
    fi

    # Generate configuration
    log "Generating NodePort service configuration..."
    CONFIG=$(generate_nodeport_config)

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "# NodePort service configuration for external hub access"
        echo "# Port: $NODEPORT"
        echo ""
        echo "$CONFIG"
        echo ""
        echo "# External endpoint would be: https://\$(NODE_IP):$NODEPORT"
        exit 0
    fi

    # Apply NodePort service
    log "Creating NodePort service for external access..."
    echo "$CONFIG" | kubectl apply -f -

    # Get external endpoint
    EXTERNAL_IP=$(get_external_endpoint)
    if [[ -n "$EXTERNAL_IP" ]]; then
        EXTERNAL_ENDPOINT="https://$EXTERNAL_IP:$NODEPORT"
        log "External endpoint configured: $EXTERNAL_ENDPOINT"

        # Test connectivity
        log "Testing external endpoint connectivity..."
        if curl -k -s "$EXTERNAL_ENDPOINT/health" >/dev/null 2>&1; then
            log "✅ External endpoint is accessible"
        else
            log "⚠️  External endpoint test failed - this may be expected if testing from within the cluster"
        fi
    else
        echo "WARNING: Could not determine external IP address" >&2
    fi

    # Show service status
    echo ""
    echo "Hub networking setup complete!"
    echo "NodePort service status:"
    kubectl get svc ovim-server-external -n ovim-system

    if [[ -n "$EXTERNAL_IP" ]]; then
        echo ""
        echo "External endpoint: $EXTERNAL_ENDPOINT"
        echo ""
        echo "Spoke agents can now connect using:"
        echo "  HUB_ENDPOINT=$EXTERNAL_ENDPOINT"
    fi

    echo ""
    echo "To deploy spoke agents to remote clusters, use:"
    echo "  ./scripts/deploy-spoke-agent.sh -k /path/to/remote-kubeconfig"
}

# Run main function
main "$@"