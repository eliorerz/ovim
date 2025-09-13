# OVIM Deployment Guide

This guide explains how to deploy OVIM (OpenShift Virtual Infrastructure Manager) to a Kubernetes cluster using the comprehensive deployment system.

## Prerequisites

- Kubernetes cluster (v1.19+)
- `kubectl` configured to access your cluster
- Go 1.19+ (for building binaries)
- `make` (for build automation)
- Database (PostgreSQL) for OVIM backend

## Quick Start

### 1. Deploy Complete OVIM Stack (Recommended)

```bash
# Deploy complete stack: PostgreSQL + OVIM + UI + Ingress
make deploy-stack

# Or with custom configuration
OVIM_NAMESPACE=my-ovim OVIM_DOMAIN=ovim.example.com make deploy-stack
```

### 2. Deploy OVIM Controllers Only

```bash
# Deploy only controllers (requires external database)
make deploy

# Or with custom configuration
OVIM_NAMESPACE=my-ovim OVIM_IMAGE_TAG=v1.0.0 make deploy
```

### 3. Verify Deployment

```bash
# Check complete stack status
make stack-status

# Check individual component status
make deployment-status

# Check logs from all components
make stack-logs
```

### 4. Access the Application

```bash
# Set up port forwarding for local access
make stack-port-forward

# Then access:
# UI: https://localhost:8443
# API: https://localhost:8444
```

### 5. Create Sample Resources

```bash
# Deploy sample organization, VDC, and catalog
make deploy-samples
```

## Deployment Options

### Configuration Variables

The deployment system uses environment variables that can be overridden:

| Variable | Default | Description |
|----------|---------|-------------|
| `OVIM_NAMESPACE` | `ovim-system` | Kubernetes namespace for OVIM |
| `OVIM_IMAGE_TAG` | `latest` | Docker image tag for containers |
| `OVIM_CONTROLLER_IMAGE` | `ovim-controller` | Controller container image name |
| `OVIM_SERVER_IMAGE` | `ovim-server` | Server container image name |
| `KUBECTL_CMD` | `kubectl` | kubectl command to use |
| `DATABASE_URL` | (none) | Database connection string |

#### Stack Configuration Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OVIM_DOMAIN` | `ovim.local` | Domain for ingress configuration |
| `OVIM_DB_STORAGE_SIZE` | `10Gi` | PostgreSQL storage size |
| `OVIM_UI_REPLICAS` | `2` | Number of UI pod replicas |
| `OVIM_INGRESS_CLASS` | `nginx` | Ingress controller class |

### Available Make Targets

#### Deployment Targets

- `make deploy` - Deploy OVIM to Kubernetes cluster
- `make deploy-dry-run` - Show what would be deployed without applying
- `make deploy-samples` - Deploy sample resources after OVIM deployment
- `make deploy-full` - Complete deployment with database migration and samples
- `make deploy-dev` - Deploy with development configuration
- `make deploy-prod` - Deploy with production configuration
- `make undeploy` - Remove OVIM from Kubernetes cluster
- `make deployment-status` - Show current deployment status

#### Full Stack Targets

- `make deploy-stack` - Deploy complete stack (PostgreSQL + OVIM + UI + Ingress)
- `make deploy-stack-dry-run` - Show what would be deployed in the full stack
- `make deploy-stack-dev` - Deploy development stack
- `make deploy-stack-prod` - Deploy production stack
- `make deploy-database` - Deploy only PostgreSQL database
- `make deploy-ui` - Deploy only OVIM UI
- `make deploy-ingress` - Deploy only ingress configuration
- `make undeploy-stack` - Remove entire stack
- `make stack-status` - Show status of all stack components
- `make stack-logs` - Show logs from all stack components
- `make stack-port-forward` - Set up port forwarding for local access

#### Development Targets

- `make build` - Build OVIM server binary
- `make build-controller` - Build OVIM controller binary
- `make test` - Run all tests
- `make fmt` - Format code
- `make lint` - Run linter

#### Database Targets

- `make db-migrate` - Apply database migration
- `make db-start` - Start PostgreSQL database (for development)
- `make db-stop` - Stop PostgreSQL database

## Deployment Scenarios

### Development Environment

```bash
# Deploy to development namespace with dev tag
make deploy-dev

# Or manually specify
OVIM_NAMESPACE=ovim-dev OVIM_IMAGE_TAG=dev make deploy
```

### Production Environment

```bash
# Deploy to production namespace with latest tag
make deploy-prod

# With database migration
DATABASE_URL="postgres://user:pass@host:5432/ovim" make deploy-full
```

### Custom Configuration

```bash
# Deploy to custom namespace with specific image
OVIM_NAMESPACE=my-ovim \
OVIM_IMAGE_TAG=v1.2.3 \
OVIM_CONTROLLER_IMAGE=my-registry/ovim-controller \
make deploy
```

## Script Usage

### Direct Script Usage

You can also use the deployment scripts directly for more control:

```bash
# Deploy script with options
./scripts/deploy.sh --namespace my-ovim --image-tag v1.0.0 --verbose

# Dry run to see what would be deployed
./scripts/deploy.sh --dry-run

# Skip building binaries (use existing images)
./scripts/deploy.sh --skip-build --image-tag existing-tag

# Deploy only CRDs and RBAC (skip controller)
./scripts/deploy.sh --skip-controller --skip-db-migration
```

### Undeploy Script

```bash
# Remove OVIM with confirmation
./scripts/undeploy.sh

# Force removal without prompts
./scripts/undeploy.sh --force

# Keep CRDs but remove controller
./scripts/undeploy.sh --keep-crds

# Dry run to see what would be removed
./scripts/undeploy.sh --dry-run
```

## Components Deployed

The deployment system installs the following components:

### 1. Custom Resource Definitions (CRDs)
- `organizations.ovim.io` - Organization management
- `virtualdatacenters.ovim.io` - Virtual Data Center management
- `catalogs.ovim.io` - Catalog management

### 2. RBAC Configuration
- ServiceAccount: `ovim-controller`
- ClusterRole: `ovim-controller` 
- ClusterRoleBinding: `ovim-controller`

### 3. Controller Deployment
- Deployment: `ovim-controller`
- Service: `ovim-controller-metrics`
- Health and metrics endpoints

### 4. Sample Resources (optional)
- Example Organization
- Example Virtual Data Center
- Example Catalog

## Database Migration

OVIM requires a database migration for the CRD-based architecture:

```bash
# Set database URL and run migration
export DATABASE_URL="postgres://user:pass@host:5432/ovim"
make db-migrate

# Or include in full deployment
DATABASE_URL="postgres://user:pass@host:5432/ovim" make deploy-full
```

## Troubleshooting

### Check Deployment Status

```bash
make deployment-status
```

### Check Controller Logs

```bash
kubectl logs -f deployment/ovim-controller -n ovim-system
```

### Check CRD Status

```bash
kubectl get crd | grep ovim.io
kubectl describe crd organizations.ovim.io
```

### Check Custom Resources

```bash
kubectl get organizations,virtualdatacenters,catalogs --all-namespaces
```

### Common Issues

1. **Controller not starting**: Check RBAC permissions and image availability
2. **CRDs not found**: Ensure `make manifests` was run and CRDs were applied
3. **Database connection**: Verify DATABASE_URL is correct and accessible
4. **Namespace issues**: Check if namespace exists and has proper labels

## Cleanup

### Complete Removal

```bash
# Remove everything (with confirmation)
make undeploy

# Force removal without prompts
./scripts/undeploy.sh --force
```

### Partial Cleanup

```bash
# Keep CRDs and custom resources
./scripts/undeploy.sh --keep-crds

# Keep namespace
./scripts/undeploy.sh --keep-namespace
```

## Advanced Configuration

### Custom Images

If you're using custom container images, set the image variables:

```bash
export OVIM_CONTROLLER_IMAGE="my-registry.com/ovim-controller"
export OVIM_SERVER_IMAGE="my-registry.com/ovim-server"
export OVIM_IMAGE_TAG="v1.2.3"
make deploy
```

### Multiple Environments

You can deploy multiple OVIM instances in different namespaces:

```bash
# Development
OVIM_NAMESPACE=ovim-dev make deploy

# Staging
OVIM_NAMESPACE=ovim-staging make deploy

# Production
OVIM_NAMESPACE=ovim-prod make deploy
```

### Using Different kubectl Context

```bash
# Use specific kubectl context
KUBECTL_CMD="kubectl --context=my-cluster" make deploy
```

## Integration with CI/CD

The deployment system is designed to work with CI/CD pipelines:

```yaml
# Example GitLab CI
deploy:
  script:
    - export OVIM_NAMESPACE=$CI_ENVIRONMENT_NAME
    - export OVIM_IMAGE_TAG=$CI_COMMIT_TAG
    - make deploy
```

```yaml
# Example GitHub Actions
- name: Deploy OVIM
  run: |
    export OVIM_NAMESPACE=ovim-${{ github.ref_name }}
    export OVIM_IMAGE_TAG=${{ github.sha }}
    make deploy
```

## Next Steps

After successful deployment:

1. Create your first organization: `kubectl apply -f config/samples/sample-organization.yaml`
2. Create a virtual data center: `kubectl apply -f config/samples/sample-vdc.yaml`
3. Set up monitoring and logging
4. Configure ingress for external access
5. Set up backup procedures for custom resources

For more information, see the main [README.md](README.md) and [CRD documentation](config/crd/README.md).