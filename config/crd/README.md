# OVIM CRD-Based Org→VDC Architecture

This directory contains the Kubernetes Custom Resource Definitions (CRDs) for OVIM's new organization and virtual data center architecture.

## Overview

The new architecture separates identity management from resource management:

- **Organizations** are identity and catalog containers
- **Virtual Data Centers (VDCs)** are resource containers with quotas and isolation
- **Catalogs** manage template and application stack repositories

## Architecture Components

### 1. Organization CRD (`organization.yaml`)
- **Purpose**: Identity, RBAC, and catalog container
- **Namespace Scope**: Cluster-scoped
- **Creates**: `org-<name>` namespace for identity and catalogs
- **Features**:
  - Admin group management
  - Catalog resource tracking
  - RBAC sync status
  - Lifecycle management with finalizers

### 2. VirtualDataCenter CRD (`virtualdatacenter.yaml`)
- **Purpose**: Resource container with quotas and workload isolation
- **Namespace Scope**: Namespaced (lives in Organization namespaces)
- **Creates**: `vdc-<org>-<vdc>` namespace for workloads
- **Features**:
  - CPU, memory, storage quotas
  - Pod and VM limits
  - Network isolation policies
  - Resource usage tracking
  - LimitRange support for per-workload constraints

### 3. Catalog CRD (`catalog.yaml`)
- **Purpose**: Template and application repository management
- **Namespace Scope**: Namespaced (lives in Organization namespaces)
- **Features**:
  - Multi-source support (Git, OCI, S3, HTTP, local)
  - Content filtering and categorization
  - Access control per VDC
  - Automatic synchronization
  - Content inventory tracking

## Installation

### Quick Start
```bash
# Install all CRDs
./install.sh

# Dry run to see what would be installed
./install.sh --dry-run

# Validate existing installation
./install.sh --validate-only
```

### Manual Installation
```bash
kubectl apply -f organization.yaml
kubectl apply -f virtualdatacenter.yaml
kubectl apply -f catalog.yaml
```

### Uninstall (⚠️ Destructive)
```bash
# This will delete ALL Organization, VDC, and Catalog resources
./install.sh --uninstall
```

## Usage Examples

### Create an Organization
```yaml
apiVersion: ovim.io/v1
kind: Organization
metadata:
  name: acme-corp
spec:
  displayName: "Acme Corporation"
  description: "Main corporate organization"
  admins:
    - "acme-admins"
    - "platform-team"
  isEnabled: true
```

### Create a VDC
```yaml
apiVersion: ovim.io/v1
kind: VirtualDataCenter
metadata:
  name: production
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "Production Environment"
  description: "Production workloads"
  quota:
    cpu: "100"
    memory: "500Gi"
    storage: "10Ti"
    pods: 200
    virtualMachines: 50
  limitRange:
    minCpu: 100      # 0.1 core in millicores
    maxCpu: 8000     # 8 cores in millicores
    minMemory: 128   # 128 MiB
    maxMemory: 32768 # 32 GiB in MiB
  networkPolicy: isolated
```

### Create a Catalog
```yaml
apiVersion: ovim.io/v1
kind: Catalog
metadata:
  name: vm-templates
  namespace: org-acme-corp
spec:
  organizationRef: acme-corp
  displayName: "VM Templates"
  description: "Corporate VM template catalog"
  type: vm-template
  source:
    type: oci
    url: "registry.corp.com/vm-templates"
    refreshInterval: "1h"
  permissions:
    allowedVDCs: ["production", "staging"]
    readOnly: true
  isEnabled: true
```

## Resource Hierarchy

```
Organization (Cluster-scoped)
├── org-<name> (Namespace)
│   ├── Catalogs (VDC-scoped resources)
│   ├── VirtualDataCenter CRs
│   └── RBAC (RoleBindings for org admins)
│
└── VDCs create:
    └── vdc-<org>-<vdc> (Workload Namespace)
        ├── ResourceQuota
        ├── LimitRange
        ├── NetworkPolicy
        └── Workloads (Pods, VMs)
```

## Status Tracking

All CRDs include comprehensive status tracking:

### Organization Status
```yaml
status:
  namespace: org-acme-corp
  phase: Active
  vdcCount: 3
  lastRBACSync: "2024-09-12T10:30:00Z"
  conditions:
    - type: Ready
      status: "True"
      reason: NamespaceCreated
```

### VDC Status
```yaml
status:
  namespace: vdc-acme-corp-production
  phase: Active
  resourceUsage:
    cpuUsed: "45"
    memoryUsed: "200Gi"
    cpuPercentage: 45.0
    memoryPercentage: 40.0
  workloadCounts:
    totalPods: 25
    runningPods: 24
    totalVMs: 5
    runningVMs: 5
  lastMetricsUpdate: "2024-09-12T10:35:00Z"
```

### Catalog Status
```yaml
status:
  phase: Ready
  contentSummary:
    totalItems: 15
    vmTemplates: 10
    applicationStacks: 5
  syncStatus:
    lastSync: "2024-09-12T09:00:00Z"
    nextSyncScheduled: "2024-09-12T10:00:00Z"
```

## Integration with Database

The CRDs integrate with OVIM's database through:

1. **Database Migration**: Run `scripts/migrations/001_org_vdc_crd_migration.sql`
2. **Model Updates**: Use types from `pkg/models/crd_types.go`
3. **Controllers**: Deploy controllers to sync CRD status with database

## Next Steps

After installing the CRDs:

1. **Run Database Migration**:
   ```bash
   # Apply migration script
   psql -d ovim -f scripts/migrations/001_org_vdc_crd_migration.sql
   
   # Validate migration
   psql -d ovim -f scripts/migrations/validate_migration_001.sql
   ```

2. **Deploy Controllers**:
   - Organization Controller
   - VDC Controller
   - Catalog Controller
   - RBAC Sync Controller
   - Metrics Collection Controller

3. **Update Application Code**:
   - Use new CRD-aware models
   - Update API endpoints
   - Migrate existing data

4. **Test the System**:
   ```bash
   # Create test organization
   kubectl apply -f examples/organization.yaml
   
   # Verify namespace creation
   kubectl get ns org-acme-corp
   
   # Create test VDC
   kubectl apply -f examples/vdc.yaml
   
   # Verify VDC namespace and quotas
   kubectl get ns vdc-acme-corp-production
   kubectl get resourcequota -n vdc-acme-corp-production
   ```

## Troubleshooting

### CRD Not Installing
```bash
# Check cluster connection
kubectl cluster-info

# Check CRD status
kubectl get crd organizations.ovim.io -o yaml

# Check for validation errors
kubectl describe crd organizations.ovim.io
```

### Custom Resources Not Creating
```bash
# Check for validation errors
kubectl describe organization acme-corp

# Check controller logs (when deployed)
kubectl logs -n ovim-system deployment/organization-controller
```

### Resource Quotas Not Applied
```bash
# Check VDC status
kubectl get vdc production -n org-acme-corp -o yaml

# Check created namespace
kubectl get resourcequota -n vdc-acme-corp-production

# Check controller logs
kubectl logs -n ovim-system deployment/vdc-controller
```

## Version Information

- **CRD API Version**: `ovim.io/v1`
- **Migration Version**: `001`
- **Compatible with**: Kubernetes 1.24+, OpenShift 4.10+

## Support

For issues with the CRD installation or configuration, check:
1. Controller logs when deployed
2. CRD validation errors: `kubectl describe crd <crd-name>`
3. Custom resource status: `kubectl describe <resource-type> <resource-name>`