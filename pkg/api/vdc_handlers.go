package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/eliorerz/ovim-updated/pkg/util"
)

// VDCHandlers handles VDC-related requests
type VDCHandlers struct {
	storage storage.Storage
}

// NewVDCHandlers creates a new VDC handlers instance
func NewVDCHandlers(storage storage.Storage) *VDCHandlers {
	return &VDCHandlers{
		storage: storage,
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

	// Check if organization can allocate these resources
	existingVDCs, err := h.storage.ListVDCs(req.OrgID)
	if err != nil {
		klog.Errorf("Failed to list existing VDCs for organization %s: %v", req.OrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate resource allocation"})
		return
	}

	if !org.CanAllocateResources(cpuReq, memoryReq, storageReq, existingVDCs) {
		usage := org.GetResourceUsage(existingVDCs)
		klog.Warningf("VDC creation denied for organization %s: insufficient resources. Requested: CPU=%d, Memory=%d, Storage=%d. Available: CPU=%d, Memory=%d, Storage=%d",
			org.Name, cpuReq, memoryReq, storageReq, usage.CPUAvailable, usage.MemoryAvailable, usage.StorageAvailable)

		c.JSON(http.StatusConflict, gin.H{
			"error": "Insufficient resources available in organization",
			"details": gin.H{
				"requested": gin.H{
					"cpu":     cpuReq,
					"memory":  memoryReq,
					"storage": storageReq,
				},
				"available": gin.H{
					"cpu":     usage.CPUAvailable,
					"memory":  usage.MemoryAvailable,
					"storage": usage.StorageAvailable,
				},
			},
		})
		return
	}

	// Create VDC
	vdc := &models.VirtualDataCenter{
		ID:             vdcID,
		Name:           req.Name,
		Description:    req.Description,
		OrgID:          req.OrgID,
		Namespace:      org.Namespace, // Use organization's namespace
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

	klog.Infof("VDC %s (%s) created in organization %s by user %s (%s)", vdc.Name, vdc.ID, org.Name, username, userID)

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

	// TODO: Check if VDC has any running VMs before deletion
	// For now, we'll allow deletion

	if err := h.storage.DeleteVDC(id); err != nil {
		klog.Errorf("Failed to delete VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete VDC"})
		return
	}

	klog.Infof("VDC %s (%s) deleted by user %s (%s)", vdc.Name, vdc.ID, username, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "VDC deleted successfully",
	})
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
