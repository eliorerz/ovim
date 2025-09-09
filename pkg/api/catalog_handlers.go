package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// CatalogHandlers handles VM catalog-related requests
type CatalogHandlers struct {
	storage storage.Storage
}

// NewCatalogHandlers creates a new catalog handlers instance
func NewCatalogHandlers(storage storage.Storage) *CatalogHandlers {
	return &CatalogHandlers{
		storage: storage,
	}
}

// ListTemplates handles listing all VM templates
func (h *CatalogHandlers) ListTemplates(c *gin.Context) {
	templates, err := h.storage.ListTemplates()
	if err != nil {
		klog.Errorf("Failed to list templates: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}

	klog.V(6).Infof("Listed %d templates", len(templates))
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
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
