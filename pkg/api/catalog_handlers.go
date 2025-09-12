package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/catalog"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/eliorerz/ovim-updated/pkg/util"
)

// CatalogHandlers handles VM catalog-related requests
type CatalogHandlers struct {
	storage        storage.Storage
	catalogService *catalog.Service
}

// NewCatalogHandlers creates a new catalog handlers instance
func NewCatalogHandlers(storage storage.Storage, catalogService *catalog.Service) *CatalogHandlers {
	return &CatalogHandlers{
		storage:        storage,
		catalogService: catalogService,
	}
}

// ListTemplates handles listing all VM templates with multi-source support
func (h *CatalogHandlers) ListTemplates(c *gin.Context) {
	// Get query parameters for filtering
	source := c.Query("source")     // global, organization, external
	category := c.Query("category") // Operating System, Database, Application, etc.

	// Get user info from context
	_, _, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions based on role
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin && role != models.RoleOrgUser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	var templates []*models.Template
	var err error

	// Use catalog service if available, fallback to direct storage
	if h.catalogService != nil {
		templates, err = h.catalogService.GetTemplates(c.Request.Context(), userOrgID, source, category)
	} else {
		// Fallback to legacy direct storage access
		allTemplates, storageErr := h.storage.ListTemplates()
		if storageErr != nil {
			err = storageErr
		} else {
			templates = allTemplates
		}
	}

	if err != nil {
		klog.Errorf("Failed to list templates: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}

	klog.V(6).Infof("Listed %d templates (source: %s, category: %s)", len(templates), source, category)
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
		"filters": gin.H{
			"source":   source,
			"category": category,
		},
	})
}

// GetTemplate handles getting a specific VM template
func (h *CatalogHandlers) GetTemplate(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template ID required"})
		return
	}

	template, err := h.storage.GetTemplate(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		klog.Errorf("Failed to get template %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// ListTemplatesByOrg handles listing VM templates for a specific organization
func (h *CatalogHandlers) ListTemplatesByOrg(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get user info from context
	_, _, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin can access any org, others can only access their own
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only access templates for your own organization"})
			return
		}
	}

	templates, err := h.storage.ListTemplatesByOrg(orgID)
	if err != nil {
		klog.Errorf("Failed to list templates for organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}

	klog.V(6).Infof("Listed %d templates for organization %s", len(templates), orgID)
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
		"org_id":    orgID,
	})
}

// GetCatalogSources handles retrieving available catalog sources for the user
func (h *CatalogHandlers) GetCatalogSources(c *gin.Context) {
	// Get user info from context
	_, _, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions based on role
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin && role != models.RoleOrgUser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	if h.catalogService == nil {
		klog.Error("Catalog service not available")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Catalog service not available"})
		return
	}

	sources, err := h.catalogService.GetCatalogSources(c.Request.Context(), userOrgID)
	if err != nil {
		klog.Errorf("Failed to get catalog sources: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get catalog sources"})
		return
	}

	klog.V(6).Infof("Retrieved %d catalog sources for user org %s", len(sources), userOrgID)
	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
		"total":   len(sources),
	})
}

// GetOrganizationCatalogSources handles retrieving catalog sources for a specific organization
func (h *CatalogHandlers) GetOrganizationCatalogSources(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get user info from context
	_, _, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin can access any org, others can only access their own
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only access catalog sources for your own organization"})
			return
		}
	}

	sources, err := h.storage.ListOrganizationCatalogSources(orgID)
	if err != nil {
		klog.Errorf("Failed to list organization catalog sources for org %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list organization catalog sources"})
		return
	}

	klog.V(6).Infof("Retrieved %d catalog sources for organization %s", len(sources), orgID)
	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
		"total":   len(sources),
	})
}

// AddCatalogSourceToOrganization handles adding a catalog source to an organization
func (h *CatalogHandlers) AddCatalogSourceToOrganization(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get user info from context
	_, _, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin and org admin can manage catalog sources
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to manage catalog sources"})
		return
	}

	// For org admin, ensure they can only manage sources for their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only manage catalog sources for your own organization"})
			return
		}
	}

	var req models.CreateOrganizationCatalogSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Generate ID for the new catalog source
	generatedID, err := util.GenerateID(8)
	if err != nil {
		klog.Errorf("Failed to generate ID for catalog source: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate ID"})
		return
	}
	sourceID := "org-cat-src-" + generatedID

	catalogSource := &models.OrganizationCatalogSource{
		ID:              sourceID,
		OrgID:           orgID,
		SourceType:      req.SourceType,
		SourceName:      req.SourceName,
		SourceNamespace: req.SourceNamespace,
		Enabled:         req.Enabled,
	}

	if err := h.storage.CreateOrganizationCatalogSource(catalogSource); err != nil {
		klog.Errorf("Failed to create organization catalog source for org %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization catalog source"})
		return
	}

	klog.Infof("Created catalog source %s for organization %s", sourceID, orgID)
	c.JSON(http.StatusCreated, catalogSource)
}

// UpdateOrganizationCatalogSource handles updating an organization catalog source
func (h *CatalogHandlers) UpdateOrganizationCatalogSource(c *gin.Context) {
	orgID := c.Param("id")
	sourceID := c.Param("sourceId")
	if orgID == "" || sourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID and source ID required"})
		return
	}

	// Get user info from context
	_, _, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin and org admin can manage catalog sources
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to manage catalog sources"})
		return
	}

	// For org admin, ensure they can only manage sources for their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only manage catalog sources for your own organization"})
			return
		}
	}

	var req models.UpdateOrganizationCatalogSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get existing catalog source
	source, err := h.storage.GetOrganizationCatalogSource(sourceID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization catalog source not found"})
			return
		}
		klog.Errorf("Failed to get organization catalog source %s: %v", sourceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization catalog source"})
		return
	}

	// Verify source belongs to the organization
	if source.OrgID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization catalog source not found"})
		return
	}

	// Update fields if provided
	if req.SourceName != nil {
		source.SourceName = *req.SourceName
	}
	if req.Enabled != nil {
		source.Enabled = *req.Enabled
	}

	if err := h.storage.UpdateOrganizationCatalogSource(source); err != nil {
		klog.Errorf("Failed to update organization catalog source %s: %v", sourceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update organization catalog source"})
		return
	}

	klog.Infof("Updated catalog source %s for organization %s", sourceID, orgID)
	c.JSON(http.StatusOK, source)
}

// RemoveOrganizationCatalogSource handles removing a catalog source from an organization
func (h *CatalogHandlers) RemoveOrganizationCatalogSource(c *gin.Context) {
	orgID := c.Param("id")
	sourceID := c.Param("sourceId")
	if orgID == "" || sourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID and source ID required"})
		return
	}

	// Get user info from context
	_, _, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin and org admin can manage catalog sources
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to manage catalog sources"})
		return
	}

	// For org admin, ensure they can only manage sources for their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only manage catalog sources for your own organization"})
			return
		}
	}

	// Get existing catalog source to verify it belongs to the organization
	source, err := h.storage.GetOrganizationCatalogSource(sourceID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization catalog source not found"})
			return
		}
		klog.Errorf("Failed to get organization catalog source %s: %v", sourceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization catalog source"})
		return
	}

	// Verify source belongs to the organization
	if source.OrgID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization catalog source not found"})
		return
	}

	if err := h.storage.DeleteOrganizationCatalogSource(sourceID); err != nil {
		klog.Errorf("Failed to delete organization catalog source %s: %v", sourceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete organization catalog source"})
		return
	}

	klog.Infof("Deleted catalog source %s from organization %s", sourceID, orgID)
	c.JSON(http.StatusNoContent, nil)
}

// GetOrganizationCatalogTemplates handles getting catalog templates based on organization's assigned catalog sources
func (h *CatalogHandlers) GetOrganizationCatalogTemplates(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get user info from context
	_, _, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin can access any org, others can only access their own
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only access catalog templates for your own organization"})
			return
		}
	}

	// Get query parameters for filtering
	sourceType := c.Query("source_type") // Filter by specific source type
	category := c.Query("category")      // Operating System, Database, Application, etc.

	// Get organization's catalog sources
	orgSources, err := h.storage.ListOrganizationCatalogSources(orgID)
	if err != nil {
		klog.Errorf("Failed to list organization catalog sources for org %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list organization catalog sources"})
		return
	}

	// Filter enabled sources and optionally by source type
	var enabledSources []*models.OrganizationCatalogSource
	for _, source := range orgSources {
		if source.Enabled && (sourceType == "" || source.SourceType == sourceType) {
			enabledSources = append(enabledSources, source)
		}
	}

	var templates []*models.Template

	// Use catalog service if available to get templates from enabled sources
	if h.catalogService != nil {
		// For now, get all templates and filter by organization's sources
		// Future enhancement: optimize to only query specific sources
		allTemplates, err := h.catalogService.GetTemplates(c.Request.Context(), orgID, "", category)
		if err != nil {
			klog.Errorf("Failed to get templates from catalog service for org %s: %v", orgID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get catalog templates"})
			return
		}
		templates = allTemplates
	} else {
		// Fallback: return empty list if catalog service is not available
		templates = []*models.Template{}
		klog.Warningf("Catalog service not available, returning empty template list for organization %s", orgID)
	}

	klog.V(6).Infof("Retrieved %d catalog templates for organization %s (source_type: %s, category: %s)",
		len(templates), orgID, sourceType, category)

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
		"org_id":    orgID,
		"filters": gin.H{
			"source_type": sourceType,
			"category":    category,
		},
	})
}
