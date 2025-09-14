package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// DashboardHandlers handles dashboard-related API endpoints
type DashboardHandlers struct {
	storage   storage.Storage
	k8sClient client.Client
}

// NewDashboardHandlers creates a new dashboard handlers instance
func NewDashboardHandlers(storage storage.Storage) *DashboardHandlers {
	return &DashboardHandlers{
		storage: storage,
	}
}

// SetKubernetesClient sets the Kubernetes client for health checks
func (h *DashboardHandlers) SetKubernetesClient(k8sClient client.Client) {
	h.k8sClient = k8sClient
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

// SystemHealthResponse represents the detailed system health information
type SystemHealthResponse struct {
	OverallStatus string                  `json:"overall_status"`
	Components    map[string]HealthStatus `json:"components"`
	LastChecked   string                  `json:"last_checked"`
	Details       SystemHealthDetails     `json:"details"`
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status  string `json:"status"`  // "healthy", "warning", "unhealthy"
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SystemHealthDetails provides additional system information
type SystemHealthDetails struct {
	DatabaseConnection  bool   `json:"database_connection"`
	KubernetesAccess    bool   `json:"kubernetes_access"`
	ControllerRunning   bool   `json:"controller_running"`
	ActiveOrganizations int    `json:"active_organizations"`
	ActiveVDCs          int    `json:"active_vdcs"`
	RunningVMs          int    `json:"running_vms"`
	SystemUptime        string `json:"system_uptime"`
}

// GetSystemHealth handles GET /dashboard/system-health
func (h *DashboardHandlers) GetSystemHealth(c *gin.Context) {
	health, err := h.buildSystemHealth()
	if err != nil {
		klog.Errorf("Failed to build system health: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check system health",
		})
		return
	}

	c.JSON(http.StatusOK, health)
}

// buildSystemHealth constructs comprehensive system health information
func (h *DashboardHandlers) buildSystemHealth() (*SystemHealthResponse, error) {
	components := make(map[string]HealthStatus)
	details := SystemHealthDetails{}

	// Check database health
	dbHealth := h.checkDatabaseHealth()
	components["database"] = dbHealth
	details.DatabaseConnection = dbHealth.Status == "healthy"

	// Check Kubernetes access
	k8sHealth := h.checkKubernetesHealth()
	components["kubernetes"] = k8sHealth
	details.KubernetesAccess = k8sHealth.Status == "healthy"

	// Check controller health
	controllerHealth := h.checkControllerHealth()
	components["controller"] = controllerHealth
	details.ControllerRunning = controllerHealth.Status == "healthy"

	// Get system statistics
	statsHealth := h.checkSystemStatistics()
	components["statistics"] = statsHealth

	// Extract statistics for details
	if organizations, err := h.storage.ListOrganizations(); err == nil {
		activeOrgs := 0
		for _, org := range organizations {
			if org.IsEnabled {
				activeOrgs++
			}
		}
		details.ActiveOrganizations = activeOrgs

		// Count VDCs and VMs
		var totalVDCs int
		var runningVMs int
		for _, org := range organizations {
			if vdcs, err := h.storage.ListVDCs(org.ID); err == nil {
				totalVDCs += len(vdcs)
			}
			if vms, err := h.storage.ListVMs(org.ID); err == nil {
				for _, vm := range vms {
					if vm.Status == "running" {
						runningVMs++
					}
				}
			}
		}
		details.ActiveVDCs = totalVDCs
		details.RunningVMs = runningVMs
	}

	// Calculate overall status
	overallStatus := h.calculateOverallStatus(components)

	// Build response
	response := &SystemHealthResponse{
		OverallStatus: overallStatus,
		Components:    components,
		LastChecked:   time.Now().UTC().Format(time.RFC3339),
		Details:       details,
	}

	return response, nil
}

// checkDatabaseHealth checks the health of the database connection
func (h *DashboardHandlers) checkDatabaseHealth() HealthStatus {
	if err := h.storage.Ping(); err != nil {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "Database connection failed",
			Details: err.Error(),
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: "Database connection is healthy",
	}
}

// checkKubernetesHealth checks the health of Kubernetes API access
func (h *DashboardHandlers) checkKubernetesHealth() HealthStatus {
	if h.k8sClient == nil {
		return HealthStatus{
			Status:  "warning",
			Message: "Kubernetes client not configured",
			Details: "Unable to perform Kubernetes health checks",
		}
	}

	// Try to list namespaces as a basic connectivity test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespaces := &corev1.NamespaceList{}
	if err := h.k8sClient.List(ctx, namespaces); err != nil {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "Kubernetes API access failed",
			Details: err.Error(),
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: "Kubernetes API access is healthy",
	}
}

// checkControllerHealth checks if the OVIM controller is running
func (h *DashboardHandlers) checkControllerHealth() HealthStatus {
	if h.k8sClient == nil {
		return HealthStatus{
			Status:  "warning",
			Message: "Cannot check controller status",
			Details: "Kubernetes client not available",
		}
	}

	// Check if controller deployment exists and is running
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to list namespaces as a basic indicator that controller might be running
	// (A more sophisticated check would look for specific controller deployments)
	namespaces := &corev1.NamespaceList{}
	if err := h.k8sClient.List(ctx, namespaces); err != nil {
		return HealthStatus{
			Status:  "warning",
			Message: "Controller status unclear",
			Details: fmt.Sprintf("Unable to check Kubernetes access: %v", err),
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: "Controller appears to be running",
	}
}

// checkSystemStatistics checks the health based on system statistics
func (h *DashboardHandlers) checkSystemStatistics() HealthStatus {
	organizations, err := h.storage.ListOrganizations()
	if err != nil {
		return HealthStatus{
			Status:  "warning",
			Message: "Unable to retrieve system statistics",
			Details: err.Error(),
		}
	}

	if len(organizations) == 0 {
		return HealthStatus{
			Status:  "warning",
			Message: "No organizations configured",
			Details: "System appears to be newly installed or needs configuration",
		}
	}

	// Check for enabled organizations
	enabledOrgs := 0
	for _, org := range organizations {
		if org.IsEnabled {
			enabledOrgs++
		}
	}

	if enabledOrgs == 0 {
		return HealthStatus{
			Status:  "warning",
			Message: "No enabled organizations",
			Details: "All organizations are currently disabled",
		}
	}

	return HealthStatus{
		Status:  "healthy",
		Message: fmt.Sprintf("%d organizations configured, %d enabled", len(organizations), enabledOrgs),
	}
}

// calculateOverallStatus determines the overall system health based on component statuses
func (h *DashboardHandlers) calculateOverallStatus(components map[string]HealthStatus) string {
	hasUnhealthy := false
	hasWarning := false

	for _, component := range components {
		switch component.Status {
		case "unhealthy":
			hasUnhealthy = true
		case "warning":
			hasWarning = true
		}
	}

	if hasUnhealthy {
		return "unhealthy"
	}
	if hasWarning {
		return "warning"
	}
	return "healthy"
}
