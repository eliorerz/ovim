package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// MockVMProvisioner is a mock implementation of kubevirt.VMProvisioner interface
type MockVMProvisioner struct {
	mock.Mock
}

func (m *MockVMProvisioner) CreateVM(ctx context.Context, vm *models.VirtualMachine, vdc *models.VirtualDataCenter, template *models.Template) error {
	args := m.Called(ctx, vm, vdc, template)
	return args.Error(0)
}

func (m *MockVMProvisioner) DeleteVM(ctx context.Context, vmID string, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) GetVMStatus(ctx context.Context, vmID string, namespace string) (*kubevirt.VMStatus, error) {
	args := m.Called(ctx, vmID, namespace)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kubevirt.VMStatus), args.Error(1)
}

func (m *MockVMProvisioner) StartVM(ctx context.Context, vmID string, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) StopVM(ctx context.Context, vmID string, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) RestartVM(ctx context.Context, vmID string, namespace string) error {
	args := m.Called(ctx, vmID, namespace)
	return args.Error(0)
}

func (m *MockVMProvisioner) GetVMIPAddress(ctx context.Context, vmID string, namespace string) (string, error) {
	args := m.Called(ctx, vmID, namespace)
	return args.String(0), args.Error(1)
}

func (m *MockVMProvisioner) CheckConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestNewVMHandlers(t *testing.T) {
	mockStorage := &MockStorage{}
	mockProvisioner := &MockVMProvisioner{}
	mockK8sClient := &MockK8sClient{}

	handlers := NewVMHandlers(mockStorage, mockProvisioner, mockK8sClient)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, mockProvisioner, handlers.provisioner)
	assert.Equal(t, mockK8sClient, handlers.k8sClient)
}

func TestVMHandlers_List(t *testing.T) {
	tests := []struct {
		name                string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
		expectedVMs         int
	}{
		{
			name:      "successful list all VMs (system admin)",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVMs", "").Return([]*models.VirtualMachine{
					{ID: "vm1", Name: "VM 1", OwnerID: "user1"},
					{ID: "vm2", Name: "VM 2", OwnerID: "user2"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedVMs:    2,
		},
		{
			name:      "list VMs for org admin",
			userRole:  models.RoleOrgAdmin,
			userOrgID: "org1",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVMs", "org1").Return([]*models.VirtualMachine{
					{ID: "vm1", Name: "VM 1", OwnerID: "user1"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedVMs:    1,
		},
		{
			name:      "list VMs for org user (own VMs only)",
			userRole:  models.RoleOrgUser,
			userOrgID: "org1",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVMs", "org1").Return([]*models.VirtualMachine{
					{ID: "vm1", Name: "VM 1", OwnerID: "user1"},
					{ID: "vm2", Name: "VM 2", OwnerID: "other-user"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedVMs:    1, // Only user's own VM
		},
		{
			name:      "user not associated with organization",
			userRole:  models.RoleOrgUser,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			expectedStatus: http.StatusForbidden,
			expectedVMs:    0,
		},
		{
			name:      "storage error",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVMs", "").Return([]*models.VirtualMachine{}, fmt.Errorf("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedVMs:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewVMHandlers(mockStorage, nil, nil)
			c, w := setupGinContext("GET", "/vms", nil, "user1", "user", tt.userRole, tt.userOrgID)

			handlers.List(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, float64(tt.expectedVMs), response["total"])
			}
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestVMHandlers_Create(t *testing.T) {
	tests := []struct {
		name                string
		requestBody         models.CreateVMRequest
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		mockK8sBehavior     func(*MockK8sClient)
		mockProvBehavior    func(*MockVMProvisioner)
		expectedStatus      int
	}{
		{
			name: "successful VM creation",
			requestBody: models.CreateVMRequest{
				Name:       "test-vm",
				TemplateID: "test-template",
				CPU:        2,
				Memory:     "4Gi",
			},
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetTemplate", "test-template").Return(&models.Template{
					ID:   "test-template",
					Name: "Test Template",
				}, nil)
				ms.On("CreateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs in organization namespace
				mk.On("List", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenterList"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					vdcList := args.Get(1).(*ovimv1.VirtualDataCenterList)
					vdcList.Items = []ovimv1.VirtualDataCenter{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-vdc",
								Namespace: "org-test-org",
							},
							Spec: ovimv1.VirtualDataCenterSpec{
								OrganizationRef: "test-org",
							},
							Status: ovimv1.VirtualDataCenterStatus{
								Phase:     ovimv1.VirtualDataCenterPhaseActive,
								Namespace: "vdc-test-org-test-vdc",
							},
						},
					}
				})
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				mp.On("CreateVM", mock.Anything, mock.AnythingOfType("*models.VirtualMachine"), mock.AnythingOfType("*models.VirtualDataCenter"), mock.AnythingOfType("*models.Template")).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "invalid request body",
			requestBody: models.CreateVMRequest{
				Name: "", // Invalid empty name
			},
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// No calls expected
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				// No calls expected
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "VDC not found",
			requestBody: models.CreateVMRequest{
				Name:       "test-vm",
				TemplateID: "test-template",
				CPU:        2,
				Memory:     "4Gi",
			},
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetTemplate", "test-template").Return(&models.Template{
					ID:   "test-template",
					Name: "Test Template",
				}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs in organization namespace returning empty list
				mk.On("List", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenterList"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					vdcList := args.Get(1).(*ovimv1.VirtualDataCenterList)
					vdcList.Items = []ovimv1.VirtualDataCenter{} // Empty list
				})
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				// No calls expected
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "no VDC available in organization",
			requestBody: models.CreateVMRequest{
				Name:       "test-vm",
				TemplateID: "test-template",
				CPU:        2,
				Memory:     "4Gi",
			},
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetTemplate", "test-template").Return(&models.Template{
					ID:   "test-template",
					Name: "Test Template",
				}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs in organization namespace returning empty list (no VDCs)
				mk.On("List", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenterList"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					vdcList := args.Get(1).(*ovimv1.VirtualDataCenterList)
					vdcList.Items = []ovimv1.VirtualDataCenter{} // Empty list
				})
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				// No calls expected
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockK8sClient := &MockK8sClient{}
			mockProvisioner := &MockVMProvisioner{}

			tt.mockStorageBehavior(mockStorage)
			tt.mockK8sBehavior(mockK8sClient)
			tt.mockProvBehavior(mockProvisioner)

			handlers := NewVMHandlers(mockStorage, mockProvisioner, mockK8sClient)
			c, w := setupGinContext("POST", "/vms", tt.requestBody, "user1", "user", tt.userRole, tt.userOrgID)

			handlers.Create(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
			mockK8sClient.AssertExpectations(t)
			mockProvisioner.AssertExpectations(t)
		})
	}
}

func TestVMHandlers_Get(t *testing.T) {
	tests := []struct {
		name                string
		vmID                string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
	}{
		{
			name:      "successful get VM",
			vmID:      "vm1",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:     "vm1",
					Name:   "Test VM",
					Status: "running",
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "VM not found",
			vmID:      "nonexistent",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "unauthorized access to different user's VM",
			vmID:      "vm1",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OwnerID: "other-user", // Different owner
				}, nil)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:      "storage error",
			vmID:      "vm1",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(nil, fmt.Errorf("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewVMHandlers(mockStorage, nil, nil)
			c, w := setupGinContext("GET", fmt.Sprintf("/vms/%s", tt.vmID), nil, "user1", "user", tt.userRole, tt.userOrgID)
			c.Params = []gin.Param{{Key: "id", Value: tt.vmID}}

			handlers.Get(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestVMHandlers_GetStatus(t *testing.T) {
	tests := []struct {
		name                string
		vmID                string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		mockProvBehavior    func(*MockVMProvisioner)
		expectedStatus      int
	}{
		{
			name:      "successful get VM status",
			vmID:      "vm1",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OrgID:   "test-org",
					VDCID:   stringPtr("test-vdc"),
					OwnerID: "user1", // Same as request user
				}, nil)
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:                "test-vdc",
					WorkloadNamespace: "vdc-test-org-test-vdc",
				}, nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				mp.On("GetVMStatus", mock.Anything, "vm1", "vdc-test-org-test-vdc").Return(&kubevirt.VMStatus{
					Phase:     "Running",
					IPAddress: "10.244.0.1",
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "VM not found",
			vmID:      "nonexistent",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				// No calls expected
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "provisioner error",
			vmID:      "vm1",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OrgID:   "test-org",
					VDCID:   stringPtr("test-vdc"),
					OwnerID: "user1",
				}, nil)
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:                "test-vdc",
					WorkloadNamespace: "vdc-test-org-test-vdc",
				}, nil)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				mp.On("GetVMStatus", mock.Anything, "vm1", "vdc-test-org-test-vdc").Return(nil, fmt.Errorf("kubevirt error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockProvisioner := &MockVMProvisioner{}

			tt.mockStorageBehavior(mockStorage)
			tt.mockProvBehavior(mockProvisioner)

			handlers := NewVMHandlers(mockStorage, mockProvisioner, nil)
			c, w := setupGinContext("GET", fmt.Sprintf("/vms/%s/status", tt.vmID), nil, "user1", "user", tt.userRole, tt.userOrgID)
			c.Params = []gin.Param{{Key: "id", Value: tt.vmID}}

			handlers.GetStatus(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
			mockProvisioner.AssertExpectations(t)
		})
	}
}

func TestVMHandlers_UpdatePower(t *testing.T) {
	tests := []struct {
		name                string
		vmID                string
		action              string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		mockProvBehavior    func(*MockVMProvisioner)
		expectedStatus      int
	}{
		{
			name:      "successful start VM",
			vmID:      "vm1",
			action:    "start",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OrgID:   "test-org",
					VDCID:   stringPtr("test-vdc"),
					OwnerID: "user1",
				}, nil)
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:                "test-vdc",
					WorkloadNamespace: "vdc-test-org-test-vdc",
				}, nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				mp.On("StartVM", mock.Anything, "vm1", "vdc-test-org-test-vdc").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "successful stop VM",
			vmID:      "vm1",
			action:    "stop",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OrgID:   "test-org",
					VDCID:   stringPtr("test-vdc"),
					OwnerID: "user1",
				}, nil)
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:                "test-vdc",
					WorkloadNamespace: "vdc-test-org-test-vdc",
				}, nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				mp.On("StopVM", mock.Anything, "vm1", "vdc-test-org-test-vdc").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "successful restart VM",
			vmID:      "vm1",
			action:    "restart",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OrgID:   "test-org",
					VDCID:   stringPtr("test-vdc"),
					OwnerID: "user1",
					Status:  models.VMStatusRunning,
				}, nil)
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:                "test-vdc",
					WorkloadNamespace: "vdc-test-org-test-vdc",
				}, nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				mp.On("RestartVM", mock.Anything, "vm1", "vdc-test-org-test-vdc").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "invalid action",
			vmID:      "vm1",
			action:    "invalid",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				// No calls expected
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "VM not found",
			vmID:      "nonexistent",
			action:    "start",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				// No calls expected
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockProvisioner := &MockVMProvisioner{}

			tt.mockStorageBehavior(mockStorage)
			tt.mockProvBehavior(mockProvisioner)

			handlers := NewVMHandlers(mockStorage, mockProvisioner, nil)
			c, w := setupGinContext("PUT", fmt.Sprintf("/vms/%s/power", tt.vmID), gin.H{"action": tt.action}, "user1", "user", tt.userRole, tt.userOrgID)
			c.Params = []gin.Param{{Key: "id", Value: tt.vmID}}

			handlers.UpdatePower(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
			mockProvisioner.AssertExpectations(t)
		})
	}
}

func TestVMHandlers_Delete(t *testing.T) {
	tests := []struct {
		name                string
		vmID                string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		mockProvBehavior    func(*MockVMProvisioner)
		expectedStatus      int
	}{
		{
			name:      "successful delete VM",
			vmID:      "vm1",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OrgID:   "test-org",
					VDCID:   stringPtr("test-vdc"),
					OwnerID: "user1",
				}, nil)
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:                "test-vdc",
					WorkloadNamespace: "vdc-test-org-test-vdc",
				}, nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
				ms.On("DeleteVM", "vm1").Return(nil)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				mp.On("DeleteVM", mock.Anything, "vm1", "vdc-test-org-test-vdc").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "VM not found",
			vmID:      "nonexistent",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				// No calls expected
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "unauthorized access to different user's VM",
			vmID:      "vm1",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OwnerID: "other-user", // Different owner
				}, nil)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				// No calls expected
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:      "provisioner error",
			vmID:      "vm1",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVM", "vm1").Return(&models.VirtualMachine{
					ID:      "vm1",
					Name:    "Test VM",
					OrgID:   "test-org",
					VDCID:   stringPtr("test-vdc"),
					OwnerID: "user1",
				}, nil)
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:                "test-vdc",
					WorkloadNamespace: "vdc-test-org-test-vdc",
				}, nil)
				ms.On("UpdateVM", mock.AnythingOfType("*models.VirtualMachine")).Return(nil)
			},
			mockProvBehavior: func(mp *MockVMProvisioner) {
				mp.On("DeleteVM", mock.Anything, "vm1", "vdc-test-org-test-vdc").Return(fmt.Errorf("kubevirt error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockProvisioner := &MockVMProvisioner{}

			tt.mockStorageBehavior(mockStorage)
			tt.mockProvBehavior(mockProvisioner)

			handlers := NewVMHandlers(mockStorage, mockProvisioner, nil)
			c, w := setupGinContext("DELETE", fmt.Sprintf("/vms/%s", tt.vmID), nil, "user1", "user", tt.userRole, tt.userOrgID)
			c.Params = []gin.Param{{Key: "id", Value: tt.vmID}}

			handlers.Delete(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
			mockProvisioner.AssertExpectations(t)
		})
	}
}
