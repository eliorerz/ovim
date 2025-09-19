package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

func TestListZones(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Create a minimal server with just router for testing
	router := gin.New()
	server := &Server{
		storage: mockStorage,
		router:  router,
	}

	// Add the specific route we're testing
	router.GET("/api/v1/zones", server.ListZones)

	// Mock zones
	mockZones := []*models.Zone{
		{
			ID:            "zone-1",
			Name:          "Production Zone",
			Status:        models.ZoneStatusAvailable,
			Region:        "us-west-2",
			CloudProvider: "aws",
			NodeCount:     3,
			CPUQuota:      24,
			MemoryQuota:   96,
			StorageQuota:  300,
		},
		{
			ID:            "zone-2",
			Name:          "Development Zone",
			Status:        models.ZoneStatusAvailable,
			Region:        "us-east-1",
			CloudProvider: "azure",
			NodeCount:     2,
			CPUQuota:      16,
			MemoryQuota:   64,
			StorageQuota:  200,
		},
	}

	mockStorage.On("ListZones").Return(mockZones, nil)

	// Test basic listing
	req, _ := http.NewRequest("GET", "/api/v1/zones", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	zones := response["zones"].([]interface{})
	assert.Len(t, zones, 2)
	assert.Equal(t, float64(2), response["total"])

	// Check first zone
	zone1 := zones[0].(map[string]interface{})
	assert.Equal(t, "zone-1", zone1["id"])
	assert.Equal(t, "Production Zone", zone1["name"])
	assert.Equal(t, models.ZoneStatusAvailable, zone1["status"])

	mockStorage.AssertExpectations(t)
}

func TestListZonesWithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Create a minimal server with just router for testing
	router := gin.New()
	server := &Server{
		storage: mockStorage,
		router:  router,
	}

	// Add the specific route we're testing
	router.GET("/api/v1/zones", server.ListZones)

	mockZones := []*models.Zone{
		{
			ID:            "zone-1",
			Name:          "AWS Zone",
			Status:        models.ZoneStatusAvailable,
			Region:        "us-west-2",
			CloudProvider: "aws",
		},
		{
			ID:            "zone-2",
			Name:          "Azure Zone",
			Status:        models.ZoneStatusMaintenance,
			Region:        "us-east-1",
			CloudProvider: "azure",
		},
	}

	mockStorage.On("ListZones").Return(mockZones, nil)

	// Test filtering by provider
	req, _ := http.NewRequest("GET", "/api/v1/zones?provider=aws", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	zones := response["zones"].([]interface{})
	assert.Len(t, zones, 1)

	zone := zones[0].(map[string]interface{})
	assert.Equal(t, "zone-1", zone["id"])
	assert.Equal(t, "aws", zone["cloud_provider"])

	mockStorage.AssertExpectations(t)
}

func TestGetZone(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Create a minimal server with just router for testing
	router := gin.New()
	server := &Server{
		storage: mockStorage,
		router:  router,
	}

	// Add the specific route we're testing
	router.GET("/api/v1/zones/:id", server.GetZone)

	mockZone := &models.Zone{
		ID:              "zone-1",
		Name:            "Test Zone",
		ClusterName:     "test-cluster",
		APIUrl:          "https://api.test.example.com:6443",
		Status:          models.ZoneStatusAvailable,
		Region:          "us-west-2",
		CloudProvider:   "aws",
		NodeCount:       3,
		CPUCapacity:     24,
		MemoryCapacity:  96,
		StorageCapacity: 300,
		CPUQuota:        20,
		MemoryQuota:     80,
		StorageQuota:    240,
		Labels: models.StringMap{
			"environment": "test",
		},
		Annotations: models.StringMap{
			"description": "Test zone",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		LastSync:  time.Now(),
	}

	mockStorage.On("GetZone", "zone-1").Return(mockZone, nil)

	req, _ := http.NewRequest("GET", "/api/v1/zones/zone-1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var zone ZoneResponse
	err := json.Unmarshal(resp.Body.Bytes(), &zone)
	require.NoError(t, err)

	assert.Equal(t, "zone-1", zone.ID)
	assert.Equal(t, "Test Zone", zone.Name)
	assert.Equal(t, "test-cluster", zone.ClusterName)
	assert.Equal(t, models.ZoneStatusAvailable, zone.Status)
	assert.Equal(t, 24, zone.CPUCapacity)
	assert.Equal(t, 20, zone.CPUQuota)
	assert.Equal(t, "test", zone.Labels["environment"])
	assert.Equal(t, "Test zone", zone.Annotations["description"])

	mockStorage.AssertExpectations(t)
}

func TestGetZoneNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Create a minimal server with just router for testing
	router := gin.New()
	server := &Server{
		storage: mockStorage,
		router:  router,
	}

	// Add the specific route we're testing
	router.GET("/api/v1/zones/:id", server.GetZone)

	mockStorage.On("GetZone", "nonexistent").Return((*models.Zone)(nil), assert.AnError)

	req, _ := http.NewRequest("GET", "/api/v1/zones/nonexistent", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Zone not found", response["error"])

	mockStorage.AssertExpectations(t)
}

func TestGetZoneUtilization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Create a minimal server with just router for testing
	router := gin.New()
	server := &Server{
		storage: mockStorage,
		router:  router,
	}

	// Add the specific route we're testing
	router.GET("/api/v1/zones/:id/utilization", server.GetZoneUtilization)

	mockZone := &models.Zone{
		ID:              "zone-1",
		Name:            "Test Zone",
		CPUCapacity:     24,
		MemoryCapacity:  96,
		StorageCapacity: 300,
	}

	mockUtilization := []*models.ZoneUtilization{
		{
			ID:          "zone-1",
			CPUUsed:     12,
			MemoryUsed:  48,
			StorageUsed: 150,
			VDCCount:    2,
		},
	}

	mockStorage.On("GetZone", "zone-1").Return(mockZone, nil)
	mockStorage.On("GetZoneUtilization").Return(mockUtilization, nil)

	req, _ := http.NewRequest("GET", "/api/v1/zones/zone-1/utilization", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var response ZoneUtilizationResponse
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "zone-1", response.ZoneID)
	assert.Equal(t, "Test Zone", response.ZoneName)
	assert.Equal(t, 12, response.CPUUsed)
	assert.Equal(t, 24, response.CPUCapacity)
	assert.Equal(t, 50.0, response.CPUUtilization) // 12/24 * 100
	assert.Equal(t, 2, response.VDCCount)
	assert.Equal(t, 0, response.VMCount) // VM count not tracked in current utilization model

	mockStorage.AssertExpectations(t)
}

func TestListOrganizationZones(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Create a minimal server with just router for testing
	router := gin.New()
	server := &Server{
		storage: mockStorage,
		router:  router,
	}

	// Add the specific route we're testing
	router.GET("/api/v1/organizations/:orgId/zones", server.ListOrganizationZones)

	mockOrg := &models.Organization{
		ID:   "org-1",
		Name: "Test Org",
	}

	mockZoneAccess := []*models.OrganizationZoneAccess{
		{
			OrganizationID: "org-1",
			ZoneID:         "zone-1",
			CPUQuota:       16,
			MemoryQuota:    64,
			StorageQuota:   200,
		},
	}

	mockZone := &models.Zone{
		ID:            "zone-1",
		Name:          "Test Zone",
		Status:        models.ZoneStatusAvailable,
		Region:        "us-west-2",
		CloudProvider: "aws",
		NodeCount:     3,
	}

	mockStorage.On("GetOrganization", "org-1").Return(mockOrg, nil)
	mockStorage.On("GetOrganizationZoneAccess", "org-1").Return(mockZoneAccess, nil)
	mockStorage.On("GetZone", "zone-1").Return(mockZone, nil)

	req, _ := http.NewRequest("GET", "/api/v1/organizations/org-1/zones", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	zones := response["zones"].([]interface{})
	assert.Len(t, zones, 1)

	zone := zones[0].(map[string]interface{})
	assert.Equal(t, "zone-1", zone["id"])
	assert.Equal(t, float64(16), zone["cpu_quota"])
	assert.Equal(t, float64(64), zone["memory_quota"])

	mockStorage.AssertExpectations(t)
}

func TestSetOrganizationZoneQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Create a minimal server with just router for testing
	router := gin.New()
	server := &Server{
		storage: mockStorage,
		router:  router,
	}

	// Add the specific routes we're testing
	router.PUT("/api/v1/organizations/:orgId/zones/:zoneId/quota", server.SetOrganizationZoneQuota)
	router.GET("/api/v1/organizations/:orgId/zones/:zoneId/quota", server.GetOrganizationZoneQuota)

	mockOrg := &models.Organization{
		ID:   "org-1",
		Name: "Test Org",
	}

	mockZone := &models.Zone{
		ID:           "zone-1",
		Name:         "Test Zone",
		CPUQuota:     24,
		MemoryQuota:  96,
		StorageQuota: 300,
	}

	quotaRequest := OrganizationZoneQuotaRequest{
		CPUQuota:     16,
		MemoryQuota:  64,
		StorageQuota: 200,
	}

	// Mock that no existing quota exists first
	mockStorage.On("GetOrganization", "org-1").Return(mockOrg, nil)
	mockStorage.On("GetZone", "zone-1").Return(mockZone, nil)
	mockStorage.On("GetOrganizationZoneQuota", "org-1", "zone-1").Return((*models.OrganizationZoneQuota)(nil), assert.AnError).Once()
	mockStorage.On("CreateOrganizationZoneQuota", &models.OrganizationZoneQuota{
		OrganizationID: "org-1",
		ZoneID:         "zone-1",
		CPUQuota:       16,
		MemoryQuota:    64,
		StorageQuota:   200,
	}).Return(nil)

	// For the response (GetOrganizationZoneQuota is called again after creation)
	newQuota := &models.OrganizationZoneQuota{
		OrganizationID: "org-1",
		ZoneID:         "zone-1",
		CPUQuota:       16,
		MemoryQuota:    64,
		StorageQuota:   200,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	mockUtilization := []*models.ZoneUtilization{
		{
			ID:          "zone-1",
			CPUUsed:     8,
			MemoryUsed:  32,
			StorageUsed: 100,
		},
	}
	mockStorage.On("GetOrganizationZoneQuota", "org-1", "zone-1").Return(newQuota, nil).Once()
	mockStorage.On("GetZoneUtilization").Return(mockUtilization, nil)

	// Prepare request
	body, _ := json.Marshal(quotaRequest)
	req, _ := http.NewRequest("PUT", "/api/v1/organizations/org-1/zones/zone-1/quota", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var response OrganizationZoneQuotaResponse
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "org-1", response.OrganizationID)
	assert.Equal(t, "zone-1", response.ZoneID)
	assert.Equal(t, 16, response.CPUQuota)
	assert.Equal(t, 64, response.MemoryQuota)
	assert.Equal(t, 200, response.StorageQuota)

	mockStorage.AssertExpectations(t)
}

func TestSetOrganizationZoneQuotaExceedsCapacity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock storage
	mockStorage := &MockStorage{}

	// Create a minimal server with just router for testing
	router := gin.New()
	server := &Server{
		storage: mockStorage,
		router:  router,
	}

	// Add the specific route we're testing
	router.PUT("/api/v1/organizations/:orgId/zones/:zoneId/quota", server.SetOrganizationZoneQuota)

	mockOrg := &models.Organization{
		ID:   "org-1",
		Name: "Test Org",
	}

	mockZone := &models.Zone{
		ID:           "zone-1",
		Name:         "Test Zone",
		CPUQuota:     24,
		MemoryQuota:  96,
		StorageQuota: 300,
	}

	quotaRequest := OrganizationZoneQuotaRequest{
		CPUQuota:     30, // Exceeds zone capacity (24)
		MemoryQuota:  64,
		StorageQuota: 200,
	}

	mockStorage.On("GetOrganization", "org-1").Return(mockOrg, nil)
	mockStorage.On("GetZone", "zone-1").Return(mockZone, nil)

	// Prepare request
	body, _ := json.Marshal(quotaRequest)
	req, _ := http.NewRequest("PUT", "/api/v1/organizations/org-1/zones/zone-1/quota", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "CPU quota exceeds zone capacity", response["error"])

	mockStorage.AssertExpectations(t)
}

func TestConvertStringMap(t *testing.T) {
	stringMap := models.StringMap{
		"key1": "value1",
		"key2": "value2",
	}

	result := convertStringMap(stringMap)

	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	assert.Equal(t, expected, result)

	// Test nil input
	nilResult := convertStringMap(nil)
	assert.Nil(t, nilResult)
}
