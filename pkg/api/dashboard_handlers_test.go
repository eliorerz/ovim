package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

func TestDashboardHandlers_GetSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMocks     func(*MockStorage)
		expectedStatus int
		validateResp   func(*testing.T, *DashboardSummary)
	}{
		{
			name: "successful summary",
			setupMocks: func(mockStorage *MockStorage) {
				orgs := []*models.Organization{
					{ID: "org1", Name: "Org 1", IsEnabled: true},
					{ID: "org2", Name: "Org 2", IsEnabled: false},
					{ID: "org3", Name: "Org 3", IsEnabled: true},
				}
				vdcs := []*models.VirtualDataCenter{
					{ID: "vdc1", OrgID: "org1"},
					{ID: "vdc2", OrgID: "org1"},
					{ID: "vdc3", OrgID: "org2"},
				}
				vms := []*models.VirtualMachine{
					{ID: "vm1", Status: "running", OrgID: "org1"},
					{ID: "vm2", Status: "stopped", OrgID: "org1"},
					{ID: "vm3", Status: "running", OrgID: "org2"},
				}
				templates := []*models.Template{
					{ID: "tpl1", Name: "Template 1"},
					{ID: "tpl2", Name: "Template 2"},
				}

				mockStorage.On("ListOrganizations").Return(orgs, nil)
				mockStorage.On("ListVDCs", "org1").Return(vdcs[:2], nil)
				mockStorage.On("ListVDCs", "org2").Return(vdcs[2:], nil)
				mockStorage.On("ListVDCs", "org3").Return([]*models.VirtualDataCenter{}, nil)
				mockStorage.On("ListVMs", "org1").Return(vms[:2], nil)
				mockStorage.On("ListVMs", "org2").Return(vms[2:], nil)
				mockStorage.On("ListVMs", "org3").Return([]*models.VirtualMachine{}, nil)
				mockStorage.On("ListTemplates").Return(templates, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, summary *DashboardSummary) {
				assert.Equal(t, 3, summary.TotalOrganizations)
				assert.Equal(t, 2, summary.EnabledOrganizations)
				assert.Equal(t, 3, summary.TotalVDCs)
				assert.Equal(t, 3, summary.TotalVMs)
				assert.Equal(t, 2, summary.RunningVMs)
				assert.Equal(t, 2, summary.TotalTemplates)
				assert.Equal(t, "healthy", summary.SystemHealth)
				assert.NotEmpty(t, summary.LastUpdated)
			},
		},
		{
			name: "no organizations - warning state",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("ListOrganizations").Return([]*models.Organization{}, nil)
				mockStorage.On("ListTemplates").Return([]*models.Template{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, summary *DashboardSummary) {
				assert.Equal(t, 0, summary.TotalOrganizations)
				assert.Equal(t, 0, summary.EnabledOrganizations)
				assert.Equal(t, "warning", summary.SystemHealth)
			},
		},
		{
			name: "storage error",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("ListOrganizations").Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewDashboardHandlers(mockStorage)

			req := httptest.NewRequest(http.MethodGet, "/dashboard/summary", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetSummary(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK && tt.validateResp != nil {
				var summary DashboardSummary
				err := json.Unmarshal(w.Body.Bytes(), &summary)
				require.NoError(t, err)
				tt.validateResp(t, &summary)
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestDashboardHandlers_GetSystemHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMocks     func(*MockStorage)
		setupK8s       func() client.Client
		expectedStatus int
		validateResp   func(*testing.T, *SystemHealthResponse)
	}{
		{
			name: "healthy system with kubernetes",
			setupMocks: func(mockStorage *MockStorage) {
				orgs := []*models.Organization{
					{ID: "org1", Name: "Org 1", IsEnabled: true},
					{ID: "org2", Name: "Org 2", IsEnabled: true},
				}
				vdcs := []*models.VirtualDataCenter{
					{ID: "vdc1", OrgID: "org1"},
				}
				vms := []*models.VirtualMachine{
					{ID: "vm1", Status: "running", OrgID: "org1"},
				}

				mockStorage.On("Ping").Return(nil)
				mockStorage.On("ListOrganizations").Return(orgs, nil).Maybe()
				mockStorage.On("ListVDCs", "org1").Return(vdcs, nil).Maybe()
				mockStorage.On("ListVDCs", "org2").Return([]*models.VirtualDataCenter{}, nil).Maybe()
				mockStorage.On("ListVMs", "org1").Return(vms, nil).Maybe()
				mockStorage.On("ListVMs", "org2").Return([]*models.VirtualMachine{}, nil).Maybe()
			},
			setupK8s: func() client.Client {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				return fake.NewClientBuilder().WithScheme(scheme).Build()
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, health *SystemHealthResponse) {
				assert.Equal(t, "healthy", health.OverallStatus)
				assert.Contains(t, health.Components, "database")
				assert.Contains(t, health.Components, "kubernetes")
				assert.Contains(t, health.Components, "controller")
				assert.Contains(t, health.Components, "statistics")
				assert.Equal(t, "healthy", health.Components["database"].Status)
				assert.True(t, health.Details.DatabaseConnection)
				assert.NotEmpty(t, health.LastChecked)
			},
		},
		{
			name: "unhealthy database",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("Ping").Return(assert.AnError)
				mockStorage.On("ListOrganizations").Return([]*models.Organization{}, nil).Maybe()
			},
			setupK8s: func() client.Client {
				return nil
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, health *SystemHealthResponse) {
				assert.Equal(t, "unhealthy", health.OverallStatus)
				assert.Equal(t, "unhealthy", health.Components["database"].Status)
				assert.False(t, health.Details.DatabaseConnection)
			},
		},
		{
			name: "storage error during build",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("Ping").Return(assert.AnError)
			},
			setupK8s: func() client.Client {
				return nil
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewDashboardHandlers(mockStorage)

			k8sClient := tt.setupK8s()
			if k8sClient != nil {
				handlers.SetKubernetesClient(k8sClient)
			}

			req := httptest.NewRequest(http.MethodGet, "/dashboard/system-health", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetSystemHealth(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK && tt.validateResp != nil {
				var health SystemHealthResponse
				err := json.Unmarshal(w.Body.Bytes(), &health)
				require.NoError(t, err)
				tt.validateResp(t, &health)
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestDashboardHandlers_checkDatabaseHealth(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*MockStorage)
		expectedStatus string
		expectedMsg    string
	}{
		{
			name: "healthy database",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("Ping").Return(nil)
			},
			expectedStatus: "healthy",
			expectedMsg:    "Database connection is healthy",
		},
		{
			name: "unhealthy database",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("Ping").Return(assert.AnError)
			},
			expectedStatus: "unhealthy",
			expectedMsg:    "Database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewDashboardHandlers(mockStorage)
			health := handlers.checkDatabaseHealth()

			assert.Equal(t, tt.expectedStatus, health.Status)
			assert.Equal(t, tt.expectedMsg, health.Message)

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestDashboardHandlers_checkKubernetesHealth(t *testing.T) {
	tests := []struct {
		name           string
		setupK8s       func() client.Client
		expectedStatus string
		expectedMsg    string
	}{
		{
			name: "no kubernetes client",
			setupK8s: func() client.Client {
				return nil
			},
			expectedStatus: "warning",
			expectedMsg:    "Kubernetes client not configured",
		},
		{
			name: "healthy kubernetes",
			setupK8s: func() client.Client {
				scheme := runtime.NewScheme()
				corev1.AddToScheme(scheme)
				return fake.NewClientBuilder().WithScheme(scheme).Build()
			},
			expectedStatus: "healthy",
			expectedMsg:    "Kubernetes API access is healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			handlers := NewDashboardHandlers(mockStorage)

			k8sClient := tt.setupK8s()
			if k8sClient != nil {
				handlers.SetKubernetesClient(k8sClient)
			}

			health := handlers.checkKubernetesHealth()

			assert.Equal(t, tt.expectedStatus, health.Status)
			assert.Equal(t, tt.expectedMsg, health.Message)
		})
	}
}

func TestDashboardHandlers_checkSystemStatistics(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*MockStorage)
		expectedStatus string
		expectedMsg    string
	}{
		{
			name: "healthy statistics",
			setupMocks: func(mockStorage *MockStorage) {
				orgs := []*models.Organization{
					{ID: "org1", Name: "Org 1", IsEnabled: true},
					{ID: "org2", Name: "Org 2", IsEnabled: true},
				}
				mockStorage.On("ListOrganizations").Return(orgs, nil)
			},
			expectedStatus: "healthy",
			expectedMsg:    "2 organizations configured, 2 enabled",
		},
		{
			name: "no organizations",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("ListOrganizations").Return([]*models.Organization{}, nil)
			},
			expectedStatus: "warning",
			expectedMsg:    "No organizations configured",
		},
		{
			name: "no enabled organizations",
			setupMocks: func(mockStorage *MockStorage) {
				orgs := []*models.Organization{
					{ID: "org1", Name: "Org 1", IsEnabled: false},
					{ID: "org2", Name: "Org 2", IsEnabled: false},
				}
				mockStorage.On("ListOrganizations").Return(orgs, nil)
			},
			expectedStatus: "warning",
			expectedMsg:    "No enabled organizations",
		},
		{
			name: "storage error",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("ListOrganizations").Return(nil, assert.AnError)
			},
			expectedStatus: "warning",
			expectedMsg:    "Unable to retrieve system statistics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewDashboardHandlers(mockStorage)
			health := handlers.checkSystemStatistics()

			assert.Equal(t, tt.expectedStatus, health.Status)
			assert.Equal(t, tt.expectedMsg, health.Message)

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestDashboardHandlers_calculateOverallStatus(t *testing.T) {
	tests := []struct {
		name       string
		components map[string]HealthStatus
		expected   string
	}{
		{
			name: "all healthy",
			components: map[string]HealthStatus{
				"database":   {Status: "healthy"},
				"kubernetes": {Status: "healthy"},
			},
			expected: "healthy",
		},
		{
			name: "has warning",
			components: map[string]HealthStatus{
				"database":   {Status: "healthy"},
				"kubernetes": {Status: "warning"},
			},
			expected: "warning",
		},
		{
			name: "has unhealthy",
			components: map[string]HealthStatus{
				"database":   {Status: "unhealthy"},
				"kubernetes": {Status: "healthy"},
			},
			expected: "unhealthy",
		},
		{
			name: "unhealthy takes precedence over warning",
			components: map[string]HealthStatus{
				"database":   {Status: "unhealthy"},
				"kubernetes": {Status: "warning"},
				"controller": {Status: "healthy"},
			},
			expected: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			handlers := NewDashboardHandlers(mockStorage)

			result := handlers.calculateOverallStatus(tt.components)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewDashboardHandlers(t *testing.T) {
	mockStorage := new(MockStorage)
	handlers := NewDashboardHandlers(mockStorage)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Nil(t, handlers.k8sClient)
}

func TestDashboardHandlers_SetKubernetesClient(t *testing.T) {
	mockStorage := new(MockStorage)
	handlers := NewDashboardHandlers(mockStorage)

	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	handlers.SetKubernetesClient(k8sClient)

	assert.Equal(t, k8sClient, handlers.k8sClient)
}

func TestDashboardSummary_Structure(t *testing.T) {
	summary := DashboardSummary{
		TotalOrganizations:   5,
		EnabledOrganizations: 3,
		TotalVDCs:            10,
		TotalVMs:             20,
		RunningVMs:           15,
		TotalPods:            0,
		RunningPods:          0,
		TotalTemplates:       8,
		SystemHealth:         "healthy",
		LastUpdated:          time.Now().UTC().Format(time.RFC3339),
	}

	// Test JSON serialization
	data, err := json.Marshal(summary)
	require.NoError(t, err)

	var unmarshaled DashboardSummary
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, summary.TotalOrganizations, unmarshaled.TotalOrganizations)
	assert.Equal(t, summary.EnabledOrganizations, unmarshaled.EnabledOrganizations)
	assert.Equal(t, summary.SystemHealth, unmarshaled.SystemHealth)
}

func TestSystemHealthResponse_Structure(t *testing.T) {
	response := SystemHealthResponse{
		OverallStatus: "healthy",
		Components: map[string]HealthStatus{
			"database": {
				Status:  "healthy",
				Message: "Database connection is healthy",
				Details: "",
			},
		},
		LastChecked: time.Now().UTC().Format(time.RFC3339),
		Details: SystemHealthDetails{
			DatabaseConnection:  true,
			KubernetesAccess:    true,
			ControllerRunning:   true,
			ActiveOrganizations: 2,
			ActiveVDCs:          3,
			RunningVMs:          5,
			SystemUptime:        "",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled SystemHealthResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, response.OverallStatus, unmarshaled.OverallStatus)
	assert.Equal(t, response.Details.DatabaseConnection, unmarshaled.Details.DatabaseConnection)
	assert.Equal(t, response.Components["database"].Status, unmarshaled.Components["database"].Status)
}
