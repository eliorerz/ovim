package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

func TestNewVDCHandlers(t *testing.T) {
	mockStorage := &MockStorage{}

	handlers := NewVDCHandlers(mockStorage, nil)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
}

func TestVDCHandlers_GetResourceUsage(t *testing.T) {
	tests := []struct {
		name                string
		vdcID               string
		userID              string
		username            string
		role                string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
		expectedCPUUsed     *int
		expectedMemoryUsed  *int
		expectedStorageUsed *int
		expectedVMCount     *int
		description         string
	}{
		{
			name:      "Successful resource usage retrieval by system admin",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
					CPUQuota:          20,
					MemoryQuota:       64,
					StorageQuota:      500,
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)

				vms := []*models.VirtualMachine{
					{
						ID:       "vm-1",
						Name:     "Test VM 1",
						VDCID:    stringPtr("vdc-123"),
						Status:   "Running",
						CPU:      4,
						Memory:   "8Gi",
						DiskSize: "50Gi",
					},
					{
						ID:       "vm-2",
						Name:     "Test VM 2",
						VDCID:    stringPtr("vdc-123"),
						Status:   "Stopped",
						CPU:      2,
						Memory:   "4Gi",
						DiskSize: "20Gi",
					},
				}
				ms.On("ListVMs", "org-123").Return(vms, nil)
			},
			expectedStatus:      http.StatusOK,
			expectedCPUUsed:     intPtr(6),
			expectedMemoryUsed:  intPtr(12),
			expectedStorageUsed: intPtr(70),
			expectedVMCount:     intPtr(2),
			description:         "Should retrieve resource usage successfully for system admin",
		},
		{
			name:      "VDC not found",
			vdcID:     "nonexistent-vdc",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "nonexistent-vdc").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			description:    "Should return 404 when VDC is not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tc.mockStorageBehavior(mockStorage)

			handlers := &VDCHandlers{
				storage:   mockStorage,
				k8sClient: nil,
			}

			c, w := setupGinContext("GET", "/vdcs/"+tc.vdcID+"/resources", "", tc.userID, tc.username, tc.role, tc.userOrgID)
			c.Params = gin.Params{{Key: "id", Value: tc.vdcID}}

			handlers.GetResourceUsage(c)

			assert.Equal(t, tc.expectedStatus, w.Code, tc.description)

			if tc.expectedStatus == http.StatusOK {
				var response models.VDCResourceUsage
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err, "Response should be valid JSON")

				if tc.expectedCPUUsed != nil {
					assert.Equal(t, *tc.expectedCPUUsed, response.CPUUsed, "CPU used should match expected value")
				}
				if tc.expectedMemoryUsed != nil {
					assert.Equal(t, *tc.expectedMemoryUsed, response.MemoryUsed, "Memory used should match expected value")
				}
				if tc.expectedStorageUsed != nil {
					assert.Equal(t, *tc.expectedStorageUsed, response.StorageUsed, "Storage used should match expected value")
				}
				if tc.expectedVMCount != nil {
					assert.Equal(t, *tc.expectedVMCount, response.VMCount, "VM count should match expected value")
				}
			}

			mockStorage.AssertExpectations(t)
		})
	}
}
