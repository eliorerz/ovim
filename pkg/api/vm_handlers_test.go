package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
)

// MockVMProvisioner is a mock implementation of kubevirt.VMProvisioner
type MockVMProvisioner struct {
	mock.Mock
}

func (m *MockVMProvisioner) CreateVM(ctx context.Context, vm *models.VirtualMachine, vdc *models.VirtualDataCenter, template *models.Template) error {
	args := m.Called(ctx, vm, vdc, template)
	return args.Error(0)
}

func (m *MockVMProvisioner) DeleteVM(ctx context.Context, vmID, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) GetVMStatus(ctx context.Context, vmID, namespace string) (*kubevirt.VMStatus, error) {
	args := m.Called(ctx, vmID, namespace)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kubevirt.VMStatus), args.Error(1)
}

func (m *MockVMProvisioner) StartVM(ctx context.Context, vmID, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) StopVM(ctx context.Context, vmID, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) RestartVM(ctx context.Context, vmID, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) CheckConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockVMProvisioner) GetVMIPAddress(ctx context.Context, vmID, namespace string) (string, error) {
	args := m.Called(ctx, vmID, namespace)
	return args.String(0), args.Error(1)
}

func TestVMHandlers_Create_WithLimitRangeValidation(t *testing.T) {
	tests := []struct {
		name                    string
		requestBody             models.CreateVMRequest
		userID                  string
		username                string
		role                    string
		userOrgID               string
		mockStorageBehavior     func(*MockStorage)
		mockOpenShiftBehavior   func(*MockOpenShiftClient)
		mockProvisionerBehavior func(*MockVMProvisioner)
		expectedStatus          int
		expectLimitRangeCheck   bool
		description             string
	}{
		{
			name: "VM creation within LimitRange bounds",
			requestBody: models.CreateVMRequest{
				Name:       "Test VM Within Limits",
				TemplateID: "template-123",
				CPU:        4,     // Within limits (1-8)
				Memory:     "8Gi", // Within limits (1-16GB)
			},
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				template := &models.Template{
					ID:       "template-123",
					Name:     "Test Template",
					CPU:      2,
					Memory:   "4Gi",
					DiskSize: "20Gi",
				}
				ms.On("GetTemplate", "template-123").Return(template, nil)

				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)

				vdcs := []*models.VirtualDataCenter{
					{
						ID:        "vdc-123",
						Name:      "Test VDC",
						OrgID:     "org-123",
						Namespace: "org-test-vdc",
					},
				}
				ms.On("ListVDCs", "org-123").Return(vdcs, nil)
				ms.On("CreateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				limitRange := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    1,
					MaxCPU:    8,
					MinMemory: 1,
					MaxMemory: 16,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRange, nil)
			},
			mockProvisionerBehavior: func(mp *MockVMProvisioner) {
				mp.On("CreateVM", mock.Anything, mock.AnythingOfType("*models.VirtualMachine"), mock.AnythingOfType("*models.VirtualDataCenter"), mock.AnythingOfType("*models.Template")).Return(nil)
			},
			expectedStatus:        http.StatusCreated,
			expectLimitRangeCheck: true,
			description:           "Should create VM when specs are within LimitRange bounds",
		},
		{
			name: "VM creation exceeds CPU maximum limit",
			requestBody: models.CreateVMRequest{
				Name:       "Test VM Exceeds CPU",
				TemplateID: "template-123",
				CPU:        16,    // Exceeds limit (max 8)
				Memory:     "8Gi", // Within limits
			},
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				template := &models.Template{
					ID:       "template-123",
					Name:     "Test Template",
					CPU:      2,
					Memory:   "4Gi",
					DiskSize: "20Gi",
				}
				ms.On("GetTemplate", "template-123").Return(template, nil)

				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)

				vdcs := []*models.VirtualDataCenter{
					{
						ID:        "vdc-123",
						Name:      "Test VDC",
						OrgID:     "org-123",
						Namespace: "org-test-vdc",
					},
				}
				ms.On("ListVDCs", "org-123").Return(vdcs, nil)
				// Should not call CreateVM due to validation failure
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				limitRange := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    1,
					MaxCPU:    8,
					MinMemory: 1,
					MaxMemory: 16,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRange, nil)
			},
			mockProvisionerBehavior: func(mp *MockVMProvisioner) {
				// Should not be called due to validation failure
			},
			expectedStatus:        http.StatusBadRequest,
			expectLimitRangeCheck: true,
			description:           "Should reject VM creation when CPU exceeds maximum limit",
		},
		{
			name: "VM creation below CPU minimum limit",
			requestBody: models.CreateVMRequest{
				Name:       "Test VM Below CPU",
				TemplateID: "template-123",
				CPU:        0,     // Will use template CPU which is 0 (below min 1)
				Memory:     "8Gi", // Within limits
			},
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				template := &models.Template{
					ID:       "template-123",
					Name:     "Test Template",
					CPU:      0, // Template CPU is 0, below minimum of 1
					Memory:   "4Gi",
					DiskSize: "20Gi",
				}
				ms.On("GetTemplate", "template-123").Return(template, nil)

				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)

				vdcs := []*models.VirtualDataCenter{
					{
						ID:        "vdc-123",
						Name:      "Test VDC",
						OrgID:     "org-123",
						Namespace: "org-test-vdc",
					},
				}
				ms.On("ListVDCs", "org-123").Return(vdcs, nil)
				// Should not call CreateVM due to validation failure
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				limitRange := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    1,
					MaxCPU:    8,
					MinMemory: 1,
					MaxMemory: 16,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRange, nil)
			},
			mockProvisionerBehavior: func(mp *MockVMProvisioner) {
				// Should not be called due to validation failure
			},
			expectedStatus:        http.StatusBadRequest,
			expectLimitRangeCheck: true,
			description:           "Should reject VM creation when CPU is below minimum limit",
		},
		{
			name: "VM creation exceeds memory maximum limit",
			requestBody: models.CreateVMRequest{
				Name:       "Test VM Exceeds Memory",
				TemplateID: "template-123",
				CPU:        4,      // Within limits
				Memory:     "32Gi", // Exceeds limit (max 16GB)
			},
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				template := &models.Template{
					ID:       "template-123",
					Name:     "Test Template",
					CPU:      2,
					Memory:   "4Gi",
					DiskSize: "20Gi",
				}
				ms.On("GetTemplate", "template-123").Return(template, nil)

				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)

				vdcs := []*models.VirtualDataCenter{
					{
						ID:        "vdc-123",
						Name:      "Test VDC",
						OrgID:     "org-123",
						Namespace: "org-test-vdc",
					},
				}
				ms.On("ListVDCs", "org-123").Return(vdcs, nil)
				// Should not call CreateVM due to validation failure
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				limitRange := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    1,
					MaxCPU:    8,
					MinMemory: 1,
					MaxMemory: 16,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRange, nil)
			},
			mockProvisionerBehavior: func(mp *MockVMProvisioner) {
				// Should not be called due to validation failure
			},
			expectedStatus:        http.StatusBadRequest,
			expectLimitRangeCheck: true,
			description:           "Should reject VM creation when memory exceeds maximum limit",
		},
		{
			name: "VM creation with no LimitRange (unlimited)",
			requestBody: models.CreateVMRequest{
				Name:       "Test VM No Limits",
				TemplateID: "template-123",
				CPU:        16,     // Would normally exceed limits
				Memory:     "64Gi", // Would normally exceed limits
			},
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				template := &models.Template{
					ID:       "template-123",
					Name:     "Test Template",
					CPU:      2,
					Memory:   "4Gi",
					DiskSize: "20Gi",
				}
				ms.On("GetTemplate", "template-123").Return(template, nil)

				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)

				vdcs := []*models.VirtualDataCenter{
					{
						ID:        "vdc-123",
						Name:      "Test VDC",
						OrgID:     "org-123",
						Namespace: "org-test-vdc",
					},
				}
				ms.On("ListVDCs", "org-123").Return(vdcs, nil)
				ms.On("CreateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				limitRange := &models.LimitRangeInfo{
					Exists:    false, // No LimitRange
					MinCPU:    0,
					MaxCPU:    0,
					MinMemory: 0,
					MaxMemory: 0,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRange, nil)
			},
			mockProvisionerBehavior: func(mp *MockVMProvisioner) {
				mp.On("CreateVM", mock.Anything, mock.AnythingOfType("*models.VirtualMachine"), mock.AnythingOfType("*models.VirtualDataCenter"), mock.AnythingOfType("*models.Template")).Return(nil)
			},
			expectedStatus:        http.StatusCreated,
			expectLimitRangeCheck: true,
			description:           "Should allow VM creation when no LimitRange is configured",
		},
		{
			name: "VM creation without OpenShift client (skip validation)",
			requestBody: models.CreateVMRequest{
				Name:       "Test VM No Client",
				TemplateID: "template-123",
				CPU:        16,     // Would normally exceed limits
				Memory:     "32Gi", // Would normally exceed limits
			},
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				template := &models.Template{
					ID:       "template-123",
					Name:     "Test Template",
					CPU:      2,
					Memory:   "4Gi",
					DiskSize: "20Gi",
				}
				ms.On("GetTemplate", "template-123").Return(template, nil)

				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)

				vdcs := []*models.VirtualDataCenter{
					{
						ID:        "vdc-123",
						Name:      "Test VDC",
						OrgID:     "org-123",
						Namespace: "org-test-vdc",
					},
				}
				ms.On("ListVDCs", "org-123").Return(vdcs, nil)
				ms.On("CreateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockOpenShiftBehavior: nil, // No OpenShift client
			mockProvisionerBehavior: func(mp *MockVMProvisioner) {
				mp.On("CreateVM", mock.Anything, mock.AnythingOfType("*models.VirtualMachine"), mock.AnythingOfType("*models.VirtualDataCenter"), mock.AnythingOfType("*models.Template")).Return(nil)
			},
			expectedStatus:        http.StatusCreated,
			expectLimitRangeCheck: false,
			description:           "Should allow VM creation when OpenShift client is not available",
		},
		{
			name: "VM creation with LimitRange fetch error (allow creation)",
			requestBody: models.CreateVMRequest{
				Name:       "Test VM Fetch Error",
				TemplateID: "template-123",
				CPU:        4,
				Memory:     "8Gi",
			},
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				template := &models.Template{
					ID:       "template-123",
					Name:     "Test Template",
					CPU:      2,
					Memory:   "4Gi",
					DiskSize: "20Gi",
				}
				ms.On("GetTemplate", "template-123").Return(template, nil)

				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)

				vdcs := []*models.VirtualDataCenter{
					{
						ID:        "vdc-123",
						Name:      "Test VDC",
						OrgID:     "org-123",
						Namespace: "org-test-vdc",
					},
				}
				ms.On("ListVDCs", "org-123").Return(vdcs, nil)
				ms.On("CreateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(nil, errors.New("failed to fetch LimitRange"))
			},
			mockProvisionerBehavior: func(mp *MockVMProvisioner) {
				mp.On("CreateVM", mock.Anything, mock.AnythingOfType("*models.VirtualMachine"), mock.AnythingOfType("*models.VirtualDataCenter"), mock.AnythingOfType("*models.Template")).Return(nil)
			},
			expectedStatus:        http.StatusCreated,
			expectLimitRangeCheck: true,
			description:           "Should allow VM creation when LimitRange fetch fails (graceful degradation)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			var mockOpenShiftClient *MockOpenShiftClient
			mockProvisioner := &MockVMProvisioner{}

			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			if tt.mockOpenShiftBehavior != nil {
				mockOpenShiftClient = &MockOpenShiftClient{}
				tt.mockOpenShiftBehavior(mockOpenShiftClient)
			}

			if tt.mockProvisionerBehavior != nil {
				tt.mockProvisionerBehavior(mockProvisioner)
			}

			// Create handlers
			var openshiftClient OpenShiftClient
			if mockOpenShiftClient != nil {
				openshiftClient = mockOpenShiftClient
			}
			handlers := &VMHandlers{
				storage:         mockStorage,
				provisioner:     mockProvisioner,
				openshiftClient: openshiftClient,
			}

			// Setup request
			c, w := setupGinContext("POST", "/vms", tt.requestBody, tt.userID, tt.username, tt.role, tt.userOrgID)

			// Execute
			handlers.Create(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}
			mockProvisioner.AssertExpectations(t)

			// Additional assertions for specific responses
			if tt.expectedStatus == http.StatusCreated {
				var response models.VirtualMachine
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.ID)
				assert.Equal(t, tt.requestBody.Name, response.Name)
				assert.Equal(t, tt.userOrgID, response.OrgID)
			} else if tt.expectedStatus == http.StatusBadRequest {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
				if tt.expectLimitRangeCheck {
					errorMsg := response["error"].(string)
					assert.True(t,
						strings.Contains(errorMsg, "limit") ||
							strings.Contains(errorMsg, "CPU") ||
							strings.Contains(errorMsg, "memory"),
						"Error message should mention limits: %s", errorMsg)
				}
			}
		})
	}
}

func TestNewVMHandlers(t *testing.T) {
	mockStorage := &MockStorage{}
	mockProvisioner := &MockVMProvisioner{}
	mockOpenShiftClient := &MockOpenShiftClient{}

	handlers := NewVMHandlers(mockStorage, mockProvisioner, mockOpenShiftClient)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, mockProvisioner, handlers.provisioner)
	assert.Equal(t, mockOpenShiftClient, handlers.openshiftClient)
}
