# OVIM Controller Documentation

## Overview

The OVIM (OpenShift Virtual Infrastructure Manager) Controller is a Kubernetes operator that manages multi-tenant virtual infrastructure on OpenShift/Kubernetes clusters. It implements a hierarchical organizational model with Organizations containing Virtual Data Centers (VDCs), each with their own resource quotas, RBAC, and catalog content.

## Architecture

The controller manages three main Custom Resource Definitions (CRDs):

1. **Organization** - Top-level tenant boundary
2. **VirtualDataCenter (VDC)** - Resource pools within organizations
3. **Catalog** - Template and content management

## Custom Resource Definitions (CRDs)

### Organization CRD

**Resource**: `organizations.ovim.io/v1`
**Scope**: Cluster-scoped
**Short Name**: `org`

Organizations represent the top-level tenant boundary in OVIM. Each organization gets its own namespace and manages multiple VDCs.

#### Specification Fields

- **displayName** (required): Human-readable organization name (1-255 chars)
- **description** (optional): Organization description (max 1000 chars)
- **admins** (required): Array of admin group names with access to this organization
- **isEnabled** (optional): Whether organization is active (default: true)
- **catalogs** (optional): Array of catalog resources managed by this organization

#### Status Fields

- **namespace**: Created organization namespace name
- **phase**: Current phase (Pending, Active, Failed, Terminating)
- **conditions**: Array of condition objects with type, status, and messages
- **vdcCount**: Number of VDCs in this organization
- **lastRBACSync**: Last time RBAC was synced to VDCs
- **observedGeneration**: Most recent generation observed by controller

#### Example

```yaml
apiVersion: ovim.io/v1
kind: Organization
metadata:
  name: acme-corp
spec:
  displayName: "ACME Corporation"
  description: "Main ACME Corp development organization"
  admins:
    - "acme-org-admins"
    - "platform-admins"
  isEnabled: true
  catalogs:
    - name: "vm-templates"
      type: "vm-template"
      enabled: true
```

### VirtualDataCenter CRD

**Resource**: `virtualdatacenters.ovim.io/v1`
**Scope**: Namespaced (lives in Organization namespaces)
**Short Name**: `vdc`

VDCs represent resource pools within organizations, providing quota management, workload isolation, and catalog access control.

#### Specification Fields

- **organizationRef** (required): Reference to parent Organization
- **displayName** (required): Human-readable VDC name (1-255 chars)
- **description** (optional): VDC description (max 1000 chars)
- **quota** (required): Resource quotas for this VDC
  - **cpu**: CPU quota in cores (pattern: `^[0-9]+$`)
  - **memory**: Memory quota (pattern: `^[0-9]+Gi$`)
  - **storage**: Storage quota (pattern: `^[0-9]+Ti$`)
  - **pods**: Maximum number of pods (default: 100)
  - **virtualMachines**: Maximum number of VMs (default: 50)
- **limitRange** (optional): Per-workload resource limits
  - **minCpu**: Minimum CPU per container/VM in millicores (default: 100)
  - **maxCpu**: Maximum CPU per container/VM in millicores
  - **minMemory**: Minimum memory per container/VM in MiB (default: 128)
  - **maxMemory**: Maximum memory per container/VM in MiB
- **networkPolicy** (optional): Network isolation policy (default, isolated, custom)
- **customNetworkConfig** (optional): Custom network configuration
- **catalogRestrictions** (optional): Restrict which org catalogs this VDC can access

#### Status Fields

- **namespace**: Created VDC workload namespace name
- **phase**: Current phase (Pending, Active, Failed, Suspended, Terminating)
- **conditions**: Array of condition objects
- **resourceUsage**: Current resource usage statistics
  - **cpuUsed**, **memoryUsed**, **storageUsed**: Currently used resources
  - **cpuPercentage**, **memoryPercentage**, **storagePercentage**: Usage percentages
- **workloadCounts**: Current workload counts
  - **totalPods**, **runningPods**: Pod counts
  - **totalVMs**, **runningVMs**: VM counts
- **lastMetricsUpdate**: Last time metrics were collected

#### Example

```yaml
apiVersion: ovim.io/v1
kind: VirtualDataCenter
metadata:
  name: development-vdc
  namespace: acme-corp
spec:
  organizationRef: "acme-corp"
  displayName: "Development Environment"
  description: "Development VDC for ACME applications"
  quota:
    cpu: "16"
    memory: "64Gi"
    storage: "2Ti"
    pods: 200
    virtualMachines: 25
  limitRange:
    minCpu: 100
    maxCpu: 8000
    minMemory: 128
    maxMemory: 16384
  networkPolicy: "isolated"
```

### Catalog CRD

**Resource**: `catalogs.ovim.io/v1`
**Scope**: Namespaced (lives in Organization namespaces)
**Short Name**: `cat`

Catalogs manage template and content distribution for organizations, supporting multiple source types and content filtering.

#### Specification Fields

- **organizationRef** (required): Organization that owns this catalog
- **displayName** (required): Human-readable catalog name (1-255 chars)
- **description** (optional): Catalog description (max 1000 chars)
- **type** (required): Content type (vm-template, application-stack, mixed)
- **source** (required): Source configuration
  - **type**: Source type (git, oci, s3, http, local)
  - **url**: Source URL (URI format)
  - **branch**: Git branch (default: "main")
  - **path**: Path within source (default: "/")
  - **credentials**: Secret name for authentication
  - **insecureSkipTLSVerify**: Skip TLS verification (default: false)
  - **refreshInterval**: Sync frequency (default: "1h")
- **filters** (optional): Content filtering rules
  - **includePatterns**: Glob patterns for files to include
  - **excludePatterns**: Glob patterns for files to exclude
  - **tags**: Required tags for content items
- **permissions** (optional): Access control
  - **allowedVDCs**: VDCs allowed to use this catalog
  - **allowedGroups**: User groups allowed to use this catalog
  - **readOnly**: Whether catalog is read-only (default: true)
- **isEnabled** (optional): Whether catalog is active (default: true)

#### Status Fields

- **phase**: Current phase (Pending, Syncing, Ready, Failed, Suspended)
- **conditions**: Array of condition objects
- **contentSummary**: Summary of catalog content
  - **totalItems**: Total number of catalog items
  - **vmTemplates**: Number of VM templates
  - **applicationStacks**: Number of application stacks
  - **categories**: Available content categories
- **syncStatus**: Source synchronization status
  - **lastSync**: Last successful sync timestamp
  - **lastSyncAttempt**: Last sync attempt timestamp
  - **syncErrors**: Recent sync errors with retry counts
  - **nextSyncScheduled**: When next sync is scheduled

#### Example

```yaml
apiVersion: ovim.io/v1
kind: Catalog
metadata:
  name: vm-templates
  namespace: acme-corp
spec:
  organizationRef: "acme-corp"
  displayName: "VM Templates"
  description: "Standard VM templates for ACME development"
  type: "vm-template"
  source:
    type: "git"
    url: "https://github.com/acme/vm-templates.git"
    branch: "main"
    path: "/templates"
    refreshInterval: "30m"
  filters:
    includePatterns:
      - "*.yaml"
      - "*.yml"
    tags:
      - "approved"
      - "production-ready"
  permissions:
    allowedGroups:
      - "developers"
      - "devops"
    readOnly: true
```

## Controller Components

### 1. Organization Controller

**File**: `controllers/organization_controller.go`
**Watches**: Organization CRDs

#### Responsibilities
- Creates organization namespaces
- Sets up RBAC for organization admins
- Manages organization lifecycle
- Tracks VDC count and resource allocation
- Handles organization deletion with proper cleanup

#### Key Functions
- Namespace creation and management
- RBAC synchronization with admin groups
- Resource quota enforcement at organization level
- VDC discovery and counting
- Status condition management

### 2. VDC Controller

**File**: `controllers/vdc_controller.go`
**Watches**: VirtualDataCenter CRDs

#### Responsibilities
- Creates VDC workload namespaces
- Sets up resource quotas and limit ranges
- Configures network policies
- Manages VDC-specific RBAC
- Collects and reports resource usage metrics

#### Key Functions
- Workload namespace provisioning
- Kubernetes ResourceQuota and LimitRange creation
- NetworkPolicy configuration based on isolation settings
- Metrics collection from workload namespaces
- VDC status and condition management

### 3. VM Controller

**File**: `controllers/vm_controller.go`
**Watches**: VirtualMachine CRDs (KubeVirt)

#### Responsibilities
- Monitors VM lifecycle within VDCs
- Enforces VDC resource limits
- Updates VDC resource usage statistics
- Manages VM placement and scheduling

### 4. Metrics Controller

**File**: `controllers/metrics_controller.go`
**Watches**: Resource usage across VDCs

#### Responsibilities
- Collects resource usage metrics from all VDCs
- Updates VDC status with current usage
- Monitors quota utilization
- Generates alerts for resource thresholds

### 5. RBAC Sync Controller

**File**: `controllers/rbac_sync_controller.go`
**Watches**: Organization and VDC changes

#### Responsibilities
- Synchronizes RBAC changes across organization hierarchy
- Propagates organization admin permissions to VDCs
- Manages VDC-specific permissions
- Handles admin group changes

## RBAC Configuration

The controller implements a hierarchical RBAC model with two main cluster roles:

### Org Admin Cluster Role

**File**: `config/rbac/org-admin-clusterrole.yaml`
**Role Name**: `ovim:org-admin`

#### Permissions
- **Namespace Management**: Get, list, watch namespaces
- **Resource Quotas**: Full CRUD on ResourceQuotas and LimitRanges
- **VDC Management**: Full CRUD on VirtualDataCenter resources within organization
- **Basic Resources**: Full CRUD on pods, services, endpoints, configmaps, secrets
- **RBAC**: Full CRUD on roles and rolebindings within organization namespace
- **Events**: Read access for debugging
- **Network Policies**: Full CRUD for network segmentation

### VDC Admin Cluster Role

**File**: `config/rbac/vdc-admin-clusterrole.yaml`
**Role Name**: `ovim:vdc-admin`

#### Permissions
- **Namespace Visibility**: Get, list, watch namespaces
- **Pod Management**: Full CRUD on pods with exec, attach, port-forward capabilities
- **Core Resources**: Full CRUD on services, endpoints, PVCs, configmaps, secrets
- **Workloads**: Full CRUD on deployments, replicasets, statefulsets, daemonsets
- **Scaling**: Update permissions on deployment/replicaset/statefulset scale subresources
- **Batch Jobs**: Full CRUD on jobs and cronjobs
- **KubeVirt VMs**: Full CRUD on VirtualMachines and VirtualMachineInstances
- **VM Operations**: Update permissions on VM start/stop/restart subresources
- **CDI DataVolumes**: Full CRUD on DataVolumes for storage management
- **Network Policies**: Full CRUD within VDC namespace
- **Ingress**: Full CRUD on ingress resources
- **Service Accounts**: Full CRUD within VDC namespace
- **RBAC**: Full CRUD on roles and rolebindings within VDC namespace
- **Events**: Read access for debugging
- **Resource Quotas**: Read-only access to view limits
- **Metrics**: Get and list access to pod and node metrics

> **Note**: The OVIM controller must have all permissions that it grants to VDC admins. Recent updates ensure the controller has comprehensive permissions to create VDC admin role bindings successfully.

## Resource Hierarchy

```
Cluster
├── Organization (cluster-scoped)
│   ├── Organization Namespace
│   │   ├── VirtualDataCenter CRDs
│   │   ├── Catalog CRDs
│   │   └── Organization Admin RoleBindings
│   ├── VDC Workload Namespace 1
│   │   ├── Resource Quotas
│   │   ├── Limit Ranges
│   │   ├── Network Policies
│   │   ├── VDC Admin RoleBindings
│   │   └── Workloads (Pods, VMs, etc.)
│   └── VDC Workload Namespace N
│       └── ...
└── Organization N
    └── ...
```

## Operational Workflows

### Organization Creation
1. User creates Organization CRD
2. Organization Controller validates specification
3. Controller creates organization namespace
4. Controller sets up RBAC for admin groups
5. Controller updates organization status to "Active"

### VDC Creation
1. User creates VirtualDataCenter CRD in organization namespace
2. VDC Controller validates quota against organization limits
3. Controller creates VDC workload namespace
4. Controller creates ResourceQuota and LimitRange objects
5. Controller configures NetworkPolicy based on isolation settings
6. Controller sets up VDC admin RBAC
7. Controller updates VDC status to "Active"

### Resource Monitoring
1. Metrics Controller periodically scans all VDC namespaces
2. Controller collects resource usage from Kubernetes metrics
3. Controller calculates usage percentages against quotas
4. Controller updates VDC status with current metrics
5. Controller triggers alerts if thresholds are exceeded

## Installation and Configuration

### Prerequisites
- OpenShift 4.8+ or Kubernetes 1.24+
- KubeVirt operator (for VM support)
- CDI operator (for storage management)
- Cluster admin permissions for OVIM installation

### Installation Steps
1. Install CRDs: `kubectl apply -f config/crd/`
2. Install RBAC: `kubectl apply -f config/rbac/`
3. Deploy controller: `kubectl apply -f config/controller/`
4. Verify installation: `kubectl get pods -n ovim-system`

### Configuration
The controller supports configuration via environment variables and config files. Key configuration areas include:

- **Database Connection**: PostgreSQL connection for persistent storage
- **KubeVirt Integration**: VM provisioning and management
- **OpenShift Integration**: Template and project management
- **Metrics Collection**: Resource usage monitoring intervals
- **RBAC Sync**: Organization admin group synchronization

## Monitoring and Troubleshooting

### Controller Logs
```bash
kubectl logs -n ovim-system deployment/ovim-controller
```

### Resource Status
```bash
kubectl get organizations
kubectl get vdc -A
kubectl get catalogs -A
```

### Common Issues
1. **Organization stuck in Pending**: Check RBAC permissions and namespace creation
2. **VDC quota validation failures**: Verify organization resource limits
3. **RBAC sync issues**: Check admin group configurations and cluster role bindings
4. **Metrics collection failures**: Verify metrics server and controller connectivity

## Security Considerations

### Multi-tenancy
- Organizations provide strong namespace isolation
- VDCs inherit organization boundaries with additional restrictions
- RBAC prevents cross-organization access
- Network policies enforce VDC-level isolation

### Resource Protection
- Resource quotas prevent resource exhaustion
- Limit ranges control individual workload sizes
- Admission controllers validate resource requests
- Metrics monitoring enables proactive management

### Access Control
- Admin groups provide centralized access management
- Role aggregation allows flexible permission models
- Audit logging tracks all administrative actions
- OIDC integration supports enterprise authentication