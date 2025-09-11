# OVIM Backend

OpenShift Virtual Infrastructure Manager (OVIM) Backend API Service

## Overview

OVIM Backend provides REST API services for managing multi-tenant virtual infrastructure on OpenShift using KubeVirt. It serves as the core business logic layer for the OVIM system, handling:

- **Multi-tenant Management**: Organizations, VDCs, and user management
- **Authentication & Authorization**: JWT-based auth with role-based access control
- **VM Lifecycle Operations**: Deploy, start, stop, delete virtual machines via KubeVirt
- **Template Catalog**: VM template management and catalog services
- **OpenShift Integration**: Native integration with OpenShift/Kubernetes APIs
- **Database Support**: PostgreSQL and in-memory storage backends

## Development

### Prerequisites

- Go 1.21+
- PostgreSQL 13+ (or use in-memory storage)
- Podman/Docker
- Access to OpenShift cluster with KubeVirt (optional for local dev)

### Quick Start

```bash
# Install dependencies
make deps

# Run with database
make dev-with-db

# Run locally (in-memory storage)
make run

# Run tests
make test

# Build container
make container-build
```

### Environment Variables

All OVIM configuration uses the `OVIM_` prefix:

**Core Settings:**
- `OVIM_PORT`: Server port (default: 8080)
- `OVIM_ENVIRONMENT`: Environment (development/production)
- `OVIM_LOG_LEVEL`: Log level (debug/info/warn/error)

**Database:**
- `OVIM_DATABASE_URL`: PostgreSQL connection string
- Example: `postgres://ovim:ovim123@localhost:5432/ovim?sslmode=disable`

**Security:**
- `OVIM_JWT_SECRET`: JWT signing secret (auto-generated if not set)
- `OVIM_TLS_ENABLED`: Enable TLS (true/false)

**OpenShift Integration:**
- `OVIM_KUBECONFIG`: Path to kubeconfig file
- `OVIM_OPENSHIFT_ENABLED`: Enable OpenShift integration (true/false)
- `OVIM_OPENSHIFT_USE_MOCK`: Use mock OpenShift client for development

### API Endpoints

**Authentication:**
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/logout` - User logout
- `GET /api/v1/auth/info` - Authentication info
- `GET /api/v1/auth/oidc/auth-url` - OIDC auth URL (if enabled)
- `POST /api/v1/auth/oidc/callback` - OIDC callback (if enabled)

**Organizations (System Admin only):**
- `GET /api/v1/organizations` - List organizations
- `POST /api/v1/organizations` - Create organization
- `GET /api/v1/organizations/:id` - Get organization
- `PUT /api/v1/organizations/:id` - Update organization
- `DELETE /api/v1/organizations/:id` - Delete organization
- `GET /api/v1/organizations/:id/users` - List organization users
- `POST /api/v1/organizations/:id/users/:userId` - Assign user to organization
- `DELETE /api/v1/organizations/:id/users/:userId` - Remove user from organization

**Users (System Admin only):**
- `GET /api/v1/users` - List users
- `POST /api/v1/users` - Create user
- `GET /api/v1/users/:id` - Get user
- `PUT /api/v1/users/:id` - Update user
- `DELETE /api/v1/users/:id` - Delete user

**Virtual Data Centers:**
- `GET /api/v1/vdcs` - List VDCs
- `POST /api/v1/vdcs` - Create VDC
- `GET /api/v1/vdcs/:id` - Get VDC
- `PUT /api/v1/vdcs/:id` - Update VDC
- `DELETE /api/v1/vdcs/:id` - Delete VDC

**Virtual Machines:**
- `GET /api/v1/vms` - List VMs
- `POST /api/v1/vms` - Deploy VM
- `GET /api/v1/vms/:id` - Get VM details
- `GET /api/v1/vms/:id/status` - Get VM status
- `PUT /api/v1/vms/:id/power` - Update VM power state (start/stop)
- `DELETE /api/v1/vms/:id` - Delete VM

**VM Templates:**
- `GET /api/v1/catalog/templates` - List templates
- `GET /api/v1/catalog/templates/:id` - Get template

**User Profile:**
- `GET /api/v1/profile/organization` - Get user's organization
- `GET /api/v1/profile/vdcs` - Get user's VDCs

**OpenShift Integration (if enabled):**
- `GET /api/v1/openshift/status` - OpenShift cluster status
- `GET /api/v1/openshift/templates` - OpenShift VM templates
- `GET /api/v1/openshift/vms` - OpenShift VMs
- `POST /api/v1/openshift/vms` - Deploy VM from OpenShift template

**Health & Info:**
- `GET /health` - Health check
- `GET /version` - Version information

### User Roles

OVIM supports three user roles with different permissions:

1. **System Administrator** (`system_admin`):
   - Full system access
   - Manage organizations and users
   - View all resources across organizations

2. **Organization Administrator** (`org_admin`):
   - Manage organization resources
   - Deploy and manage VMs within organization
   - View organization-wide metrics

3. **Organization User** (`org_user`):
   - Deploy and manage personal VMs
   - Access organization VM catalog
   - Limited to personal resources

### Project Structure

```
.
├── cmd/
│   └── ovim-server/    # Main application entry point
├── pkg/
│   ├── api/           # REST API handlers and routes
│   ├── auth/          # Authentication and JWT utilities
│   ├── config/        # Configuration management
│   ├── models/        # Data models and types
│   ├── storage/       # Storage backends (PostgreSQL, memory)
│   ├── util/          # Utility functions
│   ├── version/       # Version information
│   ├── kubevirt/      # KubeVirt integration
│   ├── openshift/     # OpenShift client integration
│   └── tls/           # TLS certificate management
├── test/
│   └── integration/   # Integration tests
├── auth/              # Legacy auth module
├── config/            # Configuration files
├── scripts/           # Build and deployment scripts
├── Makefile           # Build automation
└── docker-compose.yml # Development environment
```

### Available Make Targets

**Development:**
- `make run` - Run server locally (in-memory storage)
- `make dev` - Run with auto-reload
- `make dev-with-db` - Run with PostgreSQL database
- `make deps` - Install Go dependencies

**Building:**
- `make build` - Build binary
- `make container-build` - Build container image
- `make clean` - Clean build artifacts

**Testing:**
- `make test` - Run all tests
- `make test-unit` - Run unit tests only
- `make test-integration` - Run integration tests only
- `make lint` - Run linter (requires golangci-lint)

**Database:**
- `make db-start` - Start PostgreSQL with Docker
- `make db-stop` - Stop PostgreSQL container

**Utilities:**
- `make fmt` - Format Go code
- `make version` - Show version information

### Testing

The project includes comprehensive test coverage:

```bash
# Run all tests
make test

# Run with coverage
go test -cover ./...

# Run integration tests with PostgreSQL
make test-integration
```

### Development with Database

```bash
# Start PostgreSQL
make db-start

# Run with database
make dev-with-db

# Stop database when done
make db-stop
```

### Production Deployment

```bash
# Build production container
make container-build

# Run with environment variables
podman run -d \
  -p 8080:8080 \
  -e OVIM_DATABASE_URL="postgres://..." \
  -e OVIM_ENVIRONMENT=production \
  -e OVIM_JWT_SECRET="your-secret" \
  ovim-backend:latest
```