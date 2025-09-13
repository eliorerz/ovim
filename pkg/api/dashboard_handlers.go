package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// DashboardHandlers handles dashboard-related API endpoints
type DashboardHandlers struct {
	storage storage.Storage
}

// NewDashboardHandlers creates a new dashboard handlers instance
func NewDashboardHandlers(storage storage.Storage) *DashboardHandlers {
	return &DashboardHandlers{
		storage: storage,
	}
}

// DashboardSummary represents the overall system summary
type DashboardSummary struct {
	TotalOrganizations   int    `json:"total_organizations"`
	EnabledOrganizations int    `json:"enabled_organizations"`
	TotalVDCs            int    `json:"total_vdcs"`
	TotalVMs             int    `json:"total_vms"`
	RunningVMs           int    `json:"running_vms"`
	TotalPods            int    `json:"total_pods"`
	RunningPods          int    `json:"running_pods"`
	TotalTemplates       int    `json:"total_templates"`
	SystemHealth         string `json:"system_health"`
	LastUpdated          string `json:"last_updated"`
}

// GetSummary handles GET /dashboard/summary
func (h *DashboardHandlers) GetSummary(c *gin.Context) {
	summary, err := h.buildDashboardSummary()
	if err != nil {
		klog.Errorf("Failed to build dashboard summary: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to build dashboard summary",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// buildDashboardSummary constructs the dashboard summary by aggregating data from storage
func (h *DashboardHandlers) buildDashboardSummary() (*DashboardSummary, error) {
	// Get organizations
	organizations, err := h.storage.ListOrganizations()
	if err != nil {
		return nil, err
	}

	enabledOrgs := 0
	for _, org := range organizations {
		if org.IsEnabled {
			enabledOrgs++
		}
	}

	// Get VDCs across all organizations
	var allVDCs []*models.VirtualDataCenter
	for _, org := range organizations {
		vdcs, err := h.storage.ListVDCs(org.ID)
		if err != nil {
			return nil, err
		}
		allVDCs = append(allVDCs, vdcs...)
	}

	// Get VMs across all organizations
	var allVMs []*models.VirtualMachine
	for _, org := range organizations {
		vms, err := h.storage.ListVMs(org.ID)
		if err != nil {
			return nil, err
		}
		allVMs = append(allVMs, vms...)
	}

	runningVMs := 0
	for _, vm := range allVMs {
		if vm.Status == "running" {
			runningVMs++
		}
	}

	// Get templates
	templates, err := h.storage.ListTemplates()
	if err != nil {
		return nil, err
	}

	// Determine system health (simplified logic)
	systemHealth := "healthy"
	if len(organizations) == 0 || len(templates) == 0 {
		systemHealth = "warning"
	}

	summary := &DashboardSummary{
		TotalOrganizations:   len(organizations),
		EnabledOrganizations: enabledOrgs,
		TotalVDCs:            len(allVDCs),
		TotalVMs:             len(allVMs),
		RunningVMs:           runningVMs,
		TotalPods:            0, // TODO: Implement pod counting from k8s
		RunningPods:          0, // TODO: Implement running pod counting from k8s
		TotalTemplates:       len(templates),
		SystemHealth:         systemHealth,
		LastUpdated:          time.Now().UTC().Format(time.RFC3339),
	}

	return summary, nil
}
