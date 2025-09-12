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

// OpenShiftClient interface defines the methods needed for namespace operations
type OpenShiftClient interface {
	CreateNamespace(ctx context.Context, name string, labels map[string]string, annotations map[string]string) error
	CreateResourceQuota(ctx context.Context, namespace string, cpuQuota, memoryQuota, storageQuota int) error
	CreateLimitRange(ctx context.Context, namespace string, minCPU, maxCPU, minMemory, maxMemory int) error
	DeleteNamespace(ctx context.Context, name string) error
	NamespaceExists(ctx context.Context, name string) (bool, error)
}

// OrganizationHandlers handles organization-related requests
type OrganizationHandlers struct {
	storage         storage.Storage
	openshiftClient OpenShiftClient
}

// NewOrganizationHandlers creates a new organization handlers instance
func NewOrganizationHandlers(storage storage.Storage, openshiftClient OpenShiftClient) *OrganizationHandlers {
	return &OrganizationHandlers{
		storage:         storage,
		openshiftClient: openshiftClient,
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

	// Resource quotas are now required - no defaults
	if req.CPUQuota == nil || *req.CPUQuota <= 0 {
		klog.V(4).Infof("Invalid CPU quota in create organization request: %v", req.CPUQuota)
		c.JSON(http.StatusBadRequest, gin.H{"error": "CPU quota is required and must be greater than 0"})
		return
	}
	if req.MemoryQuota == nil || *req.MemoryQuota <= 0 {
		klog.V(4).Infof("Invalid memory quota in create organization request: %v", req.MemoryQuota)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Memory quota is required and must be greater than 0"})
		return
	}
	if req.StorageQuota == nil || *req.StorageQuota <= 0 {
		klog.V(4).Infof("Invalid storage quota in create organization request: %v", req.StorageQuota)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Storage quota is required and must be greater than 0"})
		return
	}

	cpuQuota := *req.CPUQuota
	memoryQuota := *req.MemoryQuota
	storageQuota := *req.StorageQuota

	// Create organization
	org := &models.Organization{
		ID:           orgID,
		Name:         req.Name,
		Description:  req.Description,
		Namespace:    namespace,
		IsEnabled:    req.IsEnabled,
		CPUQuota:     cpuQuota,
		MemoryQuota:  memoryQuota,
		StorageQuota: storageQuota,
	}

	// Create organization in database first
	if err := h.storage.CreateOrganization(org); err != nil {
		if err == storage.ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Organization already exists"})
			return
		}
		klog.Errorf("Failed to create organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization"})
		return
	}


	// Create namespace in OpenShift cluster if client is available
	if h.openshiftClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Check if namespace already exists
		exists, err := h.openshiftClient.NamespaceExists(ctx, namespace)
		if err != nil {
			klog.Errorf("Failed to check if namespace %s exists: %v", namespace, err)
			// Don't fail the organization creation if we can't check namespace
		} else if !exists {
			// Create namespace with appropriate labels and annotations
			labels := map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "organization",
				"app.kubernetes.io/managed-by": "ovim",
				"ovim.io/organization-id":      orgID,
				"ovim.io/organization-name":    util.SanitizeKubernetesName(req.Name),
			}

			annotations := map[string]string{
				"ovim.io/organization-description": req.Description,
				"ovim.io/created-by":               username,
				"ovim.io/created-at":               time.Now().Format(time.RFC3339),
			}

			// Create the namespace
			if err := h.openshiftClient.CreateNamespace(ctx, namespace, labels, annotations); err != nil {
				klog.Errorf("Failed to create namespace %s for organization %s: %v", namespace, orgID, err)
				// Try to rollback - delete the organization from database
				if rollbackErr := h.storage.DeleteOrganization(orgID); rollbackErr != nil {
					klog.Errorf("Failed to rollback organization creation after namespace failure: %v", rollbackErr)
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization namespace"})
				return
			}

			// Create resource quota for the namespace
			if err := h.openshiftClient.CreateResourceQuota(ctx, namespace, org.CPUQuota, org.MemoryQuota, org.StorageQuota); err != nil {
				klog.Errorf("Failed to create resource quota for namespace %s: %v", namespace, err)
				// Log error but don't fail the organization creation - quota can be created later
			} else {
				klog.Infof("Created resource quota for organization %s namespace %s (CPU: %d, Memory: %dGi, Storage: %dGi)",
					orgID, namespace, org.CPUQuota, org.MemoryQuota, org.StorageQuota)
			}

			// Create LimitRange for per-VM resource constraints if provided
			if req.LimitRange != nil {
				lr := req.LimitRange
				if err := h.openshiftClient.CreateLimitRange(ctx, namespace, lr.MinCPU, lr.MaxCPU, lr.MinMemory, lr.MaxMemory); err != nil {
					klog.Errorf("Failed to create LimitRange for namespace %s: %v", namespace, err)
					// Log error but don't fail the organization creation - LimitRange can be created later
				} else {
					klog.Infof("Created LimitRange for organization %s namespace %s (VM limits: %d-%d vCPUs, %d-%dGi RAM)",
						orgID, namespace, lr.MinCPU, lr.MaxCPU, lr.MinMemory, lr.MaxMemory)
				}
			} else {
				klog.Infof("No LimitRange requested for organization %s namespace %s", orgID, namespace)
			}

			klog.Infof("Created namespace %s for organization %s", namespace, orgID)
		} else {
			klog.Infof("Namespace %s already exists for organization %s", namespace, orgID)
		}
	} else {
		klog.Warningf("OpenShift client not available - namespace %s not created for organization %s", namespace, orgID)
	}

	klog.Infof("Organization %s (%s) created by user %s (%s) with resource quotas (CPU: %d, Memory: %dGB, Storage: %dGB)",
		org.Name, org.ID, username, userID, org.CPUQuota, org.MemoryQuota, org.StorageQuota)

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

	// Delete all attached resources before deleting the organization
	if err := h.deleteOrganizationResources(id, username, userID); err != nil {
		klog.Errorf("Failed to delete organization resources for %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete organization resources"})
		return
	}

	// Delete from database last
	if err := h.storage.DeleteOrganization(id); err != nil {
		klog.Errorf("Failed to delete organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete organization"})
		return
	}

	// Delete namespace from OpenShift cluster if client is available
	if h.openshiftClient != nil && org.Namespace != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // More time for namespace deletion
		defer cancel()

		// Check if namespace exists before trying to delete
		exists, err := h.openshiftClient.NamespaceExists(ctx, org.Namespace)
		if err != nil {
			klog.Errorf("Failed to check if namespace %s exists for organization %s: %v", org.Namespace, id, err)
		} else if exists {
			// Delete the namespace (this will delete all resources within it)
			if err := h.openshiftClient.DeleteNamespace(ctx, org.Namespace); err != nil {
				klog.Errorf("Failed to delete namespace %s for organization %s: %v", org.Namespace, id, err)
				// Don't fail the API call - organization is already deleted from database
			} else {
				klog.Infof("Deleted namespace %s for organization %s", org.Namespace, id)
			}
		} else {
			klog.Infof("Namespace %s for organization %s does not exist (already deleted)", org.Namespace, id)
		}
	} else {
		if h.openshiftClient == nil {
			klog.Warningf("OpenShift client not available - namespace %s not deleted for organization %s", org.Namespace, id)
		}
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

// deleteOrganizationResources handles the cascade deletion of all resources belonging to an organization
func (h *OrganizationHandlers) deleteOrganizationResources(orgID, username, userID string) error {
	klog.V(4).Infof("Starting cascade deletion of resources for organization %s by user %s (%s)", orgID, username, userID)

	// 1. Delete all VMs in the organization
	vms, err := h.storage.ListVMs(orgID)
	if err != nil {
		return fmt.Errorf("failed to list VMs for organization %s: %v", orgID, err)
	}

	for _, vm := range vms {
		klog.V(6).Infof("Deleting VM %s (%s) in organization %s", vm.Name, vm.ID, orgID)
		if err := h.storage.DeleteVM(vm.ID); err != nil {
			klog.Errorf("Failed to delete VM %s in organization %s: %v", vm.ID, orgID, err)
			// Continue with other VMs even if one fails
		} else {
			klog.Infof("Deleted VM %s (%s) from organization %s", vm.Name, vm.ID, orgID)
		}
	}

	// 2. Delete all VDCs in the organization
	vdcs, err := h.storage.ListVDCs(orgID)
	if err != nil {
		return fmt.Errorf("failed to list VDCs for organization %s: %v", orgID, err)
	}

	for _, vdc := range vdcs {
		klog.V(6).Infof("Deleting VDC %s (%s) in organization %s", vdc.Name, vdc.ID, orgID)
		if err := h.storage.DeleteVDC(vdc.ID); err != nil {
			klog.Errorf("Failed to delete VDC %s in organization %s: %v", vdc.ID, orgID, err)
			// Continue with other VDCs even if one fails
		} else {
			klog.Infof("Deleted VDC %s (%s) from organization %s", vdc.Name, vdc.ID, orgID)
		}
	}

	// 3. Delete organization-specific templates
	templates, err := h.storage.ListTemplatesByOrg(orgID)
	if err != nil {
		return fmt.Errorf("failed to list templates for organization %s: %v", orgID, err)
	}

	for _, template := range templates {
		klog.V(6).Infof("Deleting template %s (%s) in organization %s", template.Name, template.ID, orgID)
		if err := h.storage.DeleteTemplate(template.ID); err != nil {
			klog.Errorf("Failed to delete template %s in organization %s: %v", template.ID, orgID, err)
			// Continue with other templates even if one fails
		} else {
			klog.Infof("Deleted template %s (%s) from organization %s", template.Name, template.ID, orgID)
		}
	}

	// 4. Update users to remove their organization assignment
	// Note: We don't delete users, just remove their organization assignment
	users, err := h.storage.ListUsersByOrg(orgID)
	if err != nil {
		return fmt.Errorf("failed to list users for organization %s: %v", orgID, err)
	}

	for _, user := range users {
		klog.V(6).Infof("Removing organization assignment for user %s (%s)", user.Username, user.ID)
		user.OrgID = nil // Remove organization assignment
		if err := h.storage.UpdateUser(user); err != nil {
			klog.Errorf("Failed to remove organization assignment for user %s: %v", user.ID, err)
			// Continue with other users even if one fails
		} else {
			klog.Infof("Removed organization assignment for user %s (%s)", user.Username, user.ID)
		}
	}

	klog.Infof("Completed cascade deletion of resources for organization %s: %d VMs, %d VDCs, %d templates, %d users updated",
		orgID, len(vms), len(vdcs), len(templates), len(users))

	return nil
}
