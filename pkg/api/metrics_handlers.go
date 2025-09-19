package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// MetricsHandlers handles real-time metrics API endpoints
type MetricsHandlers struct {
	storage   storage.Storage
	k8sClient client.Client
	upgrader  websocket.Upgrader
}

// NewMetricsHandlers creates a new metrics handlers instance
func NewMetricsHandlers(storage storage.Storage, k8sClient client.Client) *MetricsHandlers {
	return &MetricsHandlers{
		storage:   storage,
		k8sClient: k8sClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for development - should be restricted in production
				return true
			},
		},
	}
}

// RealTimeMetrics represents real-time system metrics
type RealTimeMetrics struct {
	Timestamp         string                `json:"timestamp"`
	SystemSummary     SystemMetricsSummary  `json:"system_summary"`
	OrganizationUsage []OrganizationMetrics `json:"organization_usage"`
	VDCMetrics        []VDCMetrics          `json:"vdc_metrics"`
	ResourceTrends    ResourceTrends        `json:"resource_trends"`
	AlertSummary      MetricsAlertSummary   `json:"alert_summary"`
	PerformanceStats  PerformanceStats      `json:"performance_stats"`
}

// SystemMetricsSummary represents high-level system metrics
type SystemMetricsSummary struct {
	TotalOrganizations  int             `json:"total_organizations"`
	ActiveVDCs          int             `json:"active_vdcs"`
	TotalVMs            int             `json:"total_vms"`
	RunningVMs          int             `json:"running_vms"`
	ResourceUtilization ResourceSummary `json:"resource_utilization"`
	SystemHealth        string          `json:"system_health"`
}

// OrganizationMetrics represents metrics for an organization
type OrganizationMetrics struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	IsEnabled        bool            `json:"is_enabled"`
	VDCCount         int             `json:"vdc_count"`
	VMCount          int             `json:"vm_count"`
	RunningVMCount   int             `json:"running_vm_count"`
	ResourceUsage    ResourceSummary `json:"resource_usage"`
	ResourceQuota    ResourceSummary `json:"resource_quota"`
	UsagePercentages ResourceSummary `json:"usage_percentages"`
	LastUpdated      string          `json:"last_updated"`
}

// VDCMetrics represents metrics for a Virtual Data Center
type VDCMetrics struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	OrganizationID    string          `json:"organization_id"`
	Namespace         string          `json:"namespace"`
	Phase             string          `json:"phase"`
	PodCount          int             `json:"pod_count"`
	RunningPods       int             `json:"running_pods"`
	VMCount           int             `json:"vm_count"`
	RunningVMs        int             `json:"running_vms"`
	ResourceUsage     ResourceSummary `json:"resource_usage"`
	ResourceQuota     ResourceSummary `json:"resource_quota"`
	UsagePercentages  ResourceSummary `json:"usage_percentages"`
	NetworkPolicy     string          `json:"network_policy"`
	LastMetricsUpdate string          `json:"last_metrics_update"`
}

// ResourceTrends represents resource usage trends over time
type ResourceTrends struct {
	CPUTrend     []TrendDataPoint `json:"cpu_trend"`
	MemoryTrend  []TrendDataPoint `json:"memory_trend"`
	StorageTrend []TrendDataPoint `json:"storage_trend"`
	VMTrend      []TrendDataPoint `json:"vm_trend"`
}

// TrendDataPoint represents a single point in a trend
type TrendDataPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// MetricsAlertSummary represents alerts related to metrics
type MetricsAlertSummary struct {
	TotalAlerts       int               `json:"total_alerts"`
	CriticalAlerts    int               `json:"critical_alerts"`
	WarningAlerts     int               `json:"warning_alerts"`
	ResourceAlerts    []ResourceAlert   `json:"resource_alerts"`
	ThresholdBreaches []ThresholdBreach `json:"threshold_breaches"`
}

// ResourceAlert represents a resource-related alert
type ResourceAlert struct {
	Type         string  `json:"type"`          // "cpu", "memory", "storage"
	Severity     string  `json:"severity"`      // "critical", "warning", "info"
	Target       string  `json:"target"`        // organization/vdc/vm ID
	TargetName   string  `json:"target_name"`   // human-readable name
	CurrentUsage float64 `json:"current_usage"` // percentage
	Threshold    float64 `json:"threshold"`     // percentage
	Message      string  `json:"message"`
}

// ThresholdBreach represents a resource threshold breach
type ThresholdBreach struct {
	ResourceType   string    `json:"resource_type"`
	TargetID       string    `json:"target_id"`
	TargetName     string    `json:"target_name"`
	TargetType     string    `json:"target_type"` // "organization", "vdc", "vm"
	CurrentValue   float64   `json:"current_value"`
	ThresholdValue float64   `json:"threshold_value"`
	Severity       string    `json:"severity"`
	Duration       string    `json:"duration"`
	FirstDetected  time.Time `json:"first_detected"`
}

// PerformanceStats represents system performance statistics
type PerformanceStats struct {
	MetricsCollectionLatency float64 `json:"metrics_collection_latency_ms"`
	DatabaseResponseTime     float64 `json:"database_response_time_ms"`
	KubernetesAPILatency     float64 `json:"kubernetes_api_latency_ms"`
	ActiveConnections        int     `json:"active_connections"`
	MemoryUsage              float64 `json:"memory_usage_mb"`
	CPUUsage                 float64 `json:"cpu_usage_percent"`
}

// GetRealTimeMetrics handles GET /metrics/realtime
func (h *MetricsHandlers) GetRealTimeMetrics(c *gin.Context) {
	metrics, err := h.collectRealTimeMetrics()
	if err != nil {
		klog.Errorf("Failed to collect real-time metrics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to collect real-time metrics",
		})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// WebSocketMetrics handles WebSocket connections for real-time metrics streaming
func (h *MetricsHandlers) WebSocketMetrics(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		klog.Errorf("Failed to upgrade to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	// Parse query parameters for configuration
	intervalStr := c.DefaultQuery("interval", "5")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval < 1 || interval > 60 {
		interval = 5 // Default to 5 seconds
	}

	filterOrg := c.Query("organization")
	filterVDC := c.Query("vdc")

	klog.Infof("WebSocket metrics connection established, interval: %ds, org filter: %s, vdc filter: %s",
		interval, filterOrg, filterVDC)

	// Create ticker for periodic metrics collection
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Send initial metrics immediately
	if err := h.sendMetricsUpdate(conn, filterOrg, filterVDC); err != nil {
		klog.Errorf("Failed to send initial metrics: %v", err)
		return
	}

	// Set up channels for graceful shutdown
	done := make(chan struct{})

	// Listen for client disconnect
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					klog.Errorf("WebSocket error: %v", err)
				}
				return
			}
		}
	}()

	// Send periodic updates
	for {
		select {
		case <-ticker.C:
			if err := h.sendMetricsUpdate(conn, filterOrg, filterVDC); err != nil {
				klog.Errorf("Failed to send metrics update: %v", err)
				return
			}
		case <-done:
			klog.Info("WebSocket metrics connection closed")
			return
		}
	}
}

// sendMetricsUpdate sends a metrics update through the WebSocket connection
func (h *MetricsHandlers) sendMetricsUpdate(conn *websocket.Conn, filterOrg, filterVDC string) error {
	metrics, err := h.collectRealTimeMetrics()
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %v", err)
	}

	// Apply filters if specified
	if filterOrg != "" {
		filteredOrgUsage := make([]OrganizationMetrics, 0)
		for _, org := range metrics.OrganizationUsage {
			if org.ID == filterOrg {
				filteredOrgUsage = append(filteredOrgUsage, org)
				break
			}
		}
		metrics.OrganizationUsage = filteredOrgUsage
	}

	if filterVDC != "" {
		filteredVDCMetrics := make([]VDCMetrics, 0)
		for _, vdc := range metrics.VDCMetrics {
			if vdc.ID == filterVDC {
				filteredVDCMetrics = append(filteredVDCMetrics, vdc)
				break
			}
		}
		metrics.VDCMetrics = filteredVDCMetrics
	}

	// Send metrics as JSON through WebSocket
	return conn.WriteJSON(metrics)
}

// collectRealTimeMetrics collects comprehensive real-time metrics
func (h *MetricsHandlers) collectRealTimeMetrics() (*RealTimeMetrics, error) {
	startTime := time.Now()

	// Get all organizations
	organizations, err := h.storage.ListOrganizations()
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %v", err)
	}

	var orgMetrics []OrganizationMetrics
	var vdcMetrics []VDCMetrics
	var totalVMs, runningVMs, activeVDCs int
	var systemResourceUsage ResourceSummary

	// Process each organization
	for _, org := range organizations {
		orgData, vdcData, err := h.collectOrganizationMetrics(org)
		if err != nil {
			klog.Warningf("Failed to collect metrics for organization %s: %v", org.ID, err)
			continue
		}

		orgMetrics = append(orgMetrics, *orgData)
		vdcMetrics = append(vdcMetrics, vdcData...)

		totalVMs += orgData.VMCount
		runningVMs += orgData.RunningVMCount
		activeVDCs += orgData.VDCCount

		// Accumulate system resource usage
		systemResourceUsage.CPU += orgData.ResourceUsage.CPU
		systemResourceUsage.Memory += orgData.ResourceUsage.Memory
		systemResourceUsage.Storage += orgData.ResourceUsage.Storage
	}

	// Build system summary
	systemSummary := SystemMetricsSummary{
		TotalOrganizations:  len(organizations),
		ActiveVDCs:          activeVDCs,
		TotalVMs:            totalVMs,
		RunningVMs:          runningVMs,
		ResourceUtilization: systemResourceUsage,
		SystemHealth:        h.calculateSystemHealth(organizations, vdcMetrics),
	}

	// Collect resource trends (simplified - in production you'd store historical data)
	resourceTrends := h.generateResourceTrends(systemResourceUsage, totalVMs)

	// Generate alerts
	alertSummary := h.generateMetricsAlerts(orgMetrics, vdcMetrics)

	// Calculate performance stats
	perfStats := h.calculatePerformanceStats(startTime)

	return &RealTimeMetrics{
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
		SystemSummary:     systemSummary,
		OrganizationUsage: orgMetrics,
		VDCMetrics:        vdcMetrics,
		ResourceTrends:    resourceTrends,
		AlertSummary:      alertSummary,
		PerformanceStats:  perfStats,
	}, nil
}

// collectOrganizationMetrics collects metrics for a specific organization
func (h *MetricsHandlers) collectOrganizationMetrics(org *models.Organization) (*OrganizationMetrics, []VDCMetrics, error) {
	// Get VDCs for this organization
	vdcs, err := h.storage.ListVDCs(org.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list VDCs: %v", err)
	}

	// Get VMs for this organization
	vms, err := h.storage.ListVMs(org.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list VMs: %v", err)
	}

	// Calculate organization-level metrics
	orgUsage := org.GetResourceUsage(vdcs, vms)

	runningVMs := 0
	for _, vm := range vms {
		if vm.Status == "running" {
			runningVMs++
		}
	}

	orgMetrics := &OrganizationMetrics{
		ID:             org.ID,
		Name:           org.Name,
		IsEnabled:      org.IsEnabled,
		VDCCount:       len(vdcs),
		VMCount:        len(vms),
		RunningVMCount: runningVMs,
		ResourceUsage: ResourceSummary{
			CPU:     float64(orgUsage.CPUUsed),
			Memory:  float64(orgUsage.MemoryUsed),
			Storage: float64(orgUsage.StorageUsed),
		},
		ResourceQuota: ResourceSummary{
			CPU:     float64(orgUsage.CPUQuota),
			Memory:  float64(orgUsage.MemoryQuota),
			Storage: float64(orgUsage.StorageQuota),
		},
		LastUpdated: time.Now().UTC().Format(time.RFC3339),
	}

	// Calculate usage percentages
	if orgMetrics.ResourceQuota.CPU > 0 {
		orgMetrics.UsagePercentages.CPU = (orgMetrics.ResourceUsage.CPU / orgMetrics.ResourceQuota.CPU) * 100
	}
	if orgMetrics.ResourceQuota.Memory > 0 {
		orgMetrics.UsagePercentages.Memory = (orgMetrics.ResourceUsage.Memory / orgMetrics.ResourceQuota.Memory) * 100
	}
	if orgMetrics.ResourceQuota.Storage > 0 {
		orgMetrics.UsagePercentages.Storage = (orgMetrics.ResourceUsage.Storage / orgMetrics.ResourceQuota.Storage) * 100
	}

	// Collect VDC-level metrics
	var vdcMetricsList []VDCMetrics
	for _, vdc := range vdcs {
		vdcData, err := h.collectVDCMetrics(vdc, vms)
		if err != nil {
			klog.Warningf("Failed to collect metrics for VDC %s: %v", vdc.ID, err)
			continue
		}
		vdcMetricsList = append(vdcMetricsList, *vdcData)
	}

	return orgMetrics, vdcMetricsList, nil
}

// collectVDCMetrics collects metrics for a specific VDC
func (h *MetricsHandlers) collectVDCMetrics(vdc *models.VirtualDataCenter, allVMs []*models.VirtualMachine) (*VDCMetrics, error) {
	// Filter VMs for this VDC
	var vdcVMs []*models.VirtualMachine
	for _, vm := range allVMs {
		if vm.VDCID != nil && *vm.VDCID == vdc.ID {
			vdcVMs = append(vdcVMs, vm)
		}
	}

	runningVMs := 0
	for _, vm := range vdcVMs {
		if vm.Status == "running" {
			runningVMs++
		}
	}

	// Get real-time metrics from Kubernetes if available
	var podCount, runningPods int
	var k8sMetrics *ResourceSummary
	if h.k8sClient != nil && vdc.WorkloadNamespace != "" {
		podMetrics, err := h.collectKubernetesMetrics(vdc.WorkloadNamespace)
		if err != nil {
			klog.Warningf("Failed to collect Kubernetes metrics for VDC %s: %v", vdc.ID, err)
		} else {
			podCount = podMetrics.TotalPods
			runningPods = podMetrics.RunningPods
			k8sMetrics = &ResourceSummary{
				CPU:     float64(podMetrics.CPUUsed),
				Memory:  float64(podMetrics.MemoryUsed),
				Storage: float64(podMetrics.StorageUsed),
			}
		}
	}

	// Calculate VDC resource usage
	vdcUsage := vdc.GetResourceUsage(vdcVMs)

	// Use Kubernetes metrics if available, otherwise use calculated metrics
	resourceUsage := ResourceSummary{
		CPU:     float64(vdcUsage.CPUUsed),
		Memory:  float64(vdcUsage.MemoryUsed),
		Storage: float64(vdcUsage.StorageUsed),
	}
	if k8sMetrics != nil {
		resourceUsage = *k8sMetrics
	}

	vdcMetrics := &VDCMetrics{
		ID:             vdc.ID,
		Name:           vdc.Name,
		OrganizationID: vdc.OrgID,
		Namespace:      vdc.WorkloadNamespace,
		Phase:          vdc.Phase,
		PodCount:       podCount,
		RunningPods:    runningPods,
		VMCount:        len(vdcVMs),
		RunningVMs:     runningVMs,
		ResourceUsage:  resourceUsage,
		ResourceQuota: ResourceSummary{
			CPU:     float64(vdc.CPUQuota),
			Memory:  float64(vdc.MemoryQuota),
			Storage: float64(vdc.StorageQuota),
		},
		NetworkPolicy:     vdc.NetworkPolicy,
		LastMetricsUpdate: time.Now().UTC().Format(time.RFC3339),
	}

	// Calculate usage percentages
	if vdcMetrics.ResourceQuota.CPU > 0 {
		vdcMetrics.UsagePercentages.CPU = (vdcMetrics.ResourceUsage.CPU / vdcMetrics.ResourceQuota.CPU) * 100
	}
	if vdcMetrics.ResourceQuota.Memory > 0 {
		vdcMetrics.UsagePercentages.Memory = (vdcMetrics.ResourceUsage.Memory / vdcMetrics.ResourceQuota.Memory) * 100
	}
	if vdcMetrics.ResourceQuota.Storage > 0 {
		vdcMetrics.UsagePercentages.Storage = (vdcMetrics.ResourceUsage.Storage / vdcMetrics.ResourceQuota.Storage) * 100
	}

	return vdcMetrics, nil
}

// KubernetesMetrics holds metrics collected from Kubernetes
type KubernetesMetrics struct {
	TotalPods   int
	RunningPods int
	CPUUsed     float64
	MemoryUsed  float64
	StorageUsed float64
}

// collectKubernetesMetrics collects real-time metrics from Kubernetes namespace
func (h *MetricsHandlers) collectKubernetesMetrics(namespace string) (*KubernetesMetrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get pods in the namespace
	pods := &corev1.PodList{}
	if err := h.k8sClient.List(ctx, pods, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	metrics := &KubernetesMetrics{
		TotalPods: len(pods.Items),
	}

	var cpuUsed, memoryUsed resource.Quantity
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			metrics.RunningPods++
		}

		// Sum up resource requests from containers
		for _, container := range pod.Spec.Containers {
			if cpu := container.Resources.Requests[corev1.ResourceCPU]; !cpu.IsZero() {
				cpuUsed.Add(cpu)
			}
			if memory := container.Resources.Requests[corev1.ResourceMemory]; !memory.IsZero() {
				memoryUsed.Add(memory)
			}
		}
	}

	// Convert to float64 values in standard units
	metrics.CPUUsed = float64(cpuUsed.MilliValue()) / 1000.0                // Convert millicores to cores
	metrics.MemoryUsed = float64(memoryUsed.Value()) / (1024 * 1024 * 1024) // Convert bytes to GB

	// Get storage usage from PVCs
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := h.k8sClient.List(ctx, pvcs, client.InNamespace(namespace)); err == nil {
		var storageUsed resource.Quantity
		for _, pvc := range pvcs.Items {
			if pvc.Status.Phase == corev1.ClaimBound {
				if storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; !storage.IsZero() {
					storageUsed.Add(storage)
				}
			}
		}
		metrics.StorageUsed = float64(storageUsed.Value()) / (1024 * 1024 * 1024) // Convert bytes to GB
	}

	return metrics, nil
}

// calculateSystemHealth determines overall system health
func (h *MetricsHandlers) calculateSystemHealth(orgs []*models.Organization, vdcs []VDCMetrics) string {
	if len(orgs) == 0 {
		return "warning"
	}

	enabledOrgs := 0
	for _, org := range orgs {
		if org.IsEnabled {
			enabledOrgs++
		}
	}

	if enabledOrgs == 0 {
		return "warning"
	}

	// Check for resource utilization issues
	highUtilizationCount := 0
	for _, vdc := range vdcs {
		if vdc.UsagePercentages.CPU > 90 || vdc.UsagePercentages.Memory > 90 || vdc.UsagePercentages.Storage > 90 {
			highUtilizationCount++
		}
	}

	if highUtilizationCount > len(vdcs)/2 {
		return "critical"
	}

	if highUtilizationCount > 0 {
		return "warning"
	}

	return "healthy"
}

// generateResourceTrends generates simulated resource trends (in production, use historical data)
func (h *MetricsHandlers) generateResourceTrends(currentUsage ResourceSummary, totalVMs int) ResourceTrends {
	now := time.Now()
	trends := ResourceTrends{
		CPUTrend:     make([]TrendDataPoint, 12),
		MemoryTrend:  make([]TrendDataPoint, 12),
		StorageTrend: make([]TrendDataPoint, 12),
		VMTrend:      make([]TrendDataPoint, 12),
	}

	// Generate last 12 hours of data points (every hour)
	for i := 0; i < 12; i++ {
		timestamp := now.Add(-time.Duration(11-i) * time.Hour)

		// Simulate slight variations in resource usage
		cpuVariation := 1.0 + (float64(i%3)-1)*0.1
		memoryVariation := 1.0 + (float64(i%4)-1.5)*0.08
		storageVariation := 1.0 + (float64(i%2))*0.05
		vmVariation := 1.0 + (float64(i%5)-2)*0.15

		trends.CPUTrend[i] = TrendDataPoint{
			Timestamp: timestamp.Format(time.RFC3339),
			Value:     currentUsage.CPU * cpuVariation,
		}
		trends.MemoryTrend[i] = TrendDataPoint{
			Timestamp: timestamp.Format(time.RFC3339),
			Value:     currentUsage.Memory * memoryVariation,
		}
		trends.StorageTrend[i] = TrendDataPoint{
			Timestamp: timestamp.Format(time.RFC3339),
			Value:     currentUsage.Storage * storageVariation,
		}
		trends.VMTrend[i] = TrendDataPoint{
			Timestamp: timestamp.Format(time.RFC3339),
			Value:     float64(totalVMs) * vmVariation,
		}
	}

	return trends
}

// generateMetricsAlerts generates alerts based on current metrics
func (h *MetricsHandlers) generateMetricsAlerts(orgMetrics []OrganizationMetrics, vdcMetrics []VDCMetrics) MetricsAlertSummary {
	var resourceAlerts []ResourceAlert
	var thresholdBreaches []ThresholdBreach

	// Check organization-level alerts
	for _, org := range orgMetrics {
		if org.UsagePercentages.CPU > 90 {
			resourceAlerts = append(resourceAlerts, ResourceAlert{
				Type:         "cpu",
				Severity:     "critical",
				Target:       org.ID,
				TargetName:   org.Name,
				CurrentUsage: org.UsagePercentages.CPU,
				Threshold:    90,
				Message:      fmt.Sprintf("Organization %s CPU usage is at %.1f%%", org.Name, org.UsagePercentages.CPU),
			})
		} else if org.UsagePercentages.CPU > 80 {
			resourceAlerts = append(resourceAlerts, ResourceAlert{
				Type:         "cpu",
				Severity:     "warning",
				Target:       org.ID,
				TargetName:   org.Name,
				CurrentUsage: org.UsagePercentages.CPU,
				Threshold:    80,
				Message:      fmt.Sprintf("Organization %s CPU usage is at %.1f%%", org.Name, org.UsagePercentages.CPU),
			})
		}

		if org.UsagePercentages.Memory > 90 {
			resourceAlerts = append(resourceAlerts, ResourceAlert{
				Type:         "memory",
				Severity:     "critical",
				Target:       org.ID,
				TargetName:   org.Name,
				CurrentUsage: org.UsagePercentages.Memory,
				Threshold:    90,
				Message:      fmt.Sprintf("Organization %s memory usage is at %.1f%%", org.Name, org.UsagePercentages.Memory),
			})
		}

		if org.UsagePercentages.Storage > 95 {
			resourceAlerts = append(resourceAlerts, ResourceAlert{
				Type:         "storage",
				Severity:     "critical",
				Target:       org.ID,
				TargetName:   org.Name,
				CurrentUsage: org.UsagePercentages.Storage,
				Threshold:    95,
				Message:      fmt.Sprintf("Organization %s storage usage is at %.1f%%", org.Name, org.UsagePercentages.Storage),
			})
		}
	}

	// Check VDC-level alerts
	for _, vdc := range vdcMetrics {
		if vdc.UsagePercentages.CPU > 95 {
			thresholdBreaches = append(thresholdBreaches, ThresholdBreach{
				ResourceType:   "cpu",
				TargetID:       vdc.ID,
				TargetName:     vdc.Name,
				TargetType:     "vdc",
				CurrentValue:   vdc.UsagePercentages.CPU,
				ThresholdValue: 95,
				Severity:       "critical",
				Duration:       "5m", // Would be calculated from historical data
				FirstDetected:  time.Now().Add(-5 * time.Minute),
			})
		}

		if vdc.UsagePercentages.Memory > 95 {
			thresholdBreaches = append(thresholdBreaches, ThresholdBreach{
				ResourceType:   "memory",
				TargetID:       vdc.ID,
				TargetName:     vdc.Name,
				TargetType:     "vdc",
				CurrentValue:   vdc.UsagePercentages.Memory,
				ThresholdValue: 95,
				Severity:       "critical",
				Duration:       "3m",
				FirstDetected:  time.Now().Add(-3 * time.Minute),
			})
		}
	}

	criticalAlerts := 0
	warningAlerts := 0
	for _, alert := range resourceAlerts {
		if alert.Severity == "critical" {
			criticalAlerts++
		} else if alert.Severity == "warning" {
			warningAlerts++
		}
	}

	return MetricsAlertSummary{
		TotalAlerts:       len(resourceAlerts) + len(thresholdBreaches),
		CriticalAlerts:    criticalAlerts,
		WarningAlerts:     warningAlerts,
		ResourceAlerts:    resourceAlerts,
		ThresholdBreaches: thresholdBreaches,
	}
}

// calculatePerformanceStats calculates system performance statistics
func (h *MetricsHandlers) calculatePerformanceStats(startTime time.Time) PerformanceStats {
	collectionLatency := float64(time.Since(startTime).Nanoseconds()) / 1000000.0 // Convert to milliseconds

	// Test database response time
	dbStartTime := time.Now()
	h.storage.Ping()
	dbResponseTime := float64(time.Since(dbStartTime).Nanoseconds()) / 1000000.0

	// Test Kubernetes API latency if available
	var k8sLatency float64
	if h.k8sClient != nil {
		k8sStartTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		namespaces := &corev1.NamespaceList{}
		h.k8sClient.List(ctx, namespaces, client.Limit(1))
		k8sLatency = float64(time.Since(k8sStartTime).Nanoseconds()) / 1000000.0
	}

	return PerformanceStats{
		MetricsCollectionLatency: collectionLatency,
		DatabaseResponseTime:     dbResponseTime,
		KubernetesAPILatency:     k8sLatency,
		ActiveConnections:        1, // Would be tracked by connection pool
		MemoryUsage:              0, // Would be collected from runtime stats
		CPUUsage:                 0, // Would be collected from runtime stats
	}
}

// GetVDCRealTimeMetrics handles GET /metrics/vdc/:id/realtime
func (h *MetricsHandlers) GetVDCRealTimeMetrics(c *gin.Context) {
	vdcID := c.Param("id")
	if vdcID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID is required"})
		return
	}

	// Get VDC from database
	vdc, err := h.storage.GetVDC(vdcID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s: %v", vdcID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Get VMs for this VDC
	vms, err := h.storage.ListVMs(vdc.OrgID)
	if err != nil {
		klog.Errorf("Failed to list VMs for VDC %s: %v", vdcID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC VMs"})
		return
	}

	// Collect VDC metrics
	vdcMetrics, err := h.collectVDCMetrics(vdc, vms)
	if err != nil {
		klog.Errorf("Failed to collect metrics for VDC %s: %v", vdcID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to collect VDC metrics"})
		return
	}

	// Get current status from Kubernetes if available
	if h.k8sClient != nil && vdc.CRNamespace != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		vdcCR := &ovimv1.VirtualDataCenter{}
		err := h.k8sClient.Get(ctx, types.NamespacedName{
			Name:      vdc.CRName,
			Namespace: vdc.CRNamespace,
		}, vdcCR)

		if err != nil && !errors.IsNotFound(err) {
			klog.Warningf("Failed to get VDC CR status for %s: %v", vdcID, err)
		} else if err == nil {
			// Update with current Kubernetes status
			vdcMetrics.Phase = string(vdcCR.Status.Phase)
			if vdcCR.Status.LastMetricsUpdate != nil {
				vdcMetrics.LastMetricsUpdate = vdcCR.Status.LastMetricsUpdate.Format(time.RFC3339)
			}
		}
	}

	c.JSON(http.StatusOK, vdcMetrics)
}

// GetOrganizationRealTimeMetrics handles GET /metrics/organization/:id/realtime
func (h *MetricsHandlers) GetOrganizationRealTimeMetrics(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID is required"})
		return
	}

	// Get organization from database
	org, err := h.storage.GetOrganization(orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	// Collect organization metrics
	orgMetrics, vdcMetrics, err := h.collectOrganizationMetrics(org)
	if err != nil {
		klog.Errorf("Failed to collect metrics for organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to collect organization metrics"})
		return
	}

	response := struct {
		Organization *OrganizationMetrics `json:"organization"`
		VDCs         []VDCMetrics         `json:"vdcs"`
	}{
		Organization: orgMetrics,
		VDCs:         vdcMetrics,
	}

	c.JSON(http.StatusOK, response)
}
