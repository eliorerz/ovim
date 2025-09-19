package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/eliorerz/ovim-updated/pkg/openshift"
)

// MockOpenShiftClient implements openshift.Client interface for testing
type MockOpenShiftClient struct {
	mock.Mock
}

func (m *MockOpenShiftClient) GetTemplates(ctx context.Context) ([]openshift.Template, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]openshift.Template), args.Error(1)
}

func (m *MockOpenShiftClient) GetVMs(ctx context.Context, namespace string) ([]openshift.VirtualMachine, error) {
	args := m.Called(ctx, namespace)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]openshift.VirtualMachine), args.Error(1)
}

func (m *MockOpenShiftClient) DeployVM(ctx context.Context, req openshift.DeployVMRequest) (*openshift.VirtualMachine, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*openshift.VirtualMachine), args.Error(1)
}

func (m *MockOpenShiftClient) GetStatus(ctx context.Context) (*openshift.Status, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*openshift.Status), args.Error(1)
}

func (m *MockOpenShiftClient) IsConnected(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *MockOpenShiftClient) StartVM(ctx context.Context, vmID, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockOpenShiftClient) StopVM(ctx context.Context, vmID, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockOpenShiftClient) RestartVM(ctx context.Context, vmID, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockOpenShiftClient) DeleteVM(ctx context.Context, vmID, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockOpenShiftClient) GetVMConsoleURL(ctx context.Context, vmID, namespace string) (string, error) {
	args := m.Called(ctx, vmID, namespace)
	return args.String(0), args.Error(1)
}

func TestOpenShiftHandlers_GetOpenShiftTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		setupMocks      func(*MockOpenShiftClient, *MockStorage)
		expectedStatus  int
		expectTemplates bool
		validateResp    func(*testing.T, []openshift.Template)
	}{
		{
			name: "successful get templates",
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				templates := []openshift.Template{
					{
						ID:           "rhel8-template-id",
						Name:         "Red Hat Enterprise Linux 8",
						TemplateName: "rhel8-template",
						Description:  "RHEL 8 VM template",
						OSType:       "Red Hat Enterprise Linux",
						OSVersion:    "8",
						CPU:          1,
						Memory:       "2Gi",
						DiskSize:     "20Gi",
						Namespace:    "openshift-virtualization-os-images",
						ImageURL:     "quay.io/containerdisks/rhel:8",
						IconClass:    "fa fa-redhat",
					},
					{
						ID:           "ubuntu-template-id",
						Name:         "Ubuntu 20.04",
						TemplateName: "ubuntu-template",
						Description:  "Ubuntu 20.04 LTS VM template",
						OSType:       "Ubuntu",
						OSVersion:    "20.04",
						CPU:          2,
						Memory:       "4Gi",
						DiskSize:     "40Gi",
						Namespace:    "openshift-virtualization-os-images",
						ImageURL:     "quay.io/containerdisks/ubuntu:20.04",
						IconClass:    "fa fa-ubuntu",
					},
				}
				mockClient.On("GetTemplates", mock.AnythingOfType("*context.valueCtx")).Return(templates, nil)
			},
			expectedStatus:  http.StatusOK,
			expectTemplates: true,
			validateResp: func(t *testing.T, templates []openshift.Template) {
				assert.Len(t, templates, 2)
				assert.Equal(t, "Red Hat Enterprise Linux 8", templates[0].Name)
				assert.Equal(t, "Ubuntu 20.04", templates[1].Name)
				assert.Equal(t, "rhel8-template", templates[0].TemplateName)
				assert.Equal(t, "Red Hat Enterprise Linux", templates[0].OSType)
			},
		},
		{
			name: "client error",
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				mockClient.On("GetTemplates", mock.AnythingOfType("*context.valueCtx")).Return(nil, assert.AnError)
			},
			expectedStatus:  http.StatusInternalServerError,
			expectTemplates: false,
		},
		{
			name: "empty templates list",
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				templates := []openshift.Template{}
				mockClient.On("GetTemplates", mock.AnythingOfType("*context.valueCtx")).Return(templates, nil)
			},
			expectedStatus:  http.StatusOK,
			expectTemplates: true,
			validateResp: func(t *testing.T, templates []openshift.Template) {
				assert.Len(t, templates, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockOpenShiftClient)
			mockStorage := new(MockStorage)
			tt.setupMocks(mockClient, mockStorage)

			handlers := NewOpenShiftHandlers(mockClient, mockStorage)

			req := httptest.NewRequest(http.MethodGet, "/openshift/templates", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetOpenShiftTemplates(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectTemplates {
				var templates []openshift.Template
				err := json.Unmarshal(w.Body.Bytes(), &templates)
				require.NoError(t, err)
				if tt.validateResp != nil {
					tt.validateResp(t, templates)
				}
			}

			mockClient.AssertExpectations(t)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestOpenShiftHandlers_GetOpenShiftVMs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    map[string]string
		userContext    map[string]interface{}
		setupMocks     func(*MockOpenShiftClient, *MockStorage)
		expectedStatus int
		expectVMs      bool
		validateResp   func(*testing.T, []openshift.VirtualMachine)
	}{
		{
			name: "successful get VMs with namespace",
			queryParams: map[string]string{
				"namespace": "test-namespace",
			},
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     "system_admin",
				"org_id":   "",
			},
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				vms := []openshift.VirtualMachine{
					{
						ID:        "test-vm-1-id",
						Name:      "test-vm-1",
						Namespace: "test-namespace",
						Status:    "Running",
						Template:  "rhel8-template",
						Created:   "2023-01-01T00:00:00Z",
					},
					{
						ID:        "test-vm-2-id",
						Name:      "test-vm-2",
						Namespace: "test-namespace",
						Status:    "Stopped",
						Template:  "ubuntu-template",
						Created:   "2023-01-01T00:00:00Z",
					},
				}
				mockClient.On("GetVMs", mock.AnythingOfType("*context.valueCtx"), "test-namespace").Return(vms, nil)
			},
			expectedStatus: http.StatusOK,
			expectVMs:      true,
			validateResp: func(t *testing.T, vms []openshift.VirtualMachine) {
				assert.Len(t, vms, 2)
				assert.Equal(t, "test-vm-1", vms[0].Name)
				assert.Equal(t, "Running", vms[0].Status)
				assert.Equal(t, "test-vm-2", vms[1].Name)
				assert.Equal(t, "Stopped", vms[1].Status)
			},
		},
		{
			name:        "get VMs with default namespace",
			queryParams: map[string]string{},
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     "system_admin",
				"org_id":   "",
			},
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				vms := []openshift.VirtualMachine{}
				mockClient.On("GetVMs", mock.AnythingOfType("*context.valueCtx"), "default").Return(vms, nil)
			},
			expectedStatus: http.StatusOK,
			expectVMs:      true,
			validateResp: func(t *testing.T, vms []openshift.VirtualMachine) {
				assert.Len(t, vms, 0)
			},
		},
		{
			name: "client error",
			queryParams: map[string]string{
				"namespace": "test-namespace",
			},
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     "system_admin",
				"org_id":   "",
			},
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				mockClient.On("GetVMs", mock.AnythingOfType("*context.valueCtx"), "test-namespace").Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectVMs:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockOpenShiftClient)
			mockStorage := new(MockStorage)
			tt.setupMocks(mockClient, mockStorage)

			handlers := NewOpenShiftHandlers(mockClient, mockStorage)

			// Build request URL with query parameters
			url := "/openshift/vms"
			if len(tt.queryParams) > 0 {
				url += "?"
				for key, value := range tt.queryParams {
					url += key + "=" + value + "&"
				}
				url = url[:len(url)-1] // Remove trailing &
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Set user context if provided
			if tt.userContext != nil {
				for key, value := range tt.userContext {
					c.Set(key, value)
				}
			}

			handlers.GetOpenShiftVMs(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectVMs {
				var vms []openshift.VirtualMachine
				err := json.Unmarshal(w.Body.Bytes(), &vms)
				require.NoError(t, err)
				if tt.validateResp != nil {
					tt.validateResp(t, vms)
				}
			}

			mockClient.AssertExpectations(t)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestOpenShiftHandlers_GetOpenShiftStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMocks     func(*MockOpenShiftClient, *MockStorage)
		expectedStatus int
		expectStatus   bool
		validateResp   func(*testing.T, *openshift.Status)
	}{
		{
			name: "successful get status",
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				status := &openshift.Status{
					Connected:    true,
					Version:      "4.12.0",
					ClusterName:  "test-cluster",
					NodesReady:   3,
					NodesTotal:   3,
					PodsRunning:  150,
					PodsTotal:    200,
					PVCs:         25,
					Services:     30,
					Deployments:  45,
					StatefulSets: 5,
				}
				mockClient.On("GetStatus", mock.AnythingOfType("*context.valueCtx")).Return(status, nil)
			},
			expectedStatus: http.StatusOK,
			expectStatus:   true,
			validateResp: func(t *testing.T, status *openshift.Status) {
				assert.True(t, status.Connected)
				assert.Equal(t, "4.12.0", status.Version)
				assert.Equal(t, "test-cluster", status.ClusterName)
				assert.Equal(t, 3, status.NodesReady)
				assert.Equal(t, 3, status.NodesTotal)
			},
		},
		{
			name: "client error",
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				mockClient.On("GetStatus", mock.AnythingOfType("*context.valueCtx")).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectStatus:   false,
		},
		{
			name: "disconnected status",
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				status := &openshift.Status{
					Connected:   false,
					Version:     "",
					ClusterName: "",
				}
				mockClient.On("GetStatus", mock.AnythingOfType("*context.valueCtx")).Return(status, nil)
			},
			expectedStatus: http.StatusOK,
			expectStatus:   true,
			validateResp: func(t *testing.T, status *openshift.Status) {
				assert.False(t, status.Connected)
				assert.Empty(t, status.Version)
				assert.Empty(t, status.ClusterName)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockOpenShiftClient)
			mockStorage := new(MockStorage)
			tt.setupMocks(mockClient, mockStorage)

			handlers := NewOpenShiftHandlers(mockClient, mockStorage)

			req := httptest.NewRequest(http.MethodGet, "/openshift/status", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetOpenShiftStatus(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectStatus {
				var status openshift.Status
				err := json.Unmarshal(w.Body.Bytes(), &status)
				require.NoError(t, err)
				if tt.validateResp != nil {
					tt.validateResp(t, &status)
				}
			}

			mockClient.AssertExpectations(t)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestOpenShiftHandlers_DeployVMFromTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type DeployVMRequest struct {
		TemplateName string                 `json:"template_name"`
		Namespace    string                 `json:"namespace"`
		Parameters   map[string]interface{} `json:"parameters"`
	}

	tests := []struct {
		name           string
		request        DeployVMRequest
		userContext    map[string]interface{}
		setupMocks     func(*MockOpenShiftClient, *MockStorage)
		expectedStatus int
		expectVM       bool
		validateResp   func(*testing.T, *openshift.VirtualMachine)
	}{
		{
			name: "successful VM deployment",
			request: DeployVMRequest{
				TemplateName: "rhel8-template",
				Namespace:    "test-namespace",
				Parameters: map[string]interface{}{
					"VM_NAME":   "test-vm",
					"CPU_CORES": "2",
					"MEMORY":    "4Gi",
				},
			},
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     "system_admin",
				"org_id":   "",
			},
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				deployedVM := &openshift.VirtualMachine{
					ID:        "test-vm-id",
					Name:      "test-vm",
					Namespace: "test-namespace",
					Status:    "Provisioning",
					Template:  "rhel8-template",
					Created:   "2023-01-01T00:00:00Z",
				}
				mockClient.On("DeployVM", mock.AnythingOfType("*context.valueCtx"), mock.MatchedBy(func(req openshift.DeployVMRequest) bool {
					return req.TemplateName == "rhel8-template" && req.TargetNamespace == "test-namespace"
				})).Return(deployedVM, nil)
			},
			expectedStatus: http.StatusCreated,
			expectVM:       true,
			validateResp: func(t *testing.T, vm *openshift.VirtualMachine) {
				assert.Equal(t, "test-vm", vm.Name)
				assert.Equal(t, "test-namespace", vm.Namespace)
				assert.Equal(t, "Provisioning", vm.Status)
				assert.Equal(t, "rhel8-template", vm.Template)
				assert.NotEmpty(t, vm.ID)
			},
		},
		{
			name: "invalid request body",
			request: DeployVMRequest{
				// Missing required fields
				Namespace: "test-namespace",
			},
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     "system_admin",
				"org_id":   "",
			},
			setupMocks:     func(*MockOpenShiftClient, *MockStorage) {},
			expectedStatus: http.StatusBadRequest,
			expectVM:       false,
		},
		{
			name: "deployment error",
			request: DeployVMRequest{
				TemplateName: "invalid-template",
				Namespace:    "test-namespace",
				Parameters: map[string]interface{}{
					"VM_NAME": "test-vm",
				},
			},
			userContext: map[string]interface{}{
				"user_id":  "admin1",
				"username": "admin",
				"role":     "system_admin",
				"org_id":   "",
			},
			setupMocks: func(mockClient *MockOpenShiftClient, mockStorage *MockStorage) {
				mockClient.On("DeployVM", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("openshift.DeployVMRequest")).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectVM:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockOpenShiftClient)
			mockStorage := new(MockStorage)
			tt.setupMocks(mockClient, mockStorage)

			handlers := NewOpenShiftHandlers(mockClient, mockStorage)

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/openshift/vms", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Set user context if provided
			if tt.userContext != nil {
				for key, value := range tt.userContext {
					c.Set(key, value)
				}
			}

			handlers.DeployVMFromTemplate(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectVM {
				var vm openshift.VirtualMachine
				err := json.Unmarshal(w.Body.Bytes(), &vm)
				require.NoError(t, err)
				if tt.validateResp != nil {
					tt.validateResp(t, &vm)
				}
			}

			mockClient.AssertExpectations(t)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestNewOpenShiftHandlers(t *testing.T) {
	mockClient := new(MockOpenShiftClient)
	mockStorage := new(MockStorage)

	handlers := NewOpenShiftHandlers(mockClient, mockStorage)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockClient, handlers.client)
	assert.Equal(t, mockStorage, handlers.storage)
}

func TestOpenShiftHandlers_WithNilClient(t *testing.T) {
	mockStorage := new(MockStorage)

	// Test that handlers can be created with nil client
	handlers := NewOpenShiftHandlers(nil, mockStorage)

	assert.NotNil(t, handlers)
	assert.Nil(t, handlers.client)
	assert.Equal(t, mockStorage, handlers.storage)

	// Test that handlers don't panic with nil client (they should return errors)
	req := httptest.NewRequest(http.MethodGet, "/openshift/status", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	assert.NotPanics(t, func() {
		handlers.GetOpenShiftStatus(c)
	})

	// Should return an error status since client is nil
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
