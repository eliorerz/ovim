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

// OrganizationHandlers handles organization-related requests
type OrganizationHandlers struct {
	storage storage.Storage
}

// NewOrganizationHandlers creates a new organization handlers instance
func NewOrganizationHandlers(storage storage.Storage) *OrganizationHandlers {
	return &OrganizationHandlers{
		storage: storage,
	}
}

// List handles listing all organizations
func (h *OrganizationHandlers) List(c *gin.Context) {
	orgs, err := h.storage.ListOrganizations()
	if err != nil {
		klog.Errorf("Failed to list organizations: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list organizations"})
		return
	}

	klog.V(6).Infof("Listed %d organizations", len(orgs))
	c.JSON(http.StatusOK, gin.H{
		"organizations": orgs,
		"total":         len(orgs),
	})
}

// Get handles getting a specific organization
func (h *OrganizationHandlers) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	org, err := h.storage.GetOrganization(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	c.JSON(http.StatusOK, org)
}

// Create handles creating a new organization
func (h *OrganizationHandlers) Create(c *gin.Context) {
	var req models.CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid create organization request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user info from context
	userID, username, _, _, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Use sanitized name as both ID and namespace
	orgID := util.SanitizeKubernetesName(req.Name)
	namespace := orgID

	// Create organization
	org := &models.Organization{
		ID:          orgID,
		Name:        req.Name,
		Description: req.Description,
		Namespace:   namespace,
		IsEnabled:   req.IsEnabled,
	}

	if err := h.storage.CreateOrganization(org); err != nil {
		if err == storage.ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Organization already exists"})
			return
		}
		klog.Errorf("Failed to create organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization"})
		return
	}

	klog.Infof("Organization %s (%s) created by user %s (%s)", org.Name, org.ID, username, userID)

	c.JSON(http.StatusCreated, org)
}

// Update handles updating an organization
func (h *OrganizationHandlers) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get existing organization
	org, err := h.storage.GetOrganization(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	var req models.UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid update organization request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user info from context
	userID, username, _, _, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Update organization
	org.Name = req.Name
	org.Description = req.Description
	org.IsEnabled = req.IsEnabled
	// Note: We don't update the namespace as it could break existing resources

	if err := h.storage.UpdateOrganization(org); err != nil {
		klog.Errorf("Failed to update organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update organization"})
		return
	}

	klog.Infof("Organization %s (%s) updated by user %s (%s)", org.Name, org.ID, username, userID)

	c.JSON(http.StatusOK, org)
}

// Delete handles deleting an organization
func (h *OrganizationHandlers) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Check if organization exists
	org, err := h.storage.GetOrganization(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	// Get user info from context
	userID, username, _, _, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// TODO: Check if organization has any VMs or other resources before deletion
	// For now, we'll allow deletion

	if err := h.storage.DeleteOrganization(id); err != nil {
		klog.Errorf("Failed to delete organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete organization"})
		return
	}

	klog.Infof("Organization %s (%s) deleted by user %s (%s)", org.Name, org.ID, username, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Organization deleted successfully",
	})
}

// GetUserOrganization handles getting the current user's organization
func (h *OrganizationHandlers) GetUserOrganization(c *gin.Context) {
	// Get user info from context
	userID, username, _, orgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check if user has an organization
	if orgID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "User is not assigned to any organization"})
		return
	}

	// Get the organization
	org, err := h.storage.GetOrganization(orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User's organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s for user %s (%s): %v", orgID, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	klog.V(6).Infof("Retrieved organization %s for user %s", org.Name, username)
	c.JSON(http.StatusOK, org)
}

// GetResourceUsage handles getting organization resource usage
func (h *OrganizationHandlers) GetResourceUsage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin can view any org, others can only view their own
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != id {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only view resource usage for your own organization"})
			return
		}
	}

	// Get organization
	org, err := h.storage.GetOrganization(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	// Get VDCs for this organization
	vdcs, err := h.storage.ListVDCs(id)
	if err != nil {
		klog.Errorf("Failed to list VDCs for organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDCs"})
		return
	}

	// Calculate resource usage
	usage := org.GetResourceUsage(vdcs)

	klog.V(6).Infof("Retrieved resource usage for organization %s (CPU: %d/%d, Memory: %d/%d, Storage: %d/%d)",
		org.Name, usage.CPUUsed, usage.CPUQuota, usage.MemoryUsed, usage.MemoryQuota, usage.StorageUsed, usage.StorageQuota)

	c.JSON(http.StatusOK, usage)
}

// UpdateResourceQuotas handles updating organization resource quotas (system admin only)
func (h *OrganizationHandlers) UpdateResourceQuotas(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get user info from context
	userID, username, role, _, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Only system admin can update resource quotas
	if role != models.RoleSystemAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only system administrators can update resource quotas"})
		return
	}

	// Get existing organization
	org, err := h.storage.GetOrganization(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	// Define request structure for resource quotas
	type UpdateResourceQuotasRequest struct {
		CPUQuota     int `json:"cpu_quota"`
		MemoryQuota  int `json:"memory_quota"`
		StorageQuota int `json:"storage_quota"`
	}

	var req UpdateResourceQuotasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid update resource quotas request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate quotas are not negative
	if req.CPUQuota < 0 || req.MemoryQuota < 0 || req.StorageQuota < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Resource quotas cannot be negative"})
		return
	}

	// Update organization quotas
	org.CPUQuota = req.CPUQuota
	org.MemoryQuota = req.MemoryQuota
	org.StorageQuota = req.StorageQuota

	if err := h.storage.UpdateOrganization(org); err != nil {
		klog.Errorf("Failed to update organization resource quotas for %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update resource quotas"})
		return
	}

	klog.Infof("Organization %s (%s) resource quotas updated by user %s (%s): CPU=%d, Memory=%d, Storage=%d",
		org.Name, org.ID, username, userID, req.CPUQuota, req.MemoryQuota, req.StorageQuota)

	c.JSON(http.StatusOK, org)
}

// ValidateResourceAllocation handles validating if requested resources can be allocated
func (h *OrganizationHandlers) ValidateResourceAllocation(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin can validate any org, others can only validate their own
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != id {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only validate resource allocation for your own organization"})
			return
		}
	}

	// Define request structure for resource validation
	type ValidateResourceRequest struct {
		CPURequest     int `json:"cpu_request"`
		MemoryRequest  int `json:"memory_request"`
		StorageRequest int `json:"storage_request"`
	}

	var req ValidateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid validate resource request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get organization
	org, err := h.storage.GetOrganization(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	// Get VDCs for this organization
	vdcs, err := h.storage.ListVDCs(id)
	if err != nil {
		klog.Errorf("Failed to list VDCs for organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDCs"})
		return
	}

	// Check if allocation is possible
	canAllocate := org.CanAllocateResources(req.CPURequest, req.MemoryRequest, req.StorageRequest, vdcs)

	// Get current usage for detailed response
	usage := org.GetResourceUsage(vdcs)

	response := gin.H{
		"can_allocate": canAllocate,
		"requested": gin.H{
			"cpu":     req.CPURequest,
			"memory":  req.MemoryRequest,
			"storage": req.StorageRequest,
		},
		"current_usage": usage,
	}

	if !canAllocate {
		response["reason"] = "Insufficient resources available"
	}

	klog.V(6).Infof("Resource allocation validation for organization %s: requested CPU=%d, Memory=%d, Storage=%d, can_allocate=%v",
		org.Name, req.CPURequest, req.MemoryRequest, req.StorageRequest, canAllocate)

	c.JSON(http.StatusOK, response)
}
