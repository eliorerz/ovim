package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/eliorerz/ovim-updated/pkg/util"
)

// VDCHandlers handles VDC-related requests
type VDCHandlers struct {
	storage         storage.Storage
	k8sClient       client.Client
	openshiftClient OpenShiftClient // Legacy support, will be phased out
}

// NewVDCHandlers creates a new VDC handlers instance
func NewVDCHandlers(storage storage.Storage, k8sClient client.Client, openshiftClient OpenShiftClient) *VDCHandlers {
	return &VDCHandlers{
		storage:         storage,
		k8sClient:       k8sClient,
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

	// Generate VDC ID (use sanitized name for CRD)
	vdcID := util.SanitizeKubernetesName(req.Name)

	// Create VirtualDataCenter CRD if k8sClient is available
	if h.k8sClient != nil {
		vdcCR := &ovimv1.VirtualDataCenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vdcID,
				Namespace: fmt.Sprintf("org-%s", req.OrgID), // VDC CRs live in org namespace
				Annotations: map[string]string{
					"ovim.io/created-by": username,
					"ovim.io/created-at": time.Now().Format(time.RFC3339),
				},
			},
			Spec: ovimv1.VirtualDataCenterSpec{
				OrganizationRef: req.OrgID,
				DisplayName:     req.DisplayName,
				Description:     req.Description,
				Quota: ovimv1.ResourceQuota{
					CPU:     fmt.Sprintf("%d", req.CPUQuota),
					Memory:  fmt.Sprintf("%dGi", req.MemoryQuota),
					Storage: fmt.Sprintf("%dGi", req.StorageQuota),
				},
				NetworkPolicy: req.NetworkPolicy,
			},
		}

		// Add LimitRange if provided
		if req.MinCPU != nil || req.MaxCPU != nil || req.MinMemory != nil || req.MaxMemory != nil {
			vdcCR.Spec.LimitRange = &ovimv1.LimitRange{
				MinCpu:    *req.MinCPU,
				MaxCpu:    *req.MaxCPU,
				MinMemory: *req.MinMemory,
				MaxMemory: *req.MaxMemory,
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := h.k8sClient.Create(ctx, vdcCR); err != nil {
			klog.Errorf("Failed to create VirtualDataCenter CRD %s: %v", vdcID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create VDC CRD"})
			return
		}

		klog.Infof("Created VirtualDataCenter CRD %s in org %s by user %s (%s)", vdcID, req.OrgID, username, userID)
	} else {
		klog.Warningf("Kubernetes client not available - VDC CRD not created for %s", vdcID)
		// Fallback to legacy direct creation for backward compatibility
		vdc := &models.VirtualDataCenter{
			ID:                vdcID,
			Name:              req.Name,
			Description:       req.Description,
			OrgID:             req.OrgID,
			DisplayName:       &req.DisplayName,
			CRName:            vdcID,
			CRNamespace:       fmt.Sprintf("org-%s", req.OrgID),
			WorkloadNamespace: fmt.Sprintf("vdc-%s-%s", req.OrgID, vdcID),
			CPUQuota:          req.CPUQuota,
			MemoryQuota:       req.MemoryQuota,
			StorageQuota:      req.StorageQuota,
			NetworkPolicy:     req.NetworkPolicy,
			Phase:             "Active",
		}

		// Add LimitRange if provided
		if req.MinCPU != nil {
			vdc.MinCPU = req.MinCPU
		}
		if req.MaxCPU != nil {
			vdc.MaxCPU = req.MaxCPU
		}
		if req.MinMemory != nil {
			vdc.MinMemory = req.MinMemory
		}
		if req.MaxMemory != nil {
			vdc.MaxMemory = req.MaxMemory
		}

		// Create VDC in storage
		if h.storage != nil {
			if err := h.storage.CreateVDC(vdc); err != nil {
				klog.Errorf("Failed to create VDC %s in storage: %v", vdcID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create VDC"})
				return
			}
		}

		// Create VDC namespace and resources if OpenShift client is available
		if h.openshiftClient != nil && !reflect.ValueOf(h.openshiftClient).IsNil() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Create VDC namespace
			vdcNamespace := vdc.WorkloadNamespace
			exists, err := h.openshiftClient.NamespaceExists(ctx, vdcNamespace)
			if err != nil {
				klog.Errorf("Failed to check if VDC namespace %s exists: %v", vdcNamespace, err)
			} else if !exists {
				labels := map[string]string{
					"app.kubernetes.io/name":       "ovim",
					"app.kubernetes.io/component":  "vdc",
					"app.kubernetes.io/managed-by": "ovim",
					"ovim.io/organization":         req.OrgID,
					"ovim.io/vdc":                  vdcID,
					"type":                         "vdc",
					"org":                          req.OrgID,
					"vdc":                          vdcID,
				}
				annotations := map[string]string{
					"ovim.io/vdc-description": req.Description,
					"ovim.io/created-by":      username,
					"ovim.io/created-at":      time.Now().Format(time.RFC3339),
				}

				if err := h.openshiftClient.CreateNamespace(ctx, vdcNamespace, labels, annotations); err != nil {
					klog.Errorf("Failed to create VDC namespace %s: %v", vdcNamespace, err)
				} else {
					klog.Infof("Created VDC namespace %s by user %s (%s)", vdcNamespace, username, userID)

					// Create ResourceQuota
					if err := h.openshiftClient.CreateResourceQuota(ctx, vdcNamespace, req.CPUQuota, req.MemoryQuota, req.StorageQuota); err != nil {
						klog.Errorf("Failed to create ResourceQuota for VDC %s: %v", vdcID, err)
					}

					// Create LimitRange if specified and valid
					if req.MinCPU != nil && req.MaxCPU != nil && req.MinMemory != nil && req.MaxMemory != nil {
						// Validate LimitRange parameters
						if *req.MinCPU <= *req.MaxCPU && *req.MinMemory <= *req.MaxMemory {
							if err := h.openshiftClient.CreateLimitRange(ctx, vdcNamespace, *req.MinCPU, *req.MaxCPU, *req.MinMemory, *req.MaxMemory); err != nil {
								klog.Errorf("Failed to create LimitRange for VDC %s: %v", vdcID, err)
							}
						} else {
							klog.Warningf("Invalid LimitRange parameters for VDC %s: MinCPU=%d > MaxCPU=%d or MinMemory=%d > MaxMemory=%d", vdcID, *req.MinCPU, *req.MaxCPU, *req.MinMemory, *req.MaxMemory)
						}
					}
				}
			}
		}
	}

	// Return appropriate response based on architecture
	response := &models.VirtualDataCenter{
		ID:                vdcID,
		Name:              req.Name,
		Description:       req.Description,
		OrgID:             req.OrgID,
		DisplayName:       &req.DisplayName,
		CRName:            vdcID,
		CRNamespace:       fmt.Sprintf("org-%s", req.OrgID),
		WorkloadNamespace: fmt.Sprintf("vdc-%s-%s", req.OrgID, vdcID),
		CPUQuota:          req.CPUQuota,
		MemoryQuota:       req.MemoryQuota,
		StorageQuota:      req.StorageQuota,
		NetworkPolicy:     req.NetworkPolicy,
		Phase:             "Pending", // CRD mode: controller will handle creation
	}

	// For legacy mode, set phase to Active since resources are created immediately
	if h.k8sClient == nil {
		response.Phase = "Active"
	}

	klog.Infof("VDC %s (%s) creation initiated in org %s by user %s (%s) - controller will handle resource creation",
		req.DisplayName, vdcID, req.OrgID, username, userID)

	c.JSON(http.StatusCreated, response)
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

	var req models.UpdateVDCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid update VDC request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Update VirtualDataCenter CRD
	if h.k8sClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// First, find the VDC CRD - we need to check both the org namespace and discover the right one
		var vdcCR *ovimv1.VirtualDataCenter
		var orgNamespace string

		// Try to find the VDC by listing all VDCs and finding the one with matching name
		vdcList := &ovimv1.VirtualDataCenterList{}
		if err := h.k8sClient.List(ctx, vdcList); err != nil {
			klog.Errorf("Failed to list VDCs to find %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find VDC"})
			return
		}

		for _, vdc := range vdcList.Items {
			if vdc.Name == id {
				vdcCR = &vdc
				orgNamespace = vdc.Namespace
				break
			}
		}

		if vdcCR == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}

		// Check permissions - only system admin and org admin can update VDCs
		if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to update VDC"})
			return
		}

		// For org admin, ensure they can only update VDCs in their own organization
		if role == models.RoleOrgAdmin {
			expectedOrgNamespace := fmt.Sprintf("org-%s", userOrgID)
			if userOrgID == "" || orgNamespace != expectedOrgNamespace {
				c.JSON(http.StatusForbidden, gin.H{"error": "Can only update VDCs in your own organization"})
				return
			}
		}

		// Update fields
		if req.DisplayName != nil {
			vdcCR.Spec.DisplayName = *req.DisplayName
		}
		if req.Description != nil {
			vdcCR.Spec.Description = *req.Description
		}
		if req.CPUQuota != nil {
			vdcCR.Spec.Quota.CPU = fmt.Sprintf("%d", *req.CPUQuota)
		}
		if req.MemoryQuota != nil {
			vdcCR.Spec.Quota.Memory = fmt.Sprintf("%dGi", *req.MemoryQuota)
		}
		if req.StorageQuota != nil {
			vdcCR.Spec.Quota.Storage = fmt.Sprintf("%dGi", *req.StorageQuota)
		}
		if req.NetworkPolicy != nil {
			vdcCR.Spec.NetworkPolicy = *req.NetworkPolicy
		}

		// Add update annotation
		if vdcCR.Annotations == nil {
			vdcCR.Annotations = make(map[string]string)
		}
		vdcCR.Annotations["ovim.io/updated-by"] = username
		vdcCR.Annotations["ovim.io/updated-at"] = time.Now().Format(time.RFC3339)

		if err := h.k8sClient.Update(ctx, vdcCR); err != nil {
			klog.Errorf("Failed to update VirtualDataCenter CRD %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update VDC CRD"})
			return
		}

		klog.Infof("Updated VirtualDataCenter CRD %s by user %s (%s)", id, username, userID)

		// Return updated VDC data from CRD
		response := &models.VirtualDataCenter{
			ID:                vdcCR.Name,
			Name:              vdcCR.Spec.DisplayName,
			Description:       vdcCR.Spec.Description,
			OrgID:             vdcCR.Spec.OrganizationRef,
			DisplayName:       &vdcCR.Spec.DisplayName,
			CRName:            vdcCR.Name,
			CRNamespace:       vdcCR.Namespace,
			WorkloadNamespace: vdcCR.Status.Namespace,
			NetworkPolicy:     vdcCR.Spec.NetworkPolicy,
			Phase:             string(vdcCR.Status.Phase),
		}

		c.JSON(http.StatusOK, response)
	} else {
		klog.Warningf("Kubernetes client not available - cannot update VDC CRD %s", id)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Kubernetes client not available"})
	}
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

	// Delete VirtualDataCenter CRD if k8sClient is available
	if h.k8sClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// First, find the VDC CRD
		var vdcCR *ovimv1.VirtualDataCenter
		var orgNamespace string

		// Try to find the VDC by listing all VDCs and finding the one with matching name
		vdcList := &ovimv1.VirtualDataCenterList{}
		if err := h.k8sClient.List(ctx, vdcList); err != nil {
			klog.Errorf("Failed to list VDCs to find %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find VDC"})
			return
		}

		for _, vdc := range vdcList.Items {
			if vdc.Name == id {
				vdcCR = &vdc
				orgNamespace = vdc.Namespace
				break
			}
		}

		if vdcCR == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}

		// Check permissions - only system admin and org admin can delete VDCs
		if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to delete VDC"})
			return
		}

		// For org admin, ensure they can only delete VDCs in their own organization
		if role == models.RoleOrgAdmin {
			expectedOrgNamespace := fmt.Sprintf("org-%s", userOrgID)
			if userOrgID == "" || orgNamespace != expectedOrgNamespace {
				c.JSON(http.StatusForbidden, gin.H{"error": "Can only delete VDCs in your own organization"})
				return
			}
		}

		// Add deletion annotation for audit
		if vdcCR.Annotations == nil {
			vdcCR.Annotations = make(map[string]string)
		}
		vdcCR.Annotations["ovim.io/deleted-by"] = username
		vdcCR.Annotations["ovim.io/deleted-at"] = time.Now().Format(time.RFC3339)

		if err := h.k8sClient.Update(ctx, vdcCR); err != nil {
			klog.Warningf("Failed to add deletion annotation to VDC CRD %s: %v", id, err)
		}

		// Delete the VDC CRD
		if err := h.k8sClient.Delete(ctx, vdcCR); err != nil {
			klog.Errorf("Failed to delete VirtualDataCenter CRD %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete VDC CRD"})
			return
		}

		klog.Infof("Deleted VirtualDataCenter CRD %s by user %s (%s) - controller will handle cleanup", id, username, userID)

		c.JSON(http.StatusOK, gin.H{
			"message": "VDC deletion initiated - resources will be cleaned up by controller",
		})
	} else {
		klog.Warningf("Kubernetes client not available - using legacy VDC deletion for %s", id)

		// Fallback to legacy direct deletion for backward compatibility
		// Get VDC from storage first
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

		// Check for force parameter to handle cascading deletion
		forceDelete := c.Query("force") == "true"

		// Check if there are VMs in this VDC that would prevent deletion
		vms, err := h.storage.ListVMs(vdc.OrgID)
		if err != nil {
			klog.Errorf("Failed to list VMs for VDC %s deletion check: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check VDC dependencies"})
			return
		}

		// Filter VMs that belong to this VDC
		var vdcVMs []*models.VirtualMachine
		for _, vm := range vms {
			if vm.VDCID != nil && *vm.VDCID == id {
				vdcVMs = append(vdcVMs, vm)
			}
		}

		// If there are VMs and force is not set, block deletion
		if len(vdcVMs) > 0 && !forceDelete {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Cannot delete VDC with existing VMs",
				"details": gin.H{
					"vms_count":  len(vdcVMs),
					"suggestion": "Use force=true parameter to delete VDC and all VMs, or delete VMs manually first",
				},
			})
			return
		}

		// If force delete is requested, delete all VMs first
		var deletedVMs int
		if forceDelete && len(vdcVMs) > 0 {
			for _, vm := range vdcVMs {
				if err := h.storage.DeleteVM(vm.ID); err != nil {
					klog.Errorf("Failed to delete VM %s during VDC %s cascade deletion: %v", vm.ID, id, err)
					// Continue with other VMs even if one fails
				} else {
					deletedVMs++
					klog.Infof("Deleted VM %s (%s) during VDC %s cascade deletion", vm.Name, vm.ID, id)
				}
			}
		}

		// Delete VDC namespace and resources if OpenShift client is available
		if h.openshiftClient != nil && !reflect.ValueOf(h.openshiftClient).IsNil() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Delete VDC namespace (this will delete all resources in it)
			if err := h.openshiftClient.DeleteNamespace(ctx, vdc.WorkloadNamespace); err != nil {
				klog.Errorf("Failed to delete VDC namespace %s for VDC %s: %v", vdc.WorkloadNamespace, id, err)
				// Log error but continue with VDC deletion from database
			} else {
				klog.Infof("Deleted VDC namespace %s for VDC %s", vdc.WorkloadNamespace, id)
			}
		} else {
			klog.Warningf("OpenShift client not available - VDC namespace %s not deleted for VDC %s", vdc.WorkloadNamespace, id)
		}

		// Delete VDC from storage
		if err := h.storage.DeleteVDC(id); err != nil {
			klog.Errorf("Failed to delete VDC %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete VDC"})
			return
		}

		klog.Infof("Deleted VDC %s (%s) by user %s (%s)", vdc.Name, id, username, userID)

		response := gin.H{
			"message": "VDC deleted successfully",
		}

		// Add cascade deletion information if it occurred
		if forceDelete && len(vdcVMs) > 0 {
			response["cascade_deletion"] = gin.H{
				"total_vms_deleted":   deletedVMs,
				"total_vms_found":     len(vdcVMs),
				"force_delete_used":   true,
				"deletion_successful": true,
			}
		}

		c.JSON(http.StatusOK, response)
	}
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
	if h.openshiftClient != nil && !reflect.ValueOf(h.openshiftClient).IsNil() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		limitRangeInfo, err := h.openshiftClient.GetLimitRange(ctx, vdc.WorkloadNamespace)
		if err != nil {
			klog.Errorf("Failed to get LimitRange for VDC %s namespace %s: %v", id, vdc.WorkloadNamespace, err)
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
	if h.openshiftClient != nil && !reflect.ValueOf(h.openshiftClient).IsNil() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Try to update first (in case it exists), then create if it doesn't exist
		err := h.openshiftClient.UpdateLimitRange(ctx, vdc.WorkloadNamespace, req.MinCPU, req.MaxCPU, req.MinMemory, req.MaxMemory)
		if err != nil {
			// If update fails, try to create
			klog.V(4).Infof("LimitRange update failed for VDC %s, trying to create: %v", id, err)
			err = h.openshiftClient.CreateLimitRange(ctx, vdc.WorkloadNamespace, req.MinCPU, req.MaxCPU, req.MinMemory, req.MaxMemory)
			if err != nil {
				klog.Errorf("Failed to create/update LimitRange for VDC %s namespace %s: %v", id, vdc.WorkloadNamespace, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create/update LimitRange"})
				return
			}
			klog.Infof("Created LimitRange for VDC %s namespace %s (VM limits: %d-%d vCPUs, %d-%dGi RAM) by user %s (%s)",
				id, vdc.WorkloadNamespace, req.MinCPU, req.MaxCPU, req.MinMemory, req.MaxMemory, username, userID)
		} else {
			klog.Infof("Updated LimitRange for VDC %s namespace %s (VM limits: %d-%d vCPUs, %d-%dGi RAM) by user %s (%s)",
				id, vdc.WorkloadNamespace, req.MinCPU, req.MaxCPU, req.MinMemory, req.MaxMemory, username, userID)
		}

		// Get the updated LimitRange info to return
		limitRangeInfo, err := h.openshiftClient.GetLimitRange(ctx, vdc.WorkloadNamespace)
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
	if h.openshiftClient != nil && !reflect.ValueOf(h.openshiftClient).IsNil() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := h.openshiftClient.DeleteLimitRange(ctx, vdc.WorkloadNamespace)
		if err != nil {
			klog.Errorf("Failed to delete LimitRange for VDC %s namespace %s: %v", id, vdc.WorkloadNamespace, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete LimitRange"})
			return
		}

		klog.Infof("Deleted LimitRange for VDC %s namespace %s by user %s (%s)", id, vdc.WorkloadNamespace, username, userID)
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
