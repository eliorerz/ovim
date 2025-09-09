# OVIM Backend

OpenShift Virtual Infrastructure Manager (OVIM) Backend API Service

## Overview

OVIM Backend provides REST API services for managing multi-tenant virtual infrastructure on OpenShift using KubeVirt. It serves as the core business logic layer for the OVIM system, handling:

- Multi-tenant organization and VDC management
- VM lifecycle operations via KubeVirt
- VM template catalog services
- Role-based access control
- Integration with OpenShift/Kubernetes APIs

## Development

### Prerequisites

- Go 1.21+
- Podman
- Access to OpenShift cluster with KubeVirt

### Quick Start

```bash
# Run locally
make run

# Run in container
make container-run

# Development with hot reload
make dev
```

### Environment Variables

All OVIM configuration uses the `OVIM_` prefix:

- `OVIM_PORT`: Server port (default: 8080)
- `OVIM_DATABASE_URL`: PostgreSQL connection string
- `OVIM_KUBECONFIG`: Path to kubeconfig file
- `OVIM_JWT_SECRET`: JWT signing secret
- `OVIM_ENVIRONMENT`: Environment (development/production)

### API Endpoints

- `GET /health` - Health check
- `POST /api/v1/auth/login` - User authentication
- `GET /api/v1/organizations` - List organizations
- `POST /api/v1/organizations` - Create organization
- `GET /api/v1/catalog/templates` - List VM templates
- `GET /api/v1/vms` - List virtual machines
- `POST /api/v1/vms` - Deploy new VM

### Project Structure

```
.
├── api/          # API handlers and routes
├── config/       # Configuration management
├── models/       # Data models
├── kube/         # Kubernetes/KubeVirt integration
├── controller/   # Background controllers
├── auth/         # Authentication utilities
├── main.go       # Application entry point
└── Makefile      # Build and deployment targets
```

## Commands

- `make clean` - Clean containers and binaries
- `make build` - Build Go binary
- `make run` - Run locally
- `make container-run` - Run in container
- `make test` - Run tests
- `make lint` - Run linter