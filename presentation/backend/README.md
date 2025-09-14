# OVIM Backend API Documentation

## Overview

The OVIM Backend API provides a comprehensive REST API for managing virtual infrastructure in multi-tenant environments. It serves as the primary interface for the OVIM web UI and supports integration with external systems through a well-defined API contract.

## Architecture

The backend is built using Go with the Gin web framework and follows a layered architecture:

```
┌─────────────────────────────────────────┐
│             Web UI / CLI                │
├─────────────────────────────────────────┤
│             REST API Layer              │
├─────────────────────────────────────────┤
│          Authentication Layer           │
├─────────────────────────────────────────┤
│           Business Logic                │
├─────────────────────────────────────────┤
│         Storage Abstraction             │
├─────────────────────────────────────────┤
│    PostgreSQL    │    Kubernetes API    │
└─────────────────────────────────────────┘
```

### Key Components

1. **API Server** (`pkg/api/server.go`): Main HTTP server with routing and middleware
2. **Authentication** (`pkg/auth/`): JWT-based auth with OIDC support
3. **Storage Layer** (`pkg/storage/`): Abstracted data persistence
4. **Controllers Integration**: Direct interaction with Kubernetes controllers
5. **VM Provisioning** (`pkg/kubevirt/`): KubeVirt integration for VM management
6. **OpenShift Integration** (`pkg/openshift/`): Template and project management

## Authentication & Authorization

### Authentication Methods

#### 1. JWT Token Authentication
- **Endpoint**: `POST /api/v1/auth/login`
- **Method**: Username/password login with JWT token response
- **Token Lifetime**: Configurable (default: 24 hours)
- **Header Format**: `Authorization: Bearer <token>`

#### 2. OIDC Integration
- **Auth URL**: `GET /api/v1/auth/oidc/auth-url`
- **Callback**: `POST /api/v1/auth/oidc/callback`
- **Supported Providers**: Any OIDC-compliant provider (Keycloak, Auth0, etc.)
- **Role Mapping**: Automatic role assignment based on OIDC groups/roles

### Authorization Levels

#### System Admin
- Full access to all organizations and users
- Can create, modify, and delete organizations
- Can manage global catalog sources
- Can view system-wide metrics and alerts

#### Organization Admin
- Full access within assigned organization
- Can create and manage VDCs within organization
- Can manage organization users and resources
- Can configure organization catalog sources

#### VDC Admin
- Full access within assigned VDC
- Can deploy and manage VMs and applications
- Can view VDC-specific metrics
- Cannot modify VDC quotas or policies

#### User
- Limited access to assigned resources
- Can view allocated VMs and applications
- Cannot perform administrative actions

## API Endpoints

### Base Configuration
- **Base URL**: `/api/v1`
- **Content Type**: `application/json`
- **Response Format**: JSON
- **Authentication**: Required for all endpoints except `/health`, `/version`, and auth endpoints

### Health & Status Endpoints

#### Health Check
```
GET /health
```
**Response**: `200 OK`
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": {
    "database": "healthy",
    "kubernetes": "healthy",
    "kubevirt": "healthy"
  }
}
```

#### Version Information
```
GET /version
```
**Response**: `200 OK`
```json
{
  "version": "v1.0.0",
  "gitCommit": "abc123",
  "buildDate": "2024-01-15T10:00:00Z",
  "goVersion": "go1.21.0"
}
```

### Authentication Endpoints

#### User Login
```
POST /api/v1/auth/login
```
**Request Body**:
```json
{
  "username": "admin",
  "password": "password123"
}
```
**Response**: `200 OK`
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "user-123",
    "username": "admin",
    "role": "system_admin",
    "organizationId": "org-456"
  },
  "expiresAt": "2024-01-16T10:30:00Z"
}
```

#### User Logout
```
POST /api/v1/auth/logout
```
**Response**: `200 OK`
```json
{
  "message": "Successfully logged out"
}
```

#### Authentication Info
```
GET /api/v1/auth/info
```
**Response**: `200 OK`
```json
{
  "oidcEnabled": true,
  "oidcIssuer": "https://auth.example.com",
  "supportedMethods": ["password", "oidc"]
}
```

#### OIDC Auth URL
```
GET /api/v1/auth/oidc/auth-url
```
**Response**: `200 OK`
```json
{
  "authUrl": "https://auth.example.com/auth?client_id=...",
  "state": "random-state-string"
}
```

#### OIDC Callback
```
POST /api/v1/auth/oidc/callback
```
**Request Body**:
```json
{
  "code": "auth-code",
  "state": "random-state-string"
}
```
**Response**: Same as login endpoint

### Organization Management

#### List Organizations
```
GET /api/v1/organizations
```
**Authorization**: System Admin only
**Response**: `200 OK`
```json
{
  "organizations": [
    {
      "id": "org-123",
      "name": "acme-corp",
      "displayName": "ACME Corporation",
      "description": "Main ACME development organization",
      "status": "Active",
      "vdcCount": 3,
      "createdAt": "2024-01-01T00:00:00Z",
      "namespace": "acme-corp"
    }
  ],
  "total": 1
}
```

#### Create Organization
```
POST /api/v1/organizations
```
**Authorization**: System Admin only
**Request Body**:
```json
{
  "name": "acme-corp",
  "displayName": "ACME Corporation",
  "description": "Main ACME development organization",
  "adminGroups": ["acme-admins", "platform-admins"],
  "quotas": {
    "cpu": "100",
    "memory": "500Gi",
    "storage": "10Ti"
  }
}
```
**Response**: `201 Created`
```json
{
  "id": "org-123",
  "name": "acme-corp",
  "displayName": "ACME Corporation",
  "status": "Pending",
  "namespace": "acme-corp",
  "createdAt": "2024-01-15T10:30:00Z"
}
```

#### Get Organization
```
GET /api/v1/organizations/{id}
```
**Authorization**: System Admin only
**Response**: `200 OK`
```json
{
  "id": "org-123",
  "name": "acme-corp",
  "displayName": "ACME Corporation",
  "description": "Main ACME development organization",
  "status": "Active",
  "namespace": "acme-corp",
  "adminGroups": ["acme-admins"],
  "quotas": {
    "cpu": "100",
    "memory": "500Gi",
    "storage": "10Ti"
  },
  "usage": {
    "cpu": "45",
    "memory": "200Gi",
    "storage": "3Ti"
  },
  "vdcs": [
    {
      "id": "vdc-dev",
      "name": "development",
      "displayName": "Development Environment",
      "status": "Active"
    }
  ],
  "createdAt": "2024-01-01T00:00:00Z",
  "updatedAt": "2024-01-15T10:30:00Z"
}
```

#### Update Organization
```
PUT /api/v1/organizations/{id}
```
**Authorization**: System Admin only
**Request Body**: Same as create, all fields optional
**Response**: `200 OK` with updated organization object

#### Delete Organization
```
DELETE /api/v1/organizations/{id}
```
**Authorization**: System Admin only
**Response**: `204 No Content`

#### Get Organization Status
```
GET /api/v1/organizations/{id}/status
```
**Authorization**: System Admin only
**Response**: `200 OK`
```json
{
  "phase": "Active",
  "conditions": [
    {
      "type": "Ready",
      "status": "True",
      "lastTransitionTime": "2024-01-01T00:00:00Z",
      "reason": "OrganizationReady",
      "message": "Organization is ready and functional"
    }
  ],
  "vdcCount": 3,
  "lastRBACSync": "2024-01-15T10:30:00Z"
}
```

### VDC Management

#### List VDCs
```
GET /api/v1/vdcs
```
**Authorization**: System Admin, Org Admin
**Query Parameters**:
- `organization`: Filter by organization ID
- `status`: Filter by status (Active, Pending, etc.)
**Response**: `200 OK`
```json
{
  "vdcs": [
    {
      "id": "vdc-dev",
      "name": "development",
      "displayName": "Development Environment",
      "organizationId": "org-123",
      "organizationName": "acme-corp",
      "status": "Active",
      "namespace": "acme-corp-dev",
      "quotas": {
        "cpu": "16",
        "memory": "64Gi",
        "storage": "2Ti",
        "pods": 200,
        "virtualMachines": 25
      },
      "usage": {
        "cpu": "8",
        "memory": "32Gi",
        "storage": "500Gi",
        "cpuPercentage": 50.0,
        "memoryPercentage": 50.0,
        "storagePercentage": 25.0
      },
      "workloads": {
        "totalPods": 45,
        "runningPods": 42,
        "totalVMs": 8,
        "runningVMs": 6
      },
      "createdAt": "2024-01-02T00:00:00Z"
    }
  ],
  "total": 1
}
```

#### Create VDC
```
POST /api/v1/vdcs
```
**Authorization**: System Admin, Org Admin
**Request Body**:
```json
{
  "name": "development",
  "displayName": "Development Environment",
  "description": "Development VDC for ACME applications",
  "organizationId": "org-123",
  "quotas": {
    "cpu": "16",
    "memory": "64Gi",
    "storage": "2Ti",
    "pods": 200,
    "virtualMachines": 25
  },
  "limitRanges": {
    "minCpu": 100,
    "maxCpu": 8000,
    "minMemory": 128,
    "maxMemory": 16384
  },
  "networkPolicy": "isolated",
  "catalogRestrictions": ["vm-templates"]
}
```
**Response**: `201 Created` with VDC object

#### Get VDC
```
GET /api/v1/vdcs/{id}
```
**Authorization**: System Admin, Org Admin, VDC Admin
**Response**: `200 OK` with VDC object including detailed status

#### Update VDC
```
PUT /api/v1/vdcs/{id}
```
**Authorization**: System Admin, Org Admin
**Request Body**: Same as create, all fields optional
**Response**: `200 OK` with updated VDC object

#### Delete VDC
```
DELETE /api/v1/vdcs/{id}
```
**Authorization**: System Admin, Org Admin
**Response**: `204 No Content`

#### Get VDC Resource Usage
```
GET /api/v1/vdcs/{id}/resources
```
**Authorization**: System Admin, Org Admin, VDC Admin
**Response**: `200 OK`
```json
{
  "quotas": {
    "cpu": "16",
    "memory": "64Gi",
    "storage": "2Ti"
  },
  "usage": {
    "cpu": "8",
    "memory": "32Gi",
    "storage": "500Gi",
    "cpuPercentage": 50.0,
    "memoryPercentage": 50.0,
    "storagePercentage": 25.0
  },
  "limits": {
    "pods": 200,
    "virtualMachines": 25
  },
  "current": {
    "pods": 45,
    "virtualMachines": 8
  },
  "lastUpdated": "2024-01-15T10:30:00Z"
}
```

### Virtual Machine Management

#### List VMs
```
GET /api/v1/vms
```
**Authorization**: All authenticated users (filtered by access)
**Query Parameters**:
- `vdc`: Filter by VDC ID
- `status`: Filter by VM status
- `organization`: Filter by organization (System Admin only)
**Response**: `200 OK`
```json
{
  "vms": [
    {
      "id": "vm-web1",
      "name": "web-server-1",
      "displayName": "Web Server 1",
      "vdcId": "vdc-dev",
      "vdcName": "development",
      "organizationId": "org-123",
      "status": "Running",
      "powerState": "PoweredOn",
      "resources": {
        "cpu": 2,
        "memory": "4Gi",
        "storage": "50Gi"
      },
      "networking": {
        "interfaces": [
          {
            "name": "default",
            "ipAddress": "10.244.1.5",
            "macAddress": "52:54:00:12:34:56"
          }
        ]
      },
      "template": {
        "id": "template-ubuntu-20",
        "name": "Ubuntu 20.04 LTS"
      },
      "createdAt": "2024-01-10T10:00:00Z",
      "lastUpdated": "2024-01-15T10:30:00Z"
    }
  ],
  "total": 1
}
```

#### Create VM
```
POST /api/v1/vms
```
**Authorization**: All authenticated users (within accessible VDCs)
**Request Body**:
```json
{
  "name": "web-server-1",
  "displayName": "Web Server 1",
  "description": "Primary web server for application",
  "vdcId": "vdc-dev",
  "templateId": "template-ubuntu-20",
  "resources": {
    "cpu": 2,
    "memory": "4Gi",
    "storage": "50Gi"
  },
  "networking": {
    "interfaces": [
      {
        "name": "default",
        "networkName": "pod-network"
      }
    ]
  },
  "userData": {
    "cloudInit": {
      "users": [
        {
          "name": "admin",
          "sudo": "ALL=(ALL) NOPASSWD:ALL",
          "ssh_authorized_keys": ["ssh-rsa AAAAB3..."]
        }
      ]
    }
  }
}
```
**Response**: `201 Created` with VM object

#### Get VM
```
GET /api/v1/vms/{id}
```
**Authorization**: All authenticated users (within accessible VDCs)
**Response**: `200 OK` with detailed VM object including status and metrics

#### Get VM Status
```
GET /api/v1/vms/{id}/status
```
**Authorization**: All authenticated users (within accessible VDCs)
**Response**: `200 OK`
```json
{
  "phase": "Running",
  "powerState": "PoweredOn",
  "conditions": [
    {
      "type": "Ready",
      "status": "True",
      "lastTransitionTime": "2024-01-10T10:05:00Z"
    }
  ],
  "guestAgent": {
    "connected": true,
    "version": "0.59.0"
  },
  "interfaces": [
    {
      "name": "default",
      "ipAddress": "10.244.1.5",
      "ipAddresses": ["10.244.1.5"],
      "mac": "52:54:00:12:34:56",
      "interfaceName": "eth0"
    }
  ],
  "lastUpdated": "2024-01-15T10:30:00Z"
}
```

#### Update VM Power State
```
PUT /api/v1/vms/{id}/power
```
**Authorization**: All authenticated users (within accessible VDCs)
**Request Body**:
```json
{
  "action": "start|stop|restart|pause|unpause"
}
```
**Response**: `200 OK`
```json
{
  "message": "Power action initiated",
  "action": "start",
  "status": "In Progress"
}
```

#### Delete VM
```
DELETE /api/v1/vms/{id}
```
**Authorization**: All authenticated users (within accessible VDCs)
**Response**: `204 No Content`

### Catalog Management

#### List Templates
```
GET /api/v1/catalog/templates
```
**Authorization**: All authenticated users
**Query Parameters**:
- `organization`: Filter by organization
- `type`: Filter by template type
- `category`: Filter by category
**Response**: `200 OK`
```json
{
  "templates": [
    {
      "id": "template-ubuntu-20",
      "name": "ubuntu-20-04-lts",
      "displayName": "Ubuntu 20.04 LTS",
      "description": "Ubuntu 20.04 LTS base template",
      "type": "vm-template",
      "category": "Operating Systems",
      "version": "1.0.0",
      "organizationId": "org-123",
      "catalogId": "catalog-vm-templates",
      "resources": {
        "cpu": 1,
        "memory": "2Gi",
        "storage": "20Gi"
      },
      "osInfo": {
        "type": "linux",
        "distribution": "ubuntu",
        "version": "20.04"
      },
      "tags": ["ubuntu", "lts", "server"],
      "isPublic": true,
      "isEnabled": true,
      "createdAt": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1
}
```

#### Get Template
```
GET /api/v1/catalog/templates/{id}
```
**Authorization**: All authenticated users
**Response**: `200 OK` with detailed template object including deployment configuration

#### Get Catalog Sources
```
GET /api/v1/catalog/sources
```
**Authorization**: All authenticated users
**Response**: `200 OK`
```json
{
  "sources": [
    {
      "id": "source-vm-templates",
      "name": "vm-templates",
      "displayName": "VM Templates",
      "type": "git",
      "url": "https://github.com/acme/vm-templates.git",
      "status": "Ready",
      "lastSync": "2024-01-15T09:00:00Z",
      "itemCount": 25
    }
  ],
  "total": 1
}
```

### User Management

#### List Users
```
GET /api/v1/users
```
**Authorization**: System Admin only
**Query Parameters**:
- `organization`: Filter by organization
- `role`: Filter by role
**Response**: `200 OK`
```json
{
  "users": [
    {
      "id": "user-123",
      "username": "john.doe",
      "email": "john.doe@acme.com",
      "firstName": "John",
      "lastName": "Doe",
      "role": "org_admin",
      "organizationId": "org-123",
      "organizationName": "acme-corp",
      "isEnabled": true,
      "lastLogin": "2024-01-15T09:00:00Z",
      "createdAt": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1
}
```

#### Create User
```
POST /api/v1/users
```
**Authorization**: System Admin only
**Request Body**:
```json
{
  "username": "john.doe",
  "email": "john.doe@acme.com",
  "firstName": "John",
  "lastName": "Doe",
  "password": "SecurePassword123!",
  "role": "org_admin",
  "organizationId": "org-123",
  "isEnabled": true
}
```
**Response**: `201 Created` with user object (password excluded)

#### Get User
```
GET /api/v1/users/{id}
```
**Authorization**: System Admin only
**Response**: `200 OK` with user object

#### Update User
```
PUT /api/v1/users/{id}
```
**Authorization**: System Admin only
**Request Body**: Same as create, all fields optional
**Response**: `200 OK` with updated user object

#### Delete User
```
DELETE /api/v1/users/{id}
```
**Authorization**: System Admin only
**Response**: `204 No Content`

### User Profile Endpoints

#### Get User Organization
```
GET /api/v1/profile/organization
```
**Authorization**: All authenticated users
**Response**: `200 OK` with user's organization details

#### Get User VDCs
```
GET /api/v1/profile/vdcs
```
**Authorization**: All authenticated users
**Response**: `200 OK` with VDCs accessible to the user

### Dashboard & Metrics

#### Get Dashboard Summary
```
GET /api/v1/dashboard/summary
```
**Authorization**: All authenticated users (data filtered by access level)
**Response**: `200 OK`
```json
{
  "organizations": {
    "total": 5,
    "active": 4,
    "inactive": 1
  },
  "vdcs": {
    "total": 15,
    "active": 12,
    "pending": 2,
    "failed": 1
  },
  "virtualMachines": {
    "total": 48,
    "running": 35,
    "stopped": 8,
    "pending": 5
  },
  "resources": {
    "totalCpu": "500",
    "usedCpu": "245",
    "totalMemory": "2000Gi",
    "usedMemory": "1200Gi",
    "totalStorage": "50Ti",
    "usedStorage": "25Ti"
  },
  "alerts": {
    "critical": 2,
    "warning": 5,
    "info": 8
  },
  "lastUpdated": "2024-01-15T10:30:00Z"
}
```

#### Get Alert Summary
```
GET /api/v1/alerts/summary
```
**Authorization**: All authenticated users (data filtered by access level)
**Response**: `200 OK`
```json
{
  "alerts": [
    {
      "id": "alert-cpu-high",
      "type": "resource",
      "severity": "warning",
      "title": "High CPU Usage",
      "description": "VDC development CPU usage is above 80%",
      "resourceType": "vdc",
      "resourceId": "vdc-dev",
      "threshold": 80,
      "currentValue": 85,
      "triggeredAt": "2024-01-15T10:25:00Z",
      "acknowledged": false
    }
  ],
  "summary": {
    "critical": 2,
    "warning": 5,
    "info": 8
  }
}
```

## Error Handling

### HTTP Status Codes

- **200 OK**: Successful GET, PUT operations
- **201 Created**: Successful POST operations
- **204 No Content**: Successful DELETE operations
- **400 Bad Request**: Invalid request data or parameters
- **401 Unauthorized**: Authentication required or invalid token
- **403 Forbidden**: User lacks required permissions
- **404 Not Found**: Resource not found
- **409 Conflict**: Resource conflict (duplicate names, etc.)
- **422 Unprocessable Entity**: Validation errors
- **500 Internal Server Error**: Server-side errors

### Error Response Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed for request data",
    "details": [
      {
        "field": "name",
        "message": "Name must be between 1 and 255 characters"
      },
      {
        "field": "quotas.cpu",
        "message": "CPU quota must be a positive integer"
      }
    ],
    "timestamp": "2024-01-15T10:30:00Z",
    "requestId": "req-123456"
  }
}
```

### Common Error Codes

- **AUTHENTICATION_REQUIRED**: Token missing or invalid
- **PERMISSION_DENIED**: User lacks required role/permissions
- **VALIDATION_ERROR**: Request data validation failed
- **RESOURCE_NOT_FOUND**: Requested resource doesn't exist
- **RESOURCE_CONFLICT**: Resource name conflicts or constraint violations
- **QUOTA_EXCEEDED**: Resource allocation exceeds available quota
- **DEPENDENCY_ERROR**: Operation blocked by dependent resources
- **EXTERNAL_SERVICE_ERROR**: Kubernetes/OpenShift API errors

## Security Features

### Token Management
- JWT tokens with configurable expiration
- Automatic token refresh for UI sessions
- Secure token storage recommendations
- Token revocation on logout

### Input Validation
- Comprehensive request validation
- SQL injection prevention
- XSS protection in responses
- File upload restrictions

### Access Control
- Role-based access control (RBAC)
- Resource-level permissions
- Organization and VDC boundaries
- Audit logging for all operations

### Security Headers
- CORS configuration
- Security headers for web UI
- Rate limiting on authentication endpoints
- Request logging and monitoring

## Integration Points

### Kubernetes API
- Direct CRD management
- Namespace and RBAC creation
- Resource quota enforcement
- Event monitoring

### KubeVirt
- VM lifecycle management
- Resource allocation
- Storage provisioning
- Network configuration

### OpenShift
- Template management
- Project integration
- Route configuration
- Image stream handling

### PostgreSQL
- User and organization data
- Configuration storage
- Audit logs
- Session management

## Development & Testing

### Local Development
1. Start PostgreSQL database
2. Configure environment variables
3. Run `go run ./cmd/ovim-server`
4. API available at `http://localhost:8080`

### Testing
- Unit tests: `make test-unit`
- Integration tests: `make test-integration`
- API tests: `make test-api`
- Coverage report: `make coverage`

### API Documentation
- OpenAPI/Swagger spec generation
- Interactive API explorer
- Code examples and SDKs
- Postman collection

This comprehensive API provides all necessary functionality for managing virtual infrastructure in a multi-tenant environment, with strong security, comprehensive error handling, and extensive integration capabilities.