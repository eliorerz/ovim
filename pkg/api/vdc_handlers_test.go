package api

import (
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
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

func TestNewVDCHandlers(t *testing.T) {
	mockStorage := &MockStorage{}
	mockK8sClient := &MockK8sClient{}

	handlers := NewVDCHandlers(mockStorage, mockK8sClient)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, mockK8sClient, handlers.k8sClient)
}

func TestVDCHandlers_List(t *testing.T) {
	tests := []struct {
		name                string
		orgFilter           string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
		expectedVDCs        int
	}{
		{
			name:      "successful list all VDCs (system admin)",
			orgFilter: "",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVDCs", "").Return([]*models.VirtualDataCenter{
					{ID: "vdc1", Name: "VDC 1", OrgID: "org1"},
					{ID: "vdc2", Name: "VDC 2", OrgID: "org2"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedVDCs:   2,
		},
		{
			name:      "list VDCs for specific organization",
			orgFilter: "org1",
			userRole:  models.RoleOrgAdmin,
			userOrgID: "org1",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVDCs", "org1").Return([]*models.VirtualDataCenter{
					{ID: "vdc1", Name: "VDC 1", OrgID: "org1"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedVDCs:   1,
		},
		{
			name:      "storage error",
			orgFilter: "",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVDCs", "").Return([]*models.VirtualDataCenter{}, fmt.Errorf("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedVDCs:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewVDCHandlers(mockStorage, nil)
			url := "/vdcs"
			if tt.orgFilter != "" {
				url += "?org=" + tt.orgFilter
			}
			c, w := setupGinContext("GET", url, nil, "user1", "admin", tt.userRole, tt.userOrgID)

			handlers.List(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, float64(tt.expectedVDCs), response["total"])
			}
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestVDCHandlers_Get(t *testing.T) {
	tests := []struct {
		name                string
		vdcID               string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
	}{
		{
			name:      "successful get",
			vdcID:     "vdc1",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "vdc1").Return(&models.VirtualDataCenter{
					ID:    "vdc1",
					Name:  "Test VDC",
					OrgID: "org1",
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "VDC not found",
			vdcID:     "nonexistent",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "unauthorized access to different org VDC",
			vdcID:     "vdc1",
			userRole:  models.RoleOrgAdmin,
			userOrgID: "org2",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "vdc1").Return(&models.VirtualDataCenter{
					ID:    "vdc1",
					Name:  "Test VDC",
					OrgID: "org1", // Different from user's org
				}, nil)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:      "storage error",
			vdcID:     "vdc1",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "vdc1").Return(nil, fmt.Errorf("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewVDCHandlers(mockStorage, nil)
			c, w := setupGinContext("GET", fmt.Sprintf("/vdcs/%s", tt.vdcID), nil, "user1", "admin", tt.userRole, tt.userOrgID)
			c.Params = []gin.Param{{Key: "id", Value: tt.vdcID}}

			handlers.Get(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestVDCHandlers_Create_CRDOnly(t *testing.T) {
	tests := []struct {
		name                string
		requestBody         models.CreateVDCRequest
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		mockK8sBehavior     func(*MockK8sClient)
		expectedStatus      int
		description         string
	}{
		{
			name: "successful VDC creation (CRD-only)",
			requestBody: models.CreateVDCRequest{
				Name:         "test-vdc",
				DisplayName:  "Test VDC",
				Description:  "Test VDC description",
				OrgID:        "test-org",
				CPUQuota:     4,
				MemoryQuota:  8192,
				StorageQuota: 100,
			},
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				// Verify organization exists
				ms.On("GetOrganization", "test-org").Return(&models.Organization{
					ID:   "test-org",
					Name: "Test Organization",
				}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				mk.On("Create", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenter"), mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			description:    "VDC should be created via CRD",
		},
		{
			name: "invalid request body",
			requestBody: models.CreateVDCRequest{
				Name: "", // Invalid empty name
			},
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// No calls expected
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject invalid request",
		},
		{
			name: "organization not found",
			requestBody: models.CreateVDCRequest{
				Name:         "test-vdc",
				DisplayName:  "Test VDC",
				OrgID:        "nonexistent-org",
				CPUQuota:     4,
				MemoryQuota:  8192,
				StorageQuota: 100,
			},
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "nonexistent-org").Return(nil, storage.ErrNotFound)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// No calls expected
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should fail when organization doesn't exist",
		},
		{
			name: "unauthorized user (not system admin or org admin)",
			requestBody: models.CreateVDCRequest{
				Name:         "test-vdc",
				DisplayName:  "Test VDC",
				Description:  "A test VDC",
				OrgID:        "test-org",
				CPUQuota:     4,
				MemoryQuota:  8,
				StorageQuota: 50,
			},
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// No calls expected
			},
			expectedStatus: http.StatusForbidden,
			description:    "Only system admins and org admins can create VDCs",
		},
		{
			name: "kubernetes client error",
			requestBody: models.CreateVDCRequest{
				Name:         "test-vdc",
				DisplayName:  "Test VDC",
				OrgID:        "test-org",
				CPUQuota:     4,
				MemoryQuota:  8192,
				StorageQuota: 100,
			},
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "test-org").Return(&models.Organization{
					ID:   "test-org",
					Name: "Test Organization",
				}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				mk.On("Create", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenter"), mock.Anything).Return(fmt.Errorf("k8s error"))
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should handle kubernetes errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockK8sClient := &MockK8sClient{}

			tt.mockStorageBehavior(mockStorage)
			tt.mockK8sBehavior(mockK8sClient)

			handlers := NewVDCHandlers(mockStorage, mockK8sClient)
			c, w := setupGinContext("POST", "/vdcs", tt.requestBody, "user1", "admin", tt.userRole, tt.userOrgID)

			handlers.Create(c)

			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)
			mockStorage.AssertExpectations(t)
			mockK8sClient.AssertExpectations(t)
		})
	}
}

func TestVDCHandlers_Update_CRDOnly(t *testing.T) {
	tests := []struct {
		name                string
		vdcID               string
		requestBody         models.UpdateVDCRequest
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		mockK8sBehavior     func(*MockK8sClient)
		expectedStatus      int
	}{
		{
			name:  "successful update",
			vdcID: "test-vdc",
			requestBody: models.UpdateVDCRequest{
				DisplayName: stringPtr("Updated VDC"),
				Description: stringPtr("Updated description"),
				CPUQuota:    intPtr(8),
			},
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				// No storage calls needed - CRD controller handles sync
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs to find the one with matching name
				mk.On("List", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenterList"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					vdcList := args.Get(1).(*ovimv1.VirtualDataCenterList)
					vdcList.Items = []ovimv1.VirtualDataCenter{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-vdc",
								Namespace: "org-test-org",
							},
							Spec: ovimv1.VirtualDataCenterSpec{
								DisplayName:     "Original VDC",
								OrganizationRef: "test-org",
							},
						},
					}
				})
				// Mock update
				mk.On("Update", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenter"), mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "VDC not found",
			vdcID: "nonexistent",
			requestBody: models.UpdateVDCRequest{
				DisplayName: stringPtr("Updated VDC"),
			},
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				// No storage calls
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs returning empty list (VDC not found)
				mk.On("List", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenterList"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					vdcList := args.Get(1).(*ovimv1.VirtualDataCenterList)
					vdcList.Items = []ovimv1.VirtualDataCenter{} // Empty list
				})
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:  "unauthorized access",
			vdcID: "test-vdc",
			requestBody: models.UpdateVDCRequest{
				DisplayName: stringPtr("Updated VDC"),
			},
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs to find the one with matching name (needed before auth check)
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
						},
					}
				})
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockK8sClient := &MockK8sClient{}

			tt.mockStorageBehavior(mockStorage)
			tt.mockK8sBehavior(mockK8sClient)

			handlers := NewVDCHandlers(mockStorage, mockK8sClient)
			c, w := setupGinContext("PUT", fmt.Sprintf("/vdcs/%s", tt.vdcID), tt.requestBody, "user1", "admin", tt.userRole, tt.userOrgID)
			c.Params = []gin.Param{{Key: "id", Value: tt.vdcID}}

			handlers.Update(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
			mockK8sClient.AssertExpectations(t)
		})
	}
}

func TestVDCHandlers_Delete_CRDOnly(t *testing.T) {
	tests := []struct {
		name                string
		vdcID               string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		mockK8sBehavior     func(*MockK8sClient)
		expectedStatus      int
	}{
		{
			name:      "successful deletion",
			vdcID:     "test-vdc",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				// Check for dependent VMs
				ms.On("ListVMs", "").Return([]*models.VirtualMachine{}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs to find the one with matching name
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
						},
					}
				})
				// Mock update for deletion annotations
				mk.On("Update", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenter"), mock.Anything).Return(nil)
				// Mock delete
				mk.On("Delete", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenter"), mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:      "VDC has VMs",
			vdcID:     "test-vdc",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVMs", "").Return([]*models.VirtualMachine{
					{ID: "vm1", Name: "VM 1", VDCID: stringPtr("test-vdc")},
				}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs to find the one with matching name
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
						},
					}
				})
				// No Delete mock needed since it fails at VM check
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:      "VDC not found",
			vdcID:     "nonexistent",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected for VMs check since VDC doesn't exist
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs returning empty list (VDC not found)
				mk.On("List", mock.Anything, mock.AnythingOfType("*v1.VirtualDataCenterList"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					vdcList := args.Get(1).(*ovimv1.VirtualDataCenterList)
					vdcList.Items = []ovimv1.VirtualDataCenter{} // Empty list
				})
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "unauthorized access",
			vdcID:     "test-vdc",
			userRole:  models.RoleOrgUser,
			userOrgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock listing VDCs to find the one with matching name (needed before auth check)
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
						},
					}
				})
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockK8sClient := &MockK8sClient{}

			tt.mockStorageBehavior(mockStorage)
			tt.mockK8sBehavior(mockK8sClient)

			handlers := NewVDCHandlers(mockStorage, mockK8sClient)
			c, w := setupGinContext("DELETE", fmt.Sprintf("/vdcs/%s", tt.vdcID), nil, "user1", "admin", tt.userRole, tt.userOrgID)
			c.Params = []gin.Param{{Key: "id", Value: tt.vdcID}}

			handlers.Delete(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
			mockK8sClient.AssertExpectations(t)
		})
	}
}

func TestVDCHandlers_ListUserVDCs(t *testing.T) {
	tests := []struct {
		name                string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
	}{
		{
			name:      "successful list user VDCs",
			userOrgID: "user-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVDCs", "user-org").Return([]*models.VirtualDataCenter{
					{ID: "vdc1", Name: "User VDC 1", OrgID: "user-org"},
					{ID: "vdc2", Name: "User VDC 2", OrgID: "user-org"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "no organization context",
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "storage error",
			userOrgID: "user-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVDCs", "user-org").Return([]*models.VirtualDataCenter{}, fmt.Errorf("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewVDCHandlers(mockStorage, nil)
			c, w := setupGinContext("GET", "/user/vdcs", nil, "user1", "user", models.RoleOrgUser, tt.userOrgID)

			handlers.ListUserVDCs(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestVDCHandlers_GetResourceUsage(t *testing.T) {
	tests := []struct {
		name                string
		vdcID               string
		userRole            string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
		expectedCPUUsed     int
		expectedCPUQuota    int
		expectedVMCount     int
	}{
		{
			name:      "successful get resource usage with VMs",
			vdcID:     "test-vdc",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:           "test-vdc",
					Name:         "Test VDC",
					OrgID:        "test-org",
					CPUQuota:     20,
					MemoryQuota:  64,
					StorageQuota: 200,
				}, nil)
				ms.On("ListVMs", "test-org").Return([]*models.VirtualMachine{
					{
						ID:       "vm1",
						Name:     "VM 1",
						OrgID:    "test-org",
						VDCID:    stringPtr("test-vdc"),
						Status:   "Running",
						CPU:      8,
						Memory:   "16Gi",
						DiskSize: "100Gi",
					},
					{
						ID:       "vm2",
						Name:     "VM 2",
						OrgID:    "test-org",
						VDCID:    stringPtr("test-vdc"),
						Status:   "Running",
						CPU:      4,
						Memory:   "8Gi",
						DiskSize: "50Gi",
					},
					{
						ID:       "vm3",
						Name:     "VM 3",
						OrgID:    "test-org",
						VDCID:    stringPtr("other-vdc"), // Different VDC
						Status:   "Running",
						CPU:      2,
						Memory:   "4Gi",
						DiskSize: "25Gi",
					},
				}, nil)
			},
			expectedStatus:   http.StatusOK,
			expectedCPUUsed:  12, // Only vm1(8) + vm2(4) = 12, vm3 is in different VDC
			expectedCPUQuota: 20,
			expectedVMCount:  2, // Only 2 VMs in this VDC
		},
		{
			name:      "successful get resource usage empty VDC",
			vdcID:     "empty-vdc",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "empty-vdc").Return(&models.VirtualDataCenter{
					ID:           "empty-vdc",
					Name:         "Empty VDC",
					OrgID:        "test-org",
					CPUQuota:     10,
					MemoryQuota:  32,
					StorageQuota: 100,
				}, nil)
				ms.On("ListVMs", "test-org").Return([]*models.VirtualMachine{}, nil)
			},
			expectedStatus:   http.StatusOK,
			expectedCPUUsed:  0,
			expectedCPUQuota: 10,
			expectedVMCount:  0,
		},
		{
			name:      "VDC not found",
			vdcID:     "nonexistent",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "VM list error",
			vdcID:     "test-vdc",
			userRole:  models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:    "test-vdc",
					Name:  "Test VDC",
					OrgID: "test-org",
				}, nil)
				ms.On("ListVMs", "test-org").Return([]*models.VirtualMachine{}, fmt.Errorf("VM list error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:      "unauthorized access to different org VDC",
			vdcID:     "test-vdc",
			userRole:  models.RoleOrgAdmin,
			userOrgID: "other-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "test-vdc").Return(&models.VirtualDataCenter{
					ID:    "test-vdc",
					Name:  "Test VDC",
					OrgID: "test-org", // Different from user's org
				}, nil)
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewVDCHandlers(mockStorage, nil)
			c, w := setupGinContext("GET", fmt.Sprintf("/vdcs/%s/usage", tt.vdcID), nil, "user1", "admin", tt.userRole, tt.userOrgID)
			c.Params = []gin.Param{{Key: "id", Value: tt.vdcID}}

			handlers.GetResourceUsage(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response models.VDCResourceUsage
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCPUUsed, response.CPUUsed)
				assert.Equal(t, tt.expectedCPUQuota, response.CPUQuota)
				assert.Equal(t, tt.expectedVMCount, response.VMCount)
			}
			mockStorage.AssertExpectations(t)
		})
	}
}
