package api

import (
	"context"
	"net/http"
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

// OrganizationHandlers handles organization-related requests
type OrganizationHandlers struct {
	storage   storage.Storage
	k8sClient client.Client
}

// NewOrganizationHandlers creates a new organization handlers instance
func NewOrganizationHandlers(storage storage.Storage, k8sClient client.Client) *OrganizationHandlers {
	return &OrganizationHandlers{
		storage:   storage,
		k8sClient: k8sClient,
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
	userID, username, role, _, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin can create organizations
	if role != models.RoleSystemAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only system administrators can create organizations"})
		return
	}

	// Use sanitized name as ID
	orgID := util.SanitizeKubernetesName(req.Name)

	// Create Organization CRD
	orgCR := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: orgID,
			Annotations: map[string]string{
				"ovim.io/created-by": username,
				"ovim.io/created-at": time.Now().Format(time.RFC3339),
			},
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: req.DisplayName,
			Description: req.Description,
			Admins:      req.Admins,
			IsEnabled:   req.IsEnabled,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if h.k8sClient != nil {
		if err := h.k8sClient.Create(ctx, orgCR); err != nil {
			klog.Errorf("Failed to create Organization CRD %s: %v", orgID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization CRD"})
			return
		}
	} else {
		klog.Warningf("k8sClient not available, skipping CRD creation for organization %s", orgID)
	}

	klog.Infof("Created Organization CRD %s by user %s (%s)", orgID, username, userID)

	// Return organization data from CRD (controller will handle database sync)
	response := &models.Organization{
		ID:          orgCR.Name,
		Name:        orgCR.Spec.DisplayName,
		Description: orgCR.Spec.Description,
		Namespace:   "", // Will be set by controller when namespace is created
		IsEnabled:   orgCR.Spec.IsEnabled,
		DisplayName: &orgCR.Spec.DisplayName,
		CRName:      orgCR.Name,
		CRNamespace: "default",
	}

	klog.Infof("Organization %s (%s) creation initiated by user %s (%s) - controller will handle resource creation",
		req.DisplayName, orgID, username, userID)

	c.JSON(http.StatusCreated, response)
}

// Update handles updating an organization
func (h *OrganizationHandlers) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	var req models.UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid update organization request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user info from context
	userID, username, role, _, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin can update organizations
	if role != models.RoleSystemAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only system administrators can update organizations"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get existing Organization CRD
	orgCR := &ovimv1.Organization{}
	if h.k8sClient != nil {
		if err := h.k8sClient.Get(ctx, client.ObjectKey{Name: id}, orgCR); err != nil {
			klog.Errorf("Failed to get Organization CRD %s: %v", id, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
	} else {
		klog.Warningf("k8sClient not available, skipping organization retrieval for %s", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Update fields
	if req.DisplayName != nil {
		orgCR.Spec.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		orgCR.Spec.Description = *req.Description
	}
	if req.Admins != nil {
		orgCR.Spec.Admins = req.Admins
	}
	if req.IsEnabled != nil {
		orgCR.Spec.IsEnabled = *req.IsEnabled
	}

	// Add update annotation
	if orgCR.Annotations == nil {
		orgCR.Annotations = make(map[string]string)
	}
	orgCR.Annotations["ovim.io/updated-by"] = username
	orgCR.Annotations["ovim.io/updated-at"] = time.Now().Format(time.RFC3339)

	if h.k8sClient != nil {
		if err := h.k8sClient.Update(ctx, orgCR); err != nil {
			klog.Errorf("Failed to update Organization CRD %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update organization CRD"})
			return
		}
	} else {
		klog.Warningf("k8sClient not available, skipping organization update for %s", id)
	}

	klog.Infof("Updated Organization CRD %s by user %s (%s)", id, username, userID)

	// Return updated organization data from CRD
	response := &models.Organization{
		ID:          orgCR.Name,
		Name:        orgCR.Spec.DisplayName,
		Description: orgCR.Spec.Description,
		Namespace:   orgCR.Status.Namespace,
		IsEnabled:   orgCR.Spec.IsEnabled,
		DisplayName: &orgCR.Spec.DisplayName,
		CRName:      orgCR.Name,
		CRNamespace: "default",
	}

	c.JSON(http.StatusOK, response)
}

// Delete handles deleting an organization
func (h *OrganizationHandlers) Delete(c *gin.Context) {
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

	// Check permissions - only system admin can delete organizations
	if role != models.RoleSystemAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only system administrators can delete organizations"})
		return
	}

	// Check for dependent VDCs
	vdcs, err := h.storage.ListVDCs(id)
	if err != nil {
		klog.Errorf("Failed to list VDCs for organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check VDCs"})
		return
	}

	if len(vdcs) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":     "Cannot delete organization with existing VDCs",
			"vdc_count": len(vdcs),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get existing Organization CRD
	orgCR := &ovimv1.Organization{}
	if h.k8sClient != nil {
		if err := h.k8sClient.Get(ctx, client.ObjectKey{Name: id}, orgCR); err != nil {
			klog.Errorf("Failed to get Organization CRD %s: %v", id, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
	} else {
		klog.Warningf("k8sClient not available, skipping organization deletion for %s", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	// Add deletion annotation for audit
	if orgCR.Annotations == nil {
		orgCR.Annotations = make(map[string]string)
	}
	orgCR.Annotations["ovim.io/deleted-by"] = username
	orgCR.Annotations["ovim.io/deleted-at"] = time.Now().Format(time.RFC3339)

	if h.k8sClient != nil {
		if err := h.k8sClient.Update(ctx, orgCR); err != nil {
			klog.Warningf("Failed to add deletion annotation to Organization CRD %s: %v", id, err)
		}
	} else {
		klog.Warningf("k8sClient not available, skipping deletion annotation for %s", id)
	}

	// Delete the Organization CRD
	if h.k8sClient != nil {
		if err := h.k8sClient.Delete(ctx, orgCR); err != nil {
			klog.Errorf("Failed to delete Organization CRD %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete organization CRD"})
			return
		}
	} else {
		klog.Warningf("k8sClient not available, skipping organization CRD deletion for %s", id)
	}

	klog.Infof("Deleted Organization CRD %s by user %s (%s) - controller will handle cleanup", id, username, userID)

	c.JSON(http.StatusNoContent, nil)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is not assigned to any organization"})
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

	// Get VMs for this organization
	vms, err := h.storage.ListVMs(id)
	if err != nil {
		klog.Errorf("Failed to list VMs for organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VMs"})
		return
	}

	// Calculate actual resource usage
	usage := org.GetResourceUsage(vdcs, vms)

	klog.V(6).Infof("Retrieved resource usage for organization %s (CPU: %d/%d, Memory: %d/%d, Storage: %d/%d)",
		org.Name, usage.CPUUsed, usage.CPUQuota, usage.MemoryUsed, usage.MemoryQuota, usage.StorageUsed, usage.StorageQuota)

	c.JSON(http.StatusOK, usage)
}

// UpdateResourceQuotas is deprecated - organizations are identity containers only
// Resource quotas are managed at the VDC level
func (h *OrganizationHandlers) UpdateResourceQuotas(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error": "Organizations are identity containers only. Resource quotas are managed at the Virtual Data Center (VDC) level.",
	})
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

	// Get VMs for this organization
	vms, err := h.storage.ListVMs(id)
	if err != nil {
		klog.Errorf("Failed to list VMs for organization %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VMs"})
		return
	}

	// Calculate actual resource usage
	usage := org.GetResourceUsage(vdcs, vms)

	// Check if allocation is possible
	canAllocate := org.CanAllocateResources(req.CPURequest, req.MemoryRequest, req.StorageRequest, vdcs)

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
