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
