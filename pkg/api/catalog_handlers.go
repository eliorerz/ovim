package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/catalog"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
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

	// Get user organization ID from context (would be set by auth middleware)
	userOrgID := ""
	if orgID, exists := c.Get("user_org_id"); exists {
		if str, ok := orgID.(string); ok {
			userOrgID = str
		}
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
	// Get user organization ID from context (would be set by auth middleware)
	userOrgID := ""
	if orgID, exists := c.Get("user_org_id"); exists {
		if str, ok := orgID.(string); ok {
			userOrgID = str
		}
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
