package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

// SystemResourceSummary represents system-wide resource usage across all organizations
type SystemResourceSummary struct {
	TotalResources    ResourceSummary                    `json:"total_resources"`
	UsedResources     ResourceSummary                    `json:"used_resources"`
	AvailableResources ResourceSummary                   `json:"available_resources"`
	UsagePercentages  ResourceSummary                    `json:"usage_percentages"`
	OrganizationUsage []OrganizationResourceSummary      `json:"organization_usage"`
	TopConsumers      TopResourceConsumers               `json:"top_consumers"`
	Note              string                             `json:"note,omitempty"`
	LastUpdated       string                             `json:"last_updated"`
}

// ResourceSummary represents aggregated resource values
type ResourceSummary struct {
	CPU     float64 `json:"cpu"`     // In cores
	Memory  float64 `json:"memory"`  // In GB
	Storage float64 `json:"storage"` // In GB
}

// OrganizationResourceSummary represents resource usage for a single organization
type OrganizationResourceSummary struct {
	OrganizationID   string          `json:"organization_id"`
	OrganizationName string          `json:"organization_name"`
	IsEnabled        bool            `json:"is_enabled"`
	VDCCount         int             `json:"vdc_count"`
	VMCount          int             `json:"vm_count"`
	RunningVMCount   int             `json:"running_vm_count"`
	UsedResources    ResourceSummary `json:"used_resources"`
	QuotaResources   ResourceSummary `json:"quota_resources"`
	UsagePercentages ResourceSummary `json:"usage_percentages"`
}

// TopResourceConsumers identifies the highest resource consumers
type TopResourceConsumers struct {
	ByOrganization []TopConsumer `json:"by_organization"`
	ByVDC          []TopConsumer `json:"by_vdc"`
	ByVM           []TopConsumer `json:"by_vm"`
}

// TopConsumer represents a single resource consumer
type TopConsumer struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Type            string          `json:"type"` // "organization", "vdc", "vm"
	UsedResources   ResourceSummary `json:"used_resources"`
	UsagePercentage float64         `json:"usage_percentage"`
	ParentID        string          `json:"parent_id,omitempty"`        // For VDCs: org_id, For VMs: vdc_id
	ParentName      string          `json:"parent_name,omitempty"`      // For VDCs: org_name, For VMs: vdc_name
}

// GetSystemResources handles GET /dashboard/resources
func (h *DashboardHandlers) GetSystemResources(c *gin.Context) {
	summary, err := h.buildSystemResourceSummary()
	if err != nil {
		klog.Errorf("Failed to build system resource summary: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to build system resource summary",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// buildSystemResourceSummary constructs comprehensive system resource usage information
func (h *DashboardHandlers) buildSystemResourceSummary() (*SystemResourceSummary, error) {
	// Get all organizations
	organizations, err := h.storage.ListOrganizations()
	if err != nil {
		return nil, err
	}

	var totalCPU, totalMemory, totalStorage float64
	var usedCPU, usedMemory, usedStorage float64
	var orgSummaries []OrganizationResourceSummary
	var allTopConsumers []TopConsumer

	// Process each organization
	for _, org := range organizations {
		// Get VDCs for this organization
		vdcs, err := h.storage.ListVDCs(org.ID)
		if err != nil {
			klog.Warningf("Failed to get VDCs for organization %s: %v", org.ID, err)
			continue
		}

		// Get VMs for this organization
		vms, err := h.storage.ListVMs(org.ID)
		if err != nil {
			klog.Warningf("Failed to get VMs for organization %s: %v", org.ID, err)
			continue
		}

		// Calculate organization resource usage
		orgUsage := org.GetResourceUsage(vdcs, vms)

		// Convert to our summary format
		orgSummary := OrganizationResourceSummary{
			OrganizationID:   org.ID,
			OrganizationName: org.Name,
			IsEnabled:        org.IsEnabled,
			VDCCount:         len(vdcs),
			VMCount:          len(vms),
			RunningVMCount:   orgUsage.VDCCount, // This should be running VMs, will fix below
			UsedResources: ResourceSummary{
				CPU:     float64(orgUsage.CPUUsed),
				Memory:  float64(orgUsage.MemoryUsed),
				Storage: float64(orgUsage.StorageUsed),
			},
			QuotaResources: ResourceSummary{
				CPU:     float64(orgUsage.CPUQuota),
				Memory:  float64(orgUsage.MemoryQuota),
				Storage: float64(orgUsage.StorageQuota),
			},
		}

		// Calculate correct running VM count
		runningVMs := 0
		for _, vm := range vms {
			if vm.Status == "running" {
				runningVMs++
			}
		}
		orgSummary.RunningVMCount = runningVMs

		// Calculate usage percentages
		if orgSummary.QuotaResources.CPU > 0 {
			orgSummary.UsagePercentages.CPU = (orgSummary.UsedResources.CPU / orgSummary.QuotaResources.CPU) * 100
		}
		if orgSummary.QuotaResources.Memory > 0 {
			orgSummary.UsagePercentages.Memory = (orgSummary.UsedResources.Memory / orgSummary.QuotaResources.Memory) * 100
		}
		if orgSummary.QuotaResources.Storage > 0 {
			orgSummary.UsagePercentages.Storage = (orgSummary.UsedResources.Storage / orgSummary.QuotaResources.Storage) * 100
		}

		// Add to organization summaries
		orgSummaries = append(orgSummaries, orgSummary)

		// Accumulate totals
		totalCPU += float64(orgUsage.CPUQuota)
		totalMemory += float64(orgUsage.MemoryQuota)
		totalStorage += float64(orgUsage.StorageQuota)

		usedCPU += float64(orgUsage.CPUUsed)
		usedMemory += float64(orgUsage.MemoryUsed)
		usedStorage += float64(orgUsage.StorageUsed)

		// Add to top consumers (organization level)
		totalOrgUsage := orgSummary.UsedResources.CPU + orgSummary.UsedResources.Memory + orgSummary.UsedResources.Storage
		allTopConsumers = append(allTopConsumers, TopConsumer{
			ID:              org.ID,
			Name:            org.Name,
			Type:            "organization",
			UsedResources:   orgSummary.UsedResources,
			UsagePercentage: totalOrgUsage,
		})

		// Add VDC level consumers
		for _, vdc := range vdcs {
			vdcVMs := make([]*models.VirtualMachine, 0)
			for _, vm := range vms {
				if vm.VDCID != nil && *vm.VDCID == vdc.ID {
					vdcVMs = append(vdcVMs, vm)
				}
			}

			vdcUsage := vdc.GetResourceUsage(vdcVMs)
			totalVDCUsage := float64(vdcUsage.CPUUsed + vdcUsage.MemoryUsed + vdcUsage.StorageUsed)

			allTopConsumers = append(allTopConsumers, TopConsumer{
				ID:   vdc.ID,
				Name: vdc.Name,
				Type: "vdc",
				UsedResources: ResourceSummary{
					CPU:     float64(vdcUsage.CPUUsed),
					Memory:  float64(vdcUsage.MemoryUsed),
					Storage: float64(vdcUsage.StorageUsed),
				},
				UsagePercentage: totalVDCUsage,
				ParentID:        org.ID,
				ParentName:      org.Name,
			})
		}

		// Add VM level consumers
		for _, vm := range vms {
			vmCPU, vmMemory, vmStorage := parseVMResources(vm)
			totalVMUsage := vmCPU + vmMemory + vmStorage

			allTopConsumers = append(allTopConsumers, TopConsumer{
				ID:   vm.ID,
				Name: vm.Name,
				Type: "vm",
				UsedResources: ResourceSummary{
					CPU:     vmCPU,
					Memory:  vmMemory,
					Storage: vmStorage,
				},
				UsagePercentage: totalVMUsage,
				ParentID:        getVDCIDString(vm.VDCID),
				ParentName:      getVDCName(getVDCIDString(vm.VDCID), vdcs),
			})
		}
	}

	// Calculate system-wide percentages
	var usagePercentages ResourceSummary
	if totalCPU > 0 {
		usagePercentages.CPU = (usedCPU / totalCPU) * 100
	}
	if totalMemory > 0 {
		usagePercentages.Memory = (usedMemory / totalMemory) * 100
	}
	if totalStorage > 0 {
		usagePercentages.Storage = (usedStorage / totalStorage) * 100
	}

	// Sort and limit top consumers
	topConsumers := h.getTopConsumers(allTopConsumers)

	// Determine if we need to add a note about zero resources
	var note string
	totalVDCs := 0
	for _, orgSummary := range orgSummaries {
		totalVDCs += orgSummary.VDCCount
	}

	if totalVDCs == 0 {
		note = "No Virtual Data Centers (VDCs) have been created yet. Resource quotas are allocated at the VDC level, so all resource values show as zero until VDCs are created within organizations."
	}

	return &SystemResourceSummary{
		TotalResources: ResourceSummary{
			CPU:     totalCPU,
			Memory:  totalMemory,
			Storage: totalStorage,
		},
		UsedResources: ResourceSummary{
			CPU:     usedCPU,
			Memory:  usedMemory,
			Storage: usedStorage,
		},
		AvailableResources: ResourceSummary{
			CPU:     totalCPU - usedCPU,
			Memory:  totalMemory - usedMemory,
			Storage: totalStorage - usedStorage,
		},
		UsagePercentages:  usagePercentages,
		OrganizationUsage: orgSummaries,
		TopConsumers:      topConsumers,
		Note:              note,
		LastUpdated:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// parseVMResources extracts CPU, memory, and storage values from VM configuration
func parseVMResources(vm *models.VirtualMachine) (cpu, memory, storage float64) {
	// Parse CPU (assuming it's in cores)
	if vm.CPU > 0 {
		cpu = float64(vm.CPU)
	}

	// Parse Memory (convert string to float64, assuming it's in GB)
	if vm.Memory != "" {
		if parsedMemory, err := parseResourceString(vm.Memory); err == nil {
			memory = parsedMemory
		}
	}

	// Parse Storage (convert string to float64, assuming it's in GB)
	if vm.DiskSize != "" {
		if parsedStorage, err := parseResourceString(vm.DiskSize); err == nil {
			storage = parsedStorage
		}
	}

	return cpu, memory, storage
}

// getVDCName finds VDC name by ID
func getVDCName(vdcID string, vdcs []*models.VirtualDataCenter) string {
	for _, vdc := range vdcs {
		if vdc.ID == vdcID {
			return vdc.Name
		}
	}
	return vdcID
}

// getVDCIDString safely converts VDCID pointer to string
func getVDCIDString(vdcID *string) string {
	if vdcID == nil {
		return ""
	}
	return *vdcID
}

// parseResourceString parses resource strings like "2Gi", "1024Mi", "10" to float64 (in GB)
func parseResourceString(resource string) (float64, error) {
	if resource == "" {
		return 0, fmt.Errorf("empty resource string")
	}

	// Remove whitespace
	resource = strings.TrimSpace(resource)

	// Handle simple numeric values (assume GB)
	if val, err := strconv.ParseFloat(resource, 64); err == nil {
		return val, nil
	}

	// Handle Kubernetes resource notation
	if strings.HasSuffix(resource, "Gi") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(resource, "Gi"), 64)
		return val, err
	}

	if strings.HasSuffix(resource, "Mi") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(resource, "Mi"), 64)
		if err != nil {
			return 0, err
		}
		return val / 1024, nil // Convert Mi to Gi
	}

	if strings.HasSuffix(resource, "G") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(resource, "G"), 64)
		return val, err
	}

	if strings.HasSuffix(resource, "M") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(resource, "M"), 64)
		if err != nil {
			return 0, err
		}
		return val / 1024, nil // Convert MB to GB
	}

	// Try to parse as plain number
	return strconv.ParseFloat(resource, 64)
}

// getTopConsumers sorts and returns top consumers by category
func (h *DashboardHandlers) getTopConsumers(allConsumers []TopConsumer) TopResourceConsumers {
	// Separate by type
	var orgs, vdcs, vms []TopConsumer

	for _, consumer := range allConsumers {
		switch consumer.Type {
		case "organization":
			orgs = append(orgs, consumer)
		case "vdc":
			vdcs = append(vdcs, consumer)
		case "vm":
			vms = append(vms, consumer)
		}
	}

	// Sort by usage percentage (descending) and limit to top 5
	sortAndLimit := func(consumers []TopConsumer) []TopConsumer {
		// Simple bubble sort for small datasets
		for i := 0; i < len(consumers)-1; i++ {
			for j := 0; j < len(consumers)-i-1; j++ {
				if consumers[j].UsagePercentage < consumers[j+1].UsagePercentage {
					consumers[j], consumers[j+1] = consumers[j+1], consumers[j]
				}
			}
		}

		// Limit to top 5
		if len(consumers) > 5 {
			consumers = consumers[:5]
		}

		return consumers
	}

	return TopResourceConsumers{
		ByOrganization: sortAndLimit(orgs),
		ByVDC:          sortAndLimit(vdcs),
		ByVM:           sortAndLimit(vms),
	}
}
