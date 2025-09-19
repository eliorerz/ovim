package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

func TestMetricsHandlers_GetRealTimeMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Mock organizations
	organizations := []*models.Organization{
		{
			ID:        "org-1",
			Name:      "Test Organization",
			IsEnabled: true,
		},
	}

	// Mock VDCs
	vdcs := []*models.VirtualDataCenter{
		{
			ID:                "vdc-1",
			Name:              "Test VDC",
			OrgID:             "org-1",
			WorkloadNamespace: "vdc-test-org-1",
			Phase:             "Active",
			CPUQuota:          10,
			MemoryQuota:       32,
			StorageQuota:      100,
		},
	}

	// Mock VMs
	vms := []*models.VirtualMachine{
		{
			ID:       "vm-1",
			Name:     "Test VM",
			Status:   "running",
			CPU:      2,
			Memory:   "4Gi",
			DiskSize: "20Gi",
		},
	}

	// Set up mock expectations
	mockStorage.On("ListOrganizations").Return(organizations, nil)
	mockStorage.On("ListVDCs", "org-1").Return(vdcs, nil)
	mockStorage.On("ListVMs", "org-1").Return(vms, nil)
	mockStorage.On("Ping").Return(nil)

	// Create handlers
	handlers := NewMetricsHandlers(mockStorage, nil)

	// Create test router
	router := gin.New()
	router.GET("/metrics/realtime", handlers.GetRealTimeMetrics)

	// Create test request
	req, _ := http.NewRequest("GET", "/metrics/realtime", nil)
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "timestamp")
	assert.Contains(t, w.Body.String(), "system_summary")
	assert.Contains(t, w.Body.String(), "organization_usage")
	assert.Contains(t, w.Body.String(), "vdc_metrics")

	// Verify mock expectations
	mockStorage.AssertExpectations(t)
}

func TestMetricsHandlers_GetVDCRealTimeMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Mock VDC
	vdc := &models.VirtualDataCenter{
		ID:                "vdc-1",
		Name:              "Test VDC",
		OrgID:             "org-1",
		WorkloadNamespace: "vdc-test-org-1",
		Phase:             "Active",
		CPUQuota:          10,
		MemoryQuota:       32,
		StorageQuota:      100,
	}

	// Mock VMs
	vms := []*models.VirtualMachine{
		{
			ID:       "vm-1",
			Name:     "Test VM",
			Status:   "running",
			CPU:      2,
			Memory:   "4Gi",
			DiskSize: "20Gi",
		},
	}

	// Set up mock expectations
	mockStorage.On("GetVDC", "vdc-1").Return(vdc, nil)
	mockStorage.On("ListVMs", "org-1").Return(vms, nil)

	// Create handlers
	handlers := NewMetricsHandlers(mockStorage, nil)

	// Create test router
	router := gin.New()
	router.GET("/metrics/vdc/:id/realtime", handlers.GetVDCRealTimeMetrics)

	// Create test request
	req, _ := http.NewRequest("GET", "/metrics/vdc/vdc-1/realtime", nil)
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "vdc-1")
	assert.Contains(t, w.Body.String(), "Test VDC")
	assert.Contains(t, w.Body.String(), "resource_usage")
	assert.Contains(t, w.Body.String(), "resource_quota")

	// Verify mock expectations
	mockStorage.AssertExpectations(t)
}

func TestMetricsHandlers_GetOrganizationRealTimeMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Mock organization
	org := &models.Organization{
		ID:        "org-1",
		Name:      "Test Organization",
		IsEnabled: true,
	}

	// Mock VDCs
	vdcs := []*models.VirtualDataCenter{
		{
			ID:                "vdc-1",
			Name:              "Test VDC",
			OrgID:             "org-1",
			WorkloadNamespace: "vdc-test-org-1",
			Phase:             "Active",
			CPUQuota:          10,
			MemoryQuota:       32,
			StorageQuota:      100,
		},
	}

	// Mock VMs
	vms := []*models.VirtualMachine{
		{
			ID:       "vm-1",
			Name:     "Test VM",
			Status:   "running",
			CPU:      2,
			Memory:   "4Gi",
			DiskSize: "20Gi",
		},
	}

	// Set up mock expectations
	mockStorage.On("GetOrganization", "org-1").Return(org, nil)
	mockStorage.On("ListVDCs", "org-1").Return(vdcs, nil)
	mockStorage.On("ListVMs", "org-1").Return(vms, nil)

	// Create handlers
	handlers := NewMetricsHandlers(mockStorage, nil)

	// Create test router
	router := gin.New()
	router.GET("/metrics/organization/:id/realtime", handlers.GetOrganizationRealTimeMetrics)

	// Create test request
	req, _ := http.NewRequest("GET", "/metrics/organization/org-1/realtime", nil)
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "org-1")
	assert.Contains(t, w.Body.String(), "Test Organization")
	assert.Contains(t, w.Body.String(), "organization")
	assert.Contains(t, w.Body.String(), "vdcs")

	// Verify mock expectations
	mockStorage.AssertExpectations(t)
}

func TestMetricsHandlers_CollectKubernetesMetrics(t *testing.T) {
	// Create mock storage
	mockStorage := &MockStorage{}

	// Create handlers with nil k8s client (should handle gracefully)
	handlers := NewMetricsHandlers(mockStorage, nil)

	// Test with nil client
	metrics, err := handlers.collectKubernetesMetrics("test-namespace")
	assert.Error(t, err)
	assert.Nil(t, metrics)
}

func TestMetricsHandlers_CalculateSystemHealth(t *testing.T) {
	// Create mock storage
	mockStorage := &MockStorage{}
	handlers := NewMetricsHandlers(mockStorage, nil)

	// Test with no organizations (should return warning)
	health := handlers.calculateSystemHealth([]*models.Organization{}, []VDCMetrics{})
	assert.Equal(t, "warning", health)

	// Test with organizations but none enabled
	orgs := []*models.Organization{
		{ID: "org-1", Name: "Test Org", IsEnabled: false},
	}
	health = handlers.calculateSystemHealth(orgs, []VDCMetrics{})
	assert.Equal(t, "warning", health)

	// Test with enabled organizations and normal resource usage
	orgs = []*models.Organization{
		{ID: "org-1", Name: "Test Org", IsEnabled: true},
	}
	vdcMetrics := []VDCMetrics{
		{
			ID:   "vdc-1",
			Name: "Test VDC",
			UsagePercentages: ResourceSummary{
				CPU:     50.0,
				Memory:  60.0,
				Storage: 70.0,
			},
		},
	}
	health = handlers.calculateSystemHealth(orgs, vdcMetrics)
	assert.Equal(t, "healthy", health)

	// Test with high resource usage (should return warning)
	vdcMetrics[0].UsagePercentages.CPU = 95.0
	health = handlers.calculateSystemHealth(orgs, vdcMetrics)
	assert.Equal(t, "warning", health)
}

func TestMetricsHandlers_GenerateResourceTrends(t *testing.T) {
	// Create mock storage
	mockStorage := &MockStorage{}
	handlers := NewMetricsHandlers(mockStorage, nil)

	// Test resource trends generation
	currentUsage := ResourceSummary{
		CPU:     10.5,
		Memory:  32.0,
		Storage: 100.0,
	}
	totalVMs := 5

	trends := handlers.generateResourceTrends(currentUsage, totalVMs)

	// Verify trends structure
	assert.Len(t, trends.CPUTrend, 12)
	assert.Len(t, trends.MemoryTrend, 12)
	assert.Len(t, trends.StorageTrend, 12)
	assert.Len(t, trends.VMTrend, 12)

	// Verify trend data points have timestamps and values
	for i := 0; i < 12; i++ {
		assert.NotEmpty(t, trends.CPUTrend[i].Timestamp)
		assert.Greater(t, trends.CPUTrend[i].Value, 0.0)

		assert.NotEmpty(t, trends.MemoryTrend[i].Timestamp)
		assert.Greater(t, trends.MemoryTrend[i].Value, 0.0)

		assert.NotEmpty(t, trends.StorageTrend[i].Timestamp)
		assert.Greater(t, trends.StorageTrend[i].Value, 0.0)

		assert.NotEmpty(t, trends.VMTrend[i].Timestamp)
		assert.Greater(t, trends.VMTrend[i].Value, 0.0)
	}
}

func TestMetricsHandlers_GenerateMetricsAlerts(t *testing.T) {
	// Create mock storage
	mockStorage := &MockStorage{}
	handlers := NewMetricsHandlers(mockStorage, nil)

	// Test with organization metrics that should trigger alerts
	orgMetrics := []OrganizationMetrics{
		{
			ID:   "org-1",
			Name: "High CPU Org",
			UsagePercentages: ResourceSummary{
				CPU:     95.0, // Should trigger critical alert
				Memory:  70.0,
				Storage: 80.0,
			},
		},
		{
			ID:   "org-2",
			Name: "High Memory Org",
			UsagePercentages: ResourceSummary{
				CPU:     50.0,
				Memory:  92.0, // Should trigger critical alert
				Storage: 60.0,
			},
		},
	}

	// Test with VDC metrics that should trigger threshold breaches
	vdcMetrics := []VDCMetrics{
		{
			ID:   "vdc-1",
			Name: "High Storage VDC",
			UsagePercentages: ResourceSummary{
				CPU:     85.0,
				Memory:  88.0,
				Storage: 97.0, // Should trigger critical threshold breach
			},
		},
	}

	alertSummary := handlers.generateMetricsAlerts(orgMetrics, vdcMetrics)

	// Verify alert counts
	assert.Greater(t, alertSummary.TotalAlerts, 0)
	assert.Greater(t, alertSummary.CriticalAlerts, 0)
	assert.GreaterOrEqual(t, len(alertSummary.ResourceAlerts), 2) // At least 2 critical resource alerts
	assert.GreaterOrEqual(t, len(alertSummary.ThresholdBreaches), 0)

	// Verify alert details
	foundCPUAlert := false
	foundMemoryAlert := false
	for _, alert := range alertSummary.ResourceAlerts {
		if alert.Type == "cpu" && alert.Severity == "critical" {
			foundCPUAlert = true
			assert.Equal(t, "org-1", alert.Target)
			assert.Greater(t, alert.CurrentUsage, 90.0)
		}
		if alert.Type == "memory" && alert.Severity == "critical" {
			foundMemoryAlert = true
			assert.Equal(t, "org-2", alert.Target)
			assert.Greater(t, alert.CurrentUsage, 90.0)
		}
	}
	assert.True(t, foundCPUAlert, "Should find CPU critical alert")
	assert.True(t, foundMemoryAlert, "Should find memory critical alert")
}

func TestMetricsHandlers_CalculatePerformanceStats(t *testing.T) {
	// Create mock storage
	mockStorage := &MockStorage{}
	mockStorage.On("Ping").Return(nil)

	handlers := NewMetricsHandlers(mockStorage, nil)

	// Test performance stats calculation
	startTime := time.Now().Add(-100 * time.Millisecond) // Simulate 100ms elapsed
	perfStats := handlers.calculatePerformanceStats(startTime)

	// Verify performance stats
	assert.Greater(t, perfStats.MetricsCollectionLatency, 0.0)
	assert.GreaterOrEqual(t, perfStats.DatabaseResponseTime, 0.0)
	assert.GreaterOrEqual(t, perfStats.KubernetesAPILatency, 0.0) // Should be 0 with nil client
	assert.GreaterOrEqual(t, perfStats.ActiveConnections, 0)

	// Verify mock expectations
	mockStorage.AssertExpectations(t)
}

// Benchmark tests for performance
func BenchmarkGetRealTimeMetrics(b *testing.B) {
	gin.SetMode(gin.TestMode)

	// Create mock storage with minimal setup
	mockStorage := &MockStorage{}
	mockStorage.On("ListOrganizations").Return([]*models.Organization{}, nil)

	handlers := NewMetricsHandlers(mockStorage, nil)

	router := gin.New()
	router.GET("/metrics/realtime", handlers.GetRealTimeMetrics)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/metrics/realtime", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
