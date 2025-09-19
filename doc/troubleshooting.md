# OVIM Troubleshooting Guide

## Overview

This guide covers common issues encountered during OVIM deployment and operation, along with their solutions. These issues have been identified and resolved through production deployment experience.

## Deployment Issues

### 1. PostgreSQL PVC Binding Failure

**Symptom**: PostgreSQL pod remains in `Pending` state with PVC binding issues
```
Warning  FailedScheduling  pod/ovim-postgresql-0  pod has unbound immediate PersistentVolumeClaims
```

**Root Cause**: Missing or incorrect storage class configuration

**Solution**: The deployment script now automatically detects and configures storage classes
- Dynamic storage class detection implemented in `scripts/deploy-stack.sh`
- Automatically uses default storage class or first available one
- Fallback behavior for environments without default storage class

**Manual Fix** (if needed):
```bash
# Check available storage classes
kubectl get storageclass

# Update PostgreSQL PVC with correct storage class
kubectl patch pvc data-ovim-postgresql-0 -n ovim-system -p '{"spec":{"storageClassName":"your-storage-class"}}'
```

### 2. VDC Creation Stuck in Pending Status

**Symptom**: VDCs remain in "Pending" phase with RBAC permission errors
```
Error: rolebindings.rbac.authorization.k8s.io "vdc-admin-org-admins" is forbidden:
user "system:serviceaccount:ovim-system:ovim-controller" is attempting to grant
RBAC permissions not currently held
```

**Root Cause**: OVIM controller lacks permissions to create VDC admin role bindings

**Solution**: Enhanced RBAC permissions have been added to the controller
- Added all permissions required for VDC admin role binding creation
- Updated deployment scripts with comprehensive RBAC configuration
- Controller can now grant: `persistentvolumeclaims`, `serviceaccounts`, `pods/attach`, `apps/*`, `batch/*`, etc.

**Verification**:
```bash
# Check VDC status
kubectl get virtualdatacenters -A

# Check controller permissions
kubectl auth can-i create persistentvolumeclaims --as=system:serviceaccount:ovim-system:ovim-controller
```

### 3. VDC Storage Quota Validation Errors

**Symptom**: VDC creation fails with CRD validation errors
```
Error: VirtualDataCenter.ovim.io "dev" is invalid:
spec.quota.storage: Invalid value: "4.88Ti": spec.quota.storage in body should match '^[0-9]+Ti$'
```

**Root Cause**: Storage quota conversion from GB to Ti created fractional values

**Solution**: Fixed storage quota conversion with proper rounding
- Changed from `req.StorageQuota/1024` to `(req.StorageQuota+1023)/1024`
- Ensures whole number Ti values that pass CRD validation
- Example: 5000GB now converts to 5Ti instead of 4.88Ti

**Code Location**: `pkg/api/vdc_handlers.go:183` and `line 317`

## Runtime Issues

### 4. Controller Race Condition Conflicts

**Symptom**: Controllers fail to update status with conflict errors
```
Error: Operation cannot be fulfilled on organizations.ovim.io "my-company":
the object has been modified; please apply your changes to the latest version
```

**Root Cause**: Multiple controllers updating the same resource simultaneously

**Solution**: Implemented retry logic with optimistic concurrency control
- Added `retry.RetryOnConflict()` patterns in both organization and RBAC controllers
- Proper resource version handling
- Prevents status update failures during concurrent operations

**Code Locations**:
- `controllers/organization_controller.go:109-124`
- `controllers/rbac_sync_controller.go:80-94`

### 5. Platform Detection Issues

**Symptom**: Deployment script fails to detect OpenShift vs Kubernetes platform
```
Error: Kubernetes ingress not found (running on OpenShift)
```

**Root Cause**: Platform-agnostic verification logic was missing

**Solution**: Added platform-aware verification and deployment logic
- Auto-detection of OpenShift vs Kubernetes environments
- Separate handling for Routes (OpenShift) vs Ingress (Kubernetes)
- Platform-specific resource validation

**Code Location**: `scripts/deploy-stack.sh:588-607`, `scripts/deploy-stack.sh:677-695`

## Best Practices

### Storage Configuration
1. **Always verify storage classes** before deployment
2. **Use default storage classes** when available
3. **Test PVC binding** in target environment

### RBAC Configuration
1. **Verify controller permissions** after deployment
2. **Test VDC creation** to ensure RBAC is working
3. **Monitor controller logs** for permission errors

### Resource Management
1. **Use whole number storage quotas** (avoid fractional GB values)
2. **Monitor resource usage** to prevent quota exhaustion
3. **Implement proper resource limits** for workloads

### Platform Deployment
1. **Test deployment scripts** in target environment
2. **Verify platform-specific resources** (Routes vs Ingress)
3. **Use environment-appropriate configurations**

## Monitoring and Diagnostics

### Key Log Locations
```bash
# Controller logs
kubectl logs -n ovim-system deployment/ovim-controller --tail=50

# Server logs
kubectl logs -n ovim-system deployment/ovim-server --tail=50

# Database logs
kubectl logs -n ovim-system statefulset/ovim-postgresql --tail=50
```

### Health Checks
```bash
# Check all OVIM components
kubectl get pods -n ovim-system

# Check VDC status across all orgs
kubectl get virtualdatacenters -A

# Check resource quotas
kubectl get resourcequota -A | grep vdc

# Verify RBAC configuration
kubectl get clusterrolebinding | grep ovim
```

### Common Commands
```bash
# Force restart controller
kubectl rollout restart deployment/ovim-controller -n ovim-system

# Check storage classes
kubectl get storageclass

# Test RBAC permissions
kubectl auth can-i create virtualdatacenters.ovim.io --as=system:serviceaccount:ovim-system:ovim-controller

# Clean up failed pods
kubectl delete pods -n ovim-system --field-selector=status.phase=Failed --force
```

## Recent Fixes Summary

The following issues have been identified and resolved in recent releases:

- ✅ **Storage Quota Validation**: Fixed fractional Ti conversion errors
- ✅ **RBAC Permissions**: Added missing controller permissions for VDC creation
- ✅ **Race Conditions**: Implemented retry logic for status updates
- ✅ **Platform Support**: Added OpenShift vs Kubernetes detection
- ✅ **Storage Classes**: Dynamic detection and configuration
- ✅ **Build System**: Fixed Go compilation and interface issues

These fixes ensure stable deployment and operation across different Kubernetes environments.