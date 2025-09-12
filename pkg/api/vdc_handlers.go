package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/eliorerz/ovim-updated/pkg/util"
)

// VDCHandlers handles VDC-related requests
type VDCHandlers struct {
	storage         storage.Storage
	openshiftClient OpenShiftClient
}

// NewVDCHandlers creates a new VDC handlers instance
func NewVDCHandlers(storage storage.Storage, openshiftClient OpenShiftClient) *VDCHandlers {
	return &VDCHandlers{
		storage:         storage,
		openshiftClient: openshiftClient,
	}
}

// List handles listing VDCs
func (h *VDCHandlers) List(c *gin.Context) {
	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	var orgFilter string
	// Filter VDCs based on user role
	if role == models.RoleSystemAdmin {
		// System admin can see all VDCs
		orgFilter = ""
	} else if role == models.RoleOrgAdmin || role == models.RoleOrgUser {
		// Org admin and users can only see VDCs from their organization
		if userOrgID == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "User not associated with any organization"})
			return
		}
		orgFilter = userOrgID
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	vdcs, err := h.storage.ListVDCs(orgFilter)
	if err != nil {
		klog.Errorf("Failed to list VDCs for user %s (%s): %v", username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list VDCs"})
		return
	}

	klog.V(6).Infof("Listed %d VDCs for user %s (%s)", len(vdcs), username, userID)
	c.JSON(http.StatusOK, gin.H{
		"vdcs":  vdcs,
		"total": len(vdcs),
	})
}

// Get handles getting a specific VDC
func (h *VDCHandlers) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Check access permissions
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VDC"})
			return
		}
	}

	c.JSON(http.StatusOK, vdc)
}

// Create handles creating a new VDC
func (h *VDCHandlers) Create(c *gin.Context) {
	var req models.CreateVDCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid create VDC request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin and org admin can create VDCs
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to create VDC"})
		return
	}

	// For org admin, ensure they can only create VDCs in their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != req.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only create VDCs in your own organization"})
			return
		}
	}

	// Verify the organization exists
	org, err := h.storage.GetOrganization(req.OrgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to verify organization %s: %v", req.OrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify organization"})
		return
	}

	// Generate VDC ID
	vdcID, err := util.GenerateID(16)
	if err != nil {
		klog.Errorf("Failed to generate VDC ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate VDC ID"})
		return
	}
	vdcID = "vdc-" + vdcID

	// Set default resource quotas if not provided
	resourceQuotas := req.ResourceQuotas
	if resourceQuotas == nil {
		resourceQuotas = map[string]string{
			"cpu":     "10",
			"memory":  "32Gi",
			"storage": "500Gi",
		}
	}

	// Parse requested resources for validation
	var cpuReq, memoryReq, storageReq int
	if cpuStr, ok := resourceQuotas["cpu"]; ok {
		cpuReq = models.ParseCPUString(cpuStr)
	}
	if memStr, ok := resourceQuotas["memory"]; ok {
		memoryReq = models.ParseMemoryString(memStr)
	}
	if storStr, ok := resourceQuotas["storage"]; ok {
		storageReq = models.ParseStorageString(storStr)
	}

	// Organizations are identity containers - no need to validate resource allocation

	// Organizations are identity containers only - resource allocation is always allowed
	// Resource constraints will be enforced at the cluster level via ResourceQuota and LimitRange
	klog.Infof("Creating VDC %s in organization %s (identity container) with requested resources: CPU=%d, Memory=%d, Storage=%d",
		vdcID, org.Name, cpuReq, memoryReq, storageReq)

	// Generate VDC namespace name: org-<orgname>-<vdcname>
	vdcNamespace := fmt.Sprintf("%s-%s", org.Namespace, util.SanitizeKubernetesName(req.Name))

	// Create VDC
	vdc := &models.VirtualDataCenter{
		ID:             vdcID,
		Name:           req.Name,
		Description:    req.Description,
		OrgID:          req.OrgID,
		Namespace:      vdcNamespace, // Use VDC-specific namespace
		ResourceQuotas: resourceQuotas,
	}

	if err := h.storage.CreateVDC(vdc); err != nil {
		if err == storage.ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "VDC already exists"})
			return
		}
		klog.Errorf("Failed to create VDC: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create VDC"})
		return
	}

	// Create VDC namespace in OpenShift cluster if client is available
	if h.openshiftClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Check if namespace already exists
		exists, err := h.openshiftClient.NamespaceExists(ctx, vdcNamespace)
		if err != nil {
			klog.Errorf("Failed to check if VDC namespace %s exists: %v", vdcNamespace, err)
			// Don't fail the VDC creation if we can't check namespace
		} else if !exists {
			// Create VDC namespace with appropriate labels and annotations
			labels := map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "vdc",
				"app.kubernetes.io/managed-by": "ovim",
				"ovim.io/organization-id":      org.ID,
				"ovim.io/organization-name":    util.SanitizeKubernetesName(org.Name),
				"ovim.io/vdc-id":               vdc.ID,
				"ovim.io/vdc-name":             util.SanitizeKubernetesName(vdc.Name),
			}

			annotations := map[string]string{
				"ovim.io/vdc-description":     vdc.Description,
				"ovim.io/created-by":          username,
				"ovim.io/created-at":          time.Now().Format(time.RFC3339),
				"ovim.io/parent-organization": org.Namespace,
			}

			// Create the VDC namespace
			if err := h.openshiftClient.CreateNamespace(ctx, vdcNamespace, labels, annotations); err != nil {
				klog.Errorf("Failed to create VDC namespace %s: %v", vdcNamespace, err)
				// Try to rollback - delete the VDC from database
				if rollbackErr := h.storage.DeleteVDC(vdc.ID); rollbackErr != nil {
					klog.Errorf("Failed to rollback VDC creation after namespace failure: %v", rollbackErr)
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create VDC namespace"})
				return
			}

			// Create ResourceQuota for the VDC namespace
			if err := h.openshiftClient.CreateResourceQuota(ctx, vdcNamespace, cpuReq, memoryReq, storageReq); err != nil {
				klog.Errorf("Failed to create ResourceQuota for VDC namespace %s: %v", vdcNamespace, err)
				// Log error but don't fail the VDC creation - quota can be created later
			} else {
				klog.Infof("Created ResourceQuota for VDC %s namespace %s (CPU: %d, Memory: %dGi, Storage: %dGi)",
					vdc.Name, vdcNamespace, cpuReq, memoryReq, storageReq)
			}

			// Create LimitRange for the VDC namespace if parameters are provided
			if req.MinCPU != nil && req.MaxCPU != nil && req.MinMemory != nil && req.MaxMemory != nil {
				// Validate LimitRange values
				if *req.MinCPU >= 0 && *req.MaxCPU >= 0 && *req.MinMemory >= 0 && *req.MaxMemory >= 0 &&
					*req.MinCPU <= *req.MaxCPU && *req.MinMemory <= *req.MaxMemory {

					if err := h.openshiftClient.CreateLimitRange(ctx, vdcNamespace, *req.MinCPU, *req.MaxCPU, *req.MinMemory, *req.MaxMemory); err != nil {
						klog.Errorf("Failed to create LimitRange for VDC namespace %s: %v", vdcNamespace, err)
						// Log error but don't fail the VDC creation - LimitRange can be created later
					} else {
						klog.Infof("Created LimitRange for VDC %s namespace %s (VM limits: %d-%d vCPUs, %d-%dGi RAM)",
							vdc.Name, vdcNamespace, *req.MinCPU, *req.MaxCPU, *req.MinMemory, *req.MaxMemory)
					}
				} else {
					klog.Warningf("Invalid LimitRange parameters for VDC %s - skipping LimitRange creation", vdc.Name)
				}
			}

			klog.Infof("Created VDC namespace %s for VDC %s in organization %s", vdcNamespace, vdc.Name, org.Name)
		} else {
			klog.Infof("VDC namespace %s already exists for VDC %s", vdcNamespace, vdc.Name)
		}
	} else {
		klog.Warningf("OpenShift client not available - VDC namespace %s not created for VDC %s", vdcNamespace, vdc.Name)
	}

	klog.Infof("VDC %s (%s) created in organization %s by user %s (%s) with namespace %s",
		vdc.Name, vdc.ID, org.Name, username, userID, vdcNamespace)

	c.JSON(http.StatusCreated, vdc)
}

// Update handles updating a VDC
func (h *VDCHandlers) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get existing VDC
	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Check permissions - only system admin and org admin can update VDCs
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to update VDC"})
		return
	}

	// For org admin, ensure they can only update VDCs in their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only update VDCs in your own organization"})
			return
		}
	}

	var req models.UpdateVDCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid update VDC request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Update VDC fields if provided
	if req.Name != "" {
		vdc.Name = req.Name
	}
	if req.Description != "" {
		vdc.Description = req.Description
	}
	if req.ResourceQuotas != nil {
		vdc.ResourceQuotas = req.ResourceQuotas
	}

	if err := h.storage.UpdateVDC(vdc); err != nil {
		klog.Errorf("Failed to update VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update VDC"})
		return
	}

	klog.Infof("VDC %s (%s) updated by user %s (%s)", vdc.Name, vdc.ID, username, userID)

	c.JSON(http.StatusOK, vdc)
}

// Delete handles deleting a VDC
func (h *VDCHandlers) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get existing VDC
	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Check permissions - only system admin and org admin can delete VDCs
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to delete VDC"})
		return
	}

	// For org admin, ensure they can only delete VDCs in their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only delete VDCs in your own organization"})
			return
		}
	}

	// Check for force parameter to enable cascade deletion
	forceDelete := c.Query("force") == "true"

	// Check if VDC has any VMs before deletion
	vms, err := h.storage.ListVMs(vdc.OrgID)
	if err != nil {
		klog.Errorf("Failed to list VMs for VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check VDC resources"})
		return
	}

	// Filter VMs that belong to this VDC
	var vdcVMs []*models.VirtualMachine
	for _, vm := range vms {
		if vm.VDCID == id {
			vdcVMs = append(vdcVMs, vm)
		}
	}

	// Track initial VM count for response
	initialVMCount := len(vdcVMs)

	// Handle VM deletion based on force parameter
	if len(vdcVMs) > 0 {
		if !forceDelete {
			// Protection mode: Block deletion and return detailed error
			runningVMs := 0
			var vmNames []string
			for _, vm := range vdcVMs {
				vmNames = append(vmNames, vm.Name)
				if vm.Status == models.VMStatusRunning || vm.Status == models.VMStatusProvisioning {
					runningVMs++
				}
			}

			klog.Warningf("VDC %s deletion blocked - contains %d VMs (%d running): %v",
				vdc.Name, len(vdcVMs), runningVMs, vmNames)

			c.JSON(http.StatusConflict, gin.H{
				"error": "Cannot delete VDC with existing VMs",
				"details": gin.H{
					"total_vms":   len(vdcVMs),
					"running_vms": runningVMs,
					"vm_names":    vmNames,
					"suggestion":  "Use force=true parameter to delete VDC and all VMs, or delete VMs individually first",
				},
			})
			return
		} else {
			// Force cascade deletion: Delete all VMs first
			klog.Infof("Force deleting VDC %s with %d VMs by user %s (%s)", vdc.Name, len(vdcVMs), username, userID)

			var deletionErrors []string
			deletedVMs := 0

			for _, vm := range vdcVMs {
				if err := h.storage.DeleteVM(vm.ID); err != nil {
					errMsg := fmt.Sprintf("Failed to delete VM %s (%s): %v", vm.Name, vm.ID, err)
					klog.Errorf("Failed to delete VM %s (%s): %v", vm.Name, vm.ID, err)
					deletionErrors = append(deletionErrors, errMsg)
				} else {
					deletedVMs++
					klog.Infof("Deleted VM %s (%s) as part of VDC %s cascade deletion", vm.Name, vm.ID, vdc.Name)
				}
			}

			// Log cascade deletion summary
			if len(deletionErrors) > 0 {
				klog.Warningf("VDC %s cascade deletion: deleted %d/%d VMs, %d errors occurred",
					vdc.Name, deletedVMs, len(vdcVMs), len(deletionErrors))
			} else {
				klog.Infof("VDC %s cascade deletion: successfully deleted all %d VMs", vdc.Name, deletedVMs)
			}
		}
	}

	// Delete VDC namespace and resources if OpenShift client is available
	if h.openshiftClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Delete namespace (this will delete all resources in it including ResourceQuota and LimitRange)
		if err := h.openshiftClient.DeleteNamespace(ctx, vdc.Namespace); err != nil {
			klog.Errorf("Failed to delete VDC namespace %s: %v", vdc.Namespace, err)
			// Log error but continue with VDC deletion from database
			// The namespace can be cleaned up manually if needed
		} else {
			klog.Infof("Deleted VDC namespace %s for VDC %s", vdc.Namespace, vdc.Name)
		}
	} else {
		klog.Warningf("OpenShift client not available - VDC namespace %s not deleted for VDC %s", vdc.Namespace, vdc.Name)
	}

	// Delete VDC from database
	if err := h.storage.DeleteVDC(id); err != nil {
		klog.Errorf("Failed to delete VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete VDC"})
		return
	}

	// Prepare response with cascade deletion details
	response := gin.H{
		"message": "VDC deleted successfully",
	}

	// Add cascade deletion information if force delete was used
	if forceDelete && initialVMCount > 0 {
		response["cascade_deletion"] = gin.H{
			"total_vms_deleted":   initialVMCount,
			"force_delete_used":   true,
			"deletion_successful": true,
		}
		klog.Infof("VDC %s (%s) and %d VMs cascade deleted by user %s (%s)", vdc.Name, vdc.ID, initialVMCount, username, userID)
	} else {
		klog.Infof("VDC %s (%s) deleted by user %s (%s)", vdc.Name, vdc.ID, username, userID)
	}

	c.JSON(http.StatusOK, response)
}

// ListUserVDCs handles listing VDCs for the current user's organization
func (h *VDCHandlers) ListUserVDCs(c *gin.Context) {
	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Only org users and org admins can use this endpoint
	if role != models.RoleOrgAdmin && role != models.RoleOrgUser {
		c.JSON(http.StatusForbidden, gin.H{"error": "This endpoint is for organization users only"})
		return
	}

	// Check if user has an organization
	if userOrgID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "User is not assigned to any organization"})
		return
	}

	// Get VDCs for the user's organization
	vdcs, err := h.storage.ListVDCs(userOrgID)
	if err != nil {
		klog.Errorf("Failed to list VDCs for user %s (%s) in org %s: %v", username, userID, userOrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list VDCs"})
		return
	}

	klog.V(6).Infof("Listed %d VDCs for user %s (%s) in org %s", len(vdcs), username, userID, userOrgID)
	c.JSON(http.StatusOK, gin.H{
		"vdcs":  vdcs,
		"total": len(vdcs),
	})
}

// GetLimitRange handles getting the current LimitRange for a VDC namespace
func (h *VDCHandlers) GetLimitRange(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get VDC to verify it exists and get namespace
	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Check access permissions
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VDC"})
			return
		}
	}

	// Get LimitRange from OpenShift cluster if client is available
	if h.openshiftClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		limitRangeInfo, err := h.openshiftClient.GetLimitRange(ctx, vdc.Namespace)
		if err != nil {
			klog.Errorf("Failed to get LimitRange for VDC %s namespace %s: %v", id, vdc.Namespace, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get LimitRange information"})
			return
		}

		klog.V(6).Infof("Retrieved LimitRange info for VDC %s: exists=%v", vdc.Name, limitRangeInfo.Exists)
		c.JSON(http.StatusOK, limitRangeInfo)
	} else {
		// No OpenShift client available
		klog.Warningf("OpenShift client not available - cannot get LimitRange for VDC %s", id)
		c.JSON(http.StatusOK, &models.LimitRangeInfo{
			Exists:    false,
			MinCPU:    0,
			MaxCPU:    0,
			MinMemory: 0,
			MaxMemory: 0,
		})
	}
}

// UpdateLimitRange handles creating or updating LimitRange for a VDC namespace
func (h *VDCHandlers) UpdateLimitRange(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - system admin and org admin can update LimitRange
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to update VDC LimitRange"})
		return
	}

	// Get existing VDC
	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// For org admin, ensure they can only update VDCs in their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only update LimitRange for VDCs in your own organization"})
			return
		}
	}

	var req models.LimitRangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid update LimitRange request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate LimitRange values are not negative
	if req.MinCPU < 0 || req.MaxCPU < 0 || req.MinMemory < 0 || req.MaxMemory < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LimitRange values cannot be negative"})
		return
	}

	// Validate that min values are not greater than max values
	if req.MinCPU > req.MaxCPU || req.MinMemory > req.MaxMemory {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimum values cannot be greater than maximum values"})
		return
	}

	// Update LimitRange in OpenShift cluster if client is available
	if h.openshiftClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Try to update first (in case it exists), then create if it doesn't exist
		err := h.openshiftClient.UpdateLimitRange(ctx, vdc.Namespace, req.MinCPU, req.MaxCPU, req.MinMemory, req.MaxMemory)
		if err != nil {
			// If update fails, try to create
			klog.V(4).Infof("LimitRange update failed for VDC %s, trying to create: %v", id, err)
			err = h.openshiftClient.CreateLimitRange(ctx, vdc.Namespace, req.MinCPU, req.MaxCPU, req.MinMemory, req.MaxMemory)
			if err != nil {
				klog.Errorf("Failed to create/update LimitRange for VDC %s namespace %s: %v", id, vdc.Namespace, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create/update LimitRange"})
				return
			}
			klog.Infof("Created LimitRange for VDC %s namespace %s (VM limits: %d-%d vCPUs, %d-%dGi RAM) by user %s (%s)",
				id, vdc.Namespace, req.MinCPU, req.MaxCPU, req.MinMemory, req.MaxMemory, username, userID)
		} else {
			klog.Infof("Updated LimitRange for VDC %s namespace %s (VM limits: %d-%d vCPUs, %d-%dGi RAM) by user %s (%s)",
				id, vdc.Namespace, req.MinCPU, req.MaxCPU, req.MinMemory, req.MaxMemory, username, userID)
		}

		// Get the updated LimitRange info to return
		limitRangeInfo, err := h.openshiftClient.GetLimitRange(ctx, vdc.Namespace)
		if err != nil {
			klog.Errorf("Failed to get updated LimitRange for VDC %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "LimitRange updated but failed to retrieve current state"})
			return
		}

		c.JSON(http.StatusOK, limitRangeInfo)
	} else {
		klog.Warningf("OpenShift client not available - cannot update LimitRange for VDC %s", id)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OpenShift client not available"})
	}
}

// DeleteLimitRange handles deleting LimitRange for a VDC namespace
func (h *VDCHandlers) DeleteLimitRange(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - system admin and org admin can delete LimitRange
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to delete VDC LimitRange"})
		return
	}

	// Get existing VDC
	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// For org admin, ensure they can only delete VDCs in their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only delete LimitRange for VDCs in your own organization"})
			return
		}
	}

	// Delete LimitRange from OpenShift cluster if client is available
	if h.openshiftClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := h.openshiftClient.DeleteLimitRange(ctx, vdc.Namespace)
		if err != nil {
			klog.Errorf("Failed to delete LimitRange for VDC %s namespace %s: %v", id, vdc.Namespace, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete LimitRange"})
			return
		}

		klog.Infof("Deleted LimitRange for VDC %s namespace %s by user %s (%s)", id, vdc.Namespace, username, userID)
		c.JSON(http.StatusOK, gin.H{"message": "LimitRange deleted successfully"})
	} else {
		klog.Warningf("OpenShift client not available - cannot delete LimitRange for VDC %s", id)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OpenShift client not available"})
	}
}

// GetResourceUsage handles getting VDC resource usage
func (h *VDCHandlers) GetResourceUsage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get VDC
	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Check permissions - only system admin can view any VDC, others can only view VDCs from their org
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only view resource usage for VDCs in your organization"})
			return
		}
	}

	// Get VMs for this VDC (we need all VMs in the organization to pass to the method)
	vms, err := h.storage.ListVMs(vdc.OrgID)
	if err != nil {
		klog.Errorf("Failed to list VMs for VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VMs"})
		return
	}

	// Calculate resource usage
	usage := vdc.GetResourceUsage(vms)

	klog.V(6).Infof("Retrieved resource usage for VDC %s (CPU: %d/%d, Memory: %d/%d, Storage: %d/%d, VMs: %d)",
		vdc.Name, usage.CPUUsed, usage.CPUQuota, usage.MemoryUsed, usage.MemoryQuota, usage.StorageUsed, usage.StorageQuota, usage.VMCount)

	c.JSON(http.StatusOK, usage)
}
