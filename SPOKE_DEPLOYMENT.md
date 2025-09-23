# OVIM Spoke Agent Deployment Guide

This guide explains how to deploy OVIM spoke agents to multiple clusters using the new deployment infrastructure.

## Overview

The OVIM spoke agent deployment system allows you to:
- Deploy spoke agents to multiple OpenShift/Kubernetes clusters
- Manage multi-cluster deployments from a single hub
- Monitor and control spoke agents across different zones
- Build and push spoke agent container images

## Quick Start

### 1. Build and Push Spoke Agent Image

```bash
# Build and push spoke agent container
make build-push-spoke-agent

# Or build both hub and spoke agent together
make build-push-all
```

### 2. Deploy to a Single Cluster

```bash
# Deploy to a single spoke cluster
make deploy-spoke-agent \
  SPOKE_KUBECONFIG=/path/to/spoke/kubeconfig \
  CLUSTER_ID=spoke-cluster-1 \
  ZONE_ID=east-zone-1
```

### 3. Deploy to Multiple Clusters

```bash
# Deploy to multiple spoke clusters
make deploy-spoke-multiple \
  SPOKE_CLUSTERS='spoke1:/path/to/kubeconfig1:east-1 spoke2:/path/to/kubeconfig2:west-1'
```

### 4. Deploy Complete Stack with Spokes

```bash
# Deploy hub cluster + spoke agents in one command
make deploy-stack-with-spokes \
  SPOKE_CLUSTERS='spoke1:/path/to/kubeconfig1:east-1 spoke2:/path/to/kubeconfig2:west-1'
```

## Configuration Options

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SPOKE_KUBECONFIG` | Path to spoke cluster kubeconfig | Required |
| `CLUSTER_ID` | Unique identifier for the cluster | Required |
| `ZONE_ID` | Zone identifier for the cluster | Required |
| `HUB_ENDPOINT` | Hub server endpoint URL | `http://ovim-server.ovim-system.svc.cluster.local:8080` |
| `SPOKE_AGENT_IMAGE` | Spoke agent container image | `quay.io/eerez/ovim-spoke-agent` |
| `SPOKE_IMAGE_TAG` | Image tag to deploy | `latest` |
| `HUB_PROTOCOL` | Hub protocol (http/https) | `http` |
| `HUB_TLS_ENABLED` | Enable TLS for hub communication | `false` |
| `HUB_TLS_SKIP_VERIFY` | Skip TLS verification | `true` |
| `LOG_LEVEL` | Logging level | `info` |
| `CPU_REQUEST` | CPU resource request | `100m` |
| `MEMORY_REQUEST` | Memory resource request | `128Mi` |
| `CPU_LIMIT` | CPU resource limit | `500m` |
| `MEMORY_LIMIT` | Memory resource limit | `512Mi` |

### Examples

#### Basic Deployment
```bash
make deploy-spoke-agent \
  SPOKE_KUBECONFIG=~/.kube/spoke1 \
  CLUSTER_ID=production-east \
  ZONE_ID=us-east-1
```

#### Production Deployment with Custom Configuration
```bash
make deploy-spoke-agent \
  SPOKE_KUBECONFIG=/etc/kubernetes/spoke-prod.conf \
  CLUSTER_ID=prod-cluster \
  ZONE_ID=us-west-2 \
  HUB_ENDPOINT=https://ovim-hub.company.com:8443 \
  HUB_PROTOCOL=https \
  HUB_TLS_ENABLED=true \
  HUB_TLS_SKIP_VERIFY=false \
  LOG_LEVEL=debug \
  CPU_LIMIT=1000m \
  MEMORY_LIMIT=1Gi
```

#### Multi-Cluster Deployment
```bash
make deploy-spoke-multiple \
  SPOKE_CLUSTERS='prod-east:/etc/k8s/prod-east.conf:us-east-1 prod-west:/etc/k8s/prod-west.conf:us-west-1 dev-cluster:~/.kube/dev:dev-zone'
```

## Management Commands

### Status Monitoring

```bash
# Check status of all spoke agents
make spoke-status \
  SPOKE_CLUSTERS='spoke1:/path/to/kubeconfig1:zone1 spoke2:/path/to/kubeconfig2:zone2'

# View logs from all spoke agents
make spoke-logs \
  SPOKE_CLUSTERS='spoke1:/path/to/kubeconfig1:zone1'
```

### Cleanup

```bash
# Remove spoke agent from a single cluster
make undeploy-spoke-agent SPOKE_KUBECONFIG=/path/to/kubeconfig

# Remove from multiple clusters
for config in /path/to/kubeconfig1 /path/to/kubeconfig2; do
  make undeploy-spoke-agent SPOKE_KUBECONFIG=$config
done
```

## Architecture

### Deployment Components

Each spoke cluster deployment includes:

1. **Namespace**: `ovim-system` namespace for OVIM components
2. **RBAC**: ServiceAccount, ClusterRole, and ClusterRoleBinding
3. **Deployment**: Spoke agent deployment with configurable resources
4. **Service**: ClusterIP service for local API access
5. **ConfigMap**: Configuration for spoke agent settings

### Network Requirements

- Spoke agents need outbound access to the hub cluster
- Default hub endpoint: `http://ovim-server.ovim-system.svc.cluster.local:8080`
- Health check endpoint on port 8080 (configurable)

### Security

- Spoke agents run as non-root user (UID 1000)
- Read-only root filesystem
- Minimal RBAC permissions
- Network policies supported for isolation

## Troubleshooting

### Common Issues

#### 1. Deployment Fails with Permission Errors
```bash
# Check if you have admin access to the spoke cluster
KUBECONFIG=/path/to/spoke kubectl auth can-i create namespaces

# Verify RBAC permissions
KUBECONFIG=/path/to/spoke kubectl auth can-i create clusterroles
```

#### 2. Spoke Agent Not Connecting to Hub
```bash
# Check spoke agent logs
KUBECONFIG=/path/to/spoke kubectl logs -n ovim-system deployment/ovim-spoke-agent

# Verify hub endpoint is accessible
KUBECONFIG=/path/to/spoke kubectl run test-pod --image=curlimages/curl --rm -it -- curl -v $HUB_ENDPOINT/health
```

#### 3. Image Pull Errors
```bash
# Check if image exists
podman pull quay.io/eerez/ovim-spoke-agent:latest

# Verify image registry access from spoke cluster
KUBECONFIG=/path/to/spoke kubectl create job test-pull --image=quay.io/eerez/ovim-spoke-agent:latest -- echo "success"
```

### Debug Commands

```bash
# Get detailed status
KUBECONFIG=/path/to/spoke kubectl describe deployment ovim-spoke-agent -n ovim-system

# Check events
KUBECONFIG=/path/to/spoke kubectl get events -n ovim-system --sort-by='.lastTimestamp'

# View spoke agent configuration
KUBECONFIG=/path/to/spoke kubectl get deployment ovim-spoke-agent -n ovim-system -o yaml
```

## Integration with ACM

When using Red Hat Advanced Cluster Management (ACM):

1. **Hub Cluster**: Deploy OVIM hub on the ACM hub cluster
2. **Managed Clusters**: Deploy spoke agents on ACM managed clusters
3. **Zone Discovery**: Clusters are automatically discovered as zones
4. **Centralized Management**: Manage all spoke agents from the hub

Example ACM deployment:
```bash
# Hub cluster (ACM hub)
KUBECONFIG=/path/to/acm-hub make deploy-stack

# Managed clusters (ACM spokes)
make deploy-spoke-multiple \
  SPOKE_CLUSTERS='cluster1:/path/to/managed1:zone1 cluster2:/path/to/managed2:zone2'
```

## Next Steps

1. Deploy your first spoke agent to test connectivity
2. Configure hub endpoints for production use
3. Set up monitoring and alerting for spoke agents
4. Implement VM management on spoke clusters
5. Configure zone-based resource allocation