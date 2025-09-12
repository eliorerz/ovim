package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

func TestVDCHandlers_GetLimitRange(t *testing.T) {
	tests := []struct {
		name                  string
		vdcID                 string
		userID                string
		username              string
		role                  string
		userOrgID             string
		mockStorageBehavior   func(*MockStorage)
		mockOpenShiftBehavior func(*MockOpenShiftClient)
		expectedStatus        int
		expectedExists        *bool
		description           string
	}{
		{
			name:      "Successful LimitRange retrieval by system admin",
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
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				limitRangeInfo := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    1,
					MaxCPU:    8,
					MinMemory: 1,
					MaxMemory: 16,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRangeInfo, nil)
			},
			expectedStatus: http.StatusOK,
			expectedExists: boolPtr(true),
			description:    "Should retrieve LimitRange successfully for system admin",
		},
		{
			name:      "LimitRange retrieval by org admin for own organization",
			vdcID:     "vdc-123",
			userID:    "orgadmin-123",
			username:  "orgadmin",
			role:      models.RoleOrgAdmin,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Same as user's org
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				limitRangeInfo := &models.LimitRangeInfo{
					Exists:    false,
					MinCPU:    0,
					MaxCPU:    0,
					MinMemory: 0,
					MaxMemory: 0,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRangeInfo, nil)
			},
			expectedStatus: http.StatusOK,
			expectedExists: boolPtr(false),
			description:    "Should allow org admin to view LimitRange for VDCs in their organization",
		},
		{
			name:      "Forbidden access by org admin for different organization",
			vdcID:     "vdc-123",
			userID:    "orgadmin-123",
			username:  "orgadmin",
			role:      models.RoleOrgAdmin,
			userOrgID: "org-456", // Different from VDC's org
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Different from user's org
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to permission check
			},
			expectedStatus: http.StatusForbidden,
			description:    "Should forbid org admin from accessing VDCs in different organizations",
		},
		{
			name:      "VDC not found",
			vdcID:     "non-existent",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "non-existent").Return(nil, storage.ErrNotFound)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called since VDC doesn't exist
			},
			expectedStatus: http.StatusNotFound,
			description:    "Should return not found for non-existent VDC",
		},
		{
			name:      "OpenShift client not available",
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
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: nil, // No OpenShift client
			expectedStatus:        http.StatusOK,
			expectedExists:        boolPtr(false),
			description:           "Should return default LimitRange info when OpenShift client not available",
		},
		{
			name:      "OpenShift GetLimitRange fails",
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
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(nil, errors.New("OpenShift error"))
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should handle OpenShift GetLimitRange failures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			var mockOpenShiftClient *MockOpenShiftClient

			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			if tt.mockOpenShiftBehavior != nil {
				mockOpenShiftClient = &MockOpenShiftClient{}
				tt.mockOpenShiftBehavior(mockOpenShiftClient)
			}

			// Create handlers
			var handlers *VDCHandlers
			if mockOpenShiftClient != nil {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request
			c, w := setupGinContext("GET", "/vdcs/"+tt.vdcID+"/limitrange", nil, tt.userID, tt.username, tt.role, tt.userOrgID)
			c.Params = gin.Params{{Key: "id", Value: tt.vdcID}}

			// Execute
			handlers.GetLimitRange(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}

			// Additional assertions for successful responses
			if tt.expectedStatus == http.StatusOK && tt.expectedExists != nil {
				var response models.LimitRangeInfo
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, *tt.expectedExists, response.Exists)
			}
		})
	}
}

func TestVDCHandlers_UpdateLimitRange(t *testing.T) {
	tests := []struct {
		name                  string
		vdcID                 string
		userID                string
		username              string
		role                  string
		userOrgID             string
		requestBody           models.LimitRangeRequest
		mockStorageBehavior   func(*MockStorage)
		mockOpenShiftBehavior func(*MockOpenShiftClient)
		expectedStatus        int
		description           string
	}{
		{
			name:      "Successful LimitRange update by system admin",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			requestBody: models.LimitRangeRequest{
				MinCPU:    1,
				MaxCPU:    8,
				MinMemory: 1,
				MaxMemory: 16,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// First try update (fails), then create (succeeds)
				moc.On("UpdateLimitRange", mock.Anything, "org-test-vdc", 1, 8, 1, 16).Return(errors.New("not found"))
				moc.On("CreateLimitRange", mock.Anything, "org-test-vdc", 1, 8, 1, 16).Return(nil)

				limitRangeInfo := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    1,
					MaxCPU:    8,
					MinMemory: 1,
					MaxMemory: 16,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRangeInfo, nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should create LimitRange when update fails (not found)",
		},
		{
			name:      "Successful LimitRange update (existing)",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			requestBody: models.LimitRangeRequest{
				MinCPU:    2,
				MaxCPU:    16,
				MinMemory: 2,
				MaxMemory: 32,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Update succeeds directly
				moc.On("UpdateLimitRange", mock.Anything, "org-test-vdc", 2, 16, 2, 32).Return(nil)

				limitRangeInfo := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    2,
					MaxCPU:    16,
					MinMemory: 2,
					MaxMemory: 32,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRangeInfo, nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should update existing LimitRange successfully",
		},
		{
			name:      "Update by org admin for own organization",
			vdcID:     "vdc-123",
			userID:    "orgadmin-123",
			username:  "orgadmin",
			role:      models.RoleOrgAdmin,
			userOrgID: "org-123",
			requestBody: models.LimitRangeRequest{
				MinCPU:    1,
				MaxCPU:    4,
				MinMemory: 1,
				MaxMemory: 8,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Same as user's org
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("UpdateLimitRange", mock.Anything, "org-test-vdc", 1, 4, 1, 8).Return(nil)

				limitRangeInfo := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    1,
					MaxCPU:    4,
					MinMemory: 1,
					MaxMemory: 8,
				}
				moc.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRangeInfo, nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should allow org admin to update LimitRange for VDCs in their organization",
		},
		{
			name:      "Forbidden by org user",
			vdcID:     "vdc-123",
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			requestBody: models.LimitRangeRequest{
				MinCPU:    1,
				MaxCPU:    4,
				MinMemory: 1,
				MaxMemory: 8,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				// Should not be called due to permission check
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to permission check
			},
			expectedStatus: http.StatusForbidden,
			description:    "Should forbid org users from updating LimitRange",
		},
		{
			name:      "Invalid request - negative values",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			requestBody: models.LimitRangeRequest{
				MinCPU:    -1, // Invalid
				MaxCPU:    8,
				MinMemory: 1,
				MaxMemory: 16,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to validation failure
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject negative values",
		},
		{
			name:      "Invalid request - min greater than max",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			requestBody: models.LimitRangeRequest{
				MinCPU:    8, // Greater than max
				MaxCPU:    4,
				MinMemory: 1,
				MaxMemory: 16,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to validation failure
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject min values greater than max values",
		},
		{
			name:      "OpenShift client not available",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			requestBody: models.LimitRangeRequest{
				MinCPU:    1,
				MaxCPU:    8,
				MinMemory: 1,
				MaxMemory: 16,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: nil, // No OpenShift client
			expectedStatus:        http.StatusServiceUnavailable,
			description:           "Should return service unavailable when OpenShift client not available",
		},
		{
			name:      "Both update and create fail",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			requestBody: models.LimitRangeRequest{
				MinCPU:    1,
				MaxCPU:    8,
				MinMemory: 1,
				MaxMemory: 16,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Both update and create fail
				moc.On("UpdateLimitRange", mock.Anything, "org-test-vdc", 1, 8, 1, 16).Return(errors.New("update failed"))
				moc.On("CreateLimitRange", mock.Anything, "org-test-vdc", 1, 8, 1, 16).Return(errors.New("create failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should handle both update and create failures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			var mockOpenShiftClient *MockOpenShiftClient

			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			if tt.mockOpenShiftBehavior != nil {
				mockOpenShiftClient = &MockOpenShiftClient{}
				tt.mockOpenShiftBehavior(mockOpenShiftClient)
			}

			// Create handlers
			var handlers *VDCHandlers
			if mockOpenShiftClient != nil {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request
			c, w := setupGinContext("PUT", "/vdcs/"+tt.vdcID+"/limitrange", tt.requestBody, tt.userID, tt.username, tt.role, tt.userOrgID)
			c.Params = gin.Params{{Key: "id", Value: tt.vdcID}}

			// Execute
			handlers.UpdateLimitRange(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}

			// Additional assertions for successful responses
			if tt.expectedStatus == http.StatusOK {
				var response models.LimitRangeInfo
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.True(t, response.Exists)
			}
		})
	}
}

func TestVDCHandlers_DeleteLimitRange(t *testing.T) {
	tests := []struct {
		name                  string
		vdcID                 string
		userID                string
		username              string
		role                  string
		userOrgID             string
		mockStorageBehavior   func(*MockStorage)
		mockOpenShiftBehavior func(*MockOpenShiftClient)
		expectedStatus        int
		description           string
	}{
		{
			name:      "Successful LimitRange deletion by system admin",
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
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteLimitRange", mock.Anything, "org-test-vdc").Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should delete LimitRange successfully for system admin",
		},
		{
			name:      "Deletion by org admin for own organization",
			vdcID:     "vdc-123",
			userID:    "orgadmin-123",
			username:  "orgadmin",
			role:      models.RoleOrgAdmin,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Same as user's org
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteLimitRange", mock.Anything, "org-test-vdc").Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should allow org admin to delete LimitRange for VDCs in their organization",
		},
		{
			name:      "Forbidden by org admin for different organization",
			vdcID:     "vdc-123",
			userID:    "orgadmin-123",
			username:  "orgadmin",
			role:      models.RoleOrgAdmin,
			userOrgID: "org-456", // Different from VDC's org
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Different from user's org
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to permission check
			},
			expectedStatus: http.StatusForbidden,
			description:    "Should forbid org admin from deleting LimitRange for VDCs in different organizations",
		},
		{
			name:      "Forbidden by org user",
			vdcID:     "vdc-123",
			userID:    "user-123",
			username:  "user",
			role:      models.RoleOrgUser,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				// Should not be called due to permission check
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to permission check
			},
			expectedStatus: http.StatusForbidden,
			description:    "Should forbid org users from deleting LimitRange",
		},
		{
			name:      "VDC not found",
			vdcID:     "non-existent",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "non-existent").Return(nil, storage.ErrNotFound)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called since VDC doesn't exist
			},
			expectedStatus: http.StatusNotFound,
			description:    "Should return not found for non-existent VDC",
		},
		{
			name:      "OpenShift client not available",
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
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: nil, // No OpenShift client
			expectedStatus:        http.StatusServiceUnavailable,
			description:           "Should return service unavailable when OpenShift client not available",
		},
		{
			name:      "OpenShift deletion fails",
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
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteLimitRange", mock.Anything, "org-test-vdc").Return(errors.New("deletion failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should handle OpenShift deletion failures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			var mockOpenShiftClient *MockOpenShiftClient

			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			if tt.mockOpenShiftBehavior != nil {
				mockOpenShiftClient = &MockOpenShiftClient{}
				tt.mockOpenShiftBehavior(mockOpenShiftClient)
			}

			// Create handlers
			var handlers *VDCHandlers
			if mockOpenShiftClient != nil {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request
			c, w := setupGinContext("DELETE", "/vdcs/"+tt.vdcID+"/limitrange", nil, tt.userID, tt.username, tt.role, tt.userOrgID)
			c.Params = gin.Params{{Key: "id", Value: tt.vdcID}}

			// Execute
			handlers.DeleteLimitRange(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}

			// Additional assertions for successful responses
			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "LimitRange deleted successfully", response["message"])
			}
		})
	}
}

func TestVDCHandlers_CreateWithLimitRange(t *testing.T) {
	tests := []struct {
		name                  string
		requestBody           models.CreateVDCRequest
		userID                string
		username              string
		role                  string
		userOrgID             string
		mockStorageBehavior   func(*MockStorage)
		mockOpenShiftBehavior func(*MockOpenShiftClient)
		expectedStatus        int
		expectLimitRange      bool
		description           string
	}{
		{
			name: "VDC creation with LimitRange parameters",
			requestBody: models.CreateVDCRequest{
				Name:         "Test VDC with LimitRange",
				DisplayName:  "Test VDC with LimitRange",
				Description:  "VDC with VM resource constraints",
				OrgID:        "org-123",
				CPUQuota:     10,
				MemoryQuota:  32,
				StorageQuota: 100,
				MinCPU:       intPtr(1),
				MaxCPU:       intPtr(8),
				MinMemory:    intPtr(1),
				MaxMemory:    intPtr(16),
			},
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)
				ms.On("CreateVDC", mock.AnythingOfType("*models.VirtualDataCenter")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, mock.AnythingOfType("string"), 10, 32, 100).Return(nil)
				moc.On("CreateLimitRange", mock.Anything, mock.AnythingOfType("string"), 1, 8, 1, 16).Return(nil)
			},
			expectedStatus:   http.StatusCreated,
			expectLimitRange: true,
			description:      "Should create VDC with LimitRange when all parameters are provided",
		},
		{
			name: "VDC creation without LimitRange parameters",
			requestBody: models.CreateVDCRequest{
				Name:         "Test VDC without LimitRange",
				DisplayName:  "Test VDC without LimitRange",
				Description:  "VDC without VM resource constraints",
				OrgID:        "org-123",
				CPUQuota:     10,
				MemoryQuota:  32,
				StorageQuota: 100,
				// No LimitRange parameters
			},
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)
				ms.On("CreateVDC", mock.AnythingOfType("*models.VirtualDataCenter")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, mock.AnythingOfType("string"), 10, 32, 100).Return(nil)
				// Should not call CreateLimitRange
			},
			expectedStatus:   http.StatusCreated,
			expectLimitRange: false,
			description:      "Should create VDC without LimitRange when parameters are not provided",
		},
		{
			name: "VDC creation with partial LimitRange parameters",
			requestBody: models.CreateVDCRequest{
				Name:         "Test VDC with partial LimitRange",
				DisplayName:  "Test VDC with partial LimitRange",
				Description:  "VDC with incomplete VM resource constraints",
				OrgID:        "org-123",
				CPUQuota:     10,
				MemoryQuota:  32,
				StorageQuota: 100,
				MinCPU:       intPtr(1),
				MaxCPU:       intPtr(8),
				// Missing MinMemory and MaxMemory
			},
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)
				ms.On("CreateVDC", mock.AnythingOfType("*models.VirtualDataCenter")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, mock.AnythingOfType("string"), 10, 32, 100).Return(nil)
				// Should not call CreateLimitRange due to incomplete parameters
			},
			expectedStatus:   http.StatusCreated,
			expectLimitRange: false,
			description:      "Should create VDC without LimitRange when parameters are incomplete",
		},
		{
			name: "VDC creation with invalid LimitRange parameters",
			requestBody: models.CreateVDCRequest{
				Name:         "Test VDC with invalid LimitRange",
				DisplayName:  "Test VDC with invalid LimitRange",
				Description:  "VDC with invalid VM resource constraints",
				OrgID:        "org-123",
				CPUQuota:     10,
				MemoryQuota:  32,
				StorageQuota: 100,
				MinCPU:       intPtr(8), // Min greater than max
				MaxCPU:       intPtr(4),
				MinMemory:    intPtr(1),
				MaxMemory:    intPtr(16),
			},
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)
				ms.On("CreateVDC", mock.AnythingOfType("*models.VirtualDataCenter")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, mock.AnythingOfType("string"), 10, 32, 100).Return(nil)
				// Should not call CreateLimitRange due to invalid parameters
			},
			expectedStatus:   http.StatusCreated,
			expectLimitRange: false,
			description:      "Should create VDC without LimitRange when parameters are invalid",
		},
		{
			name: "VDC creation with LimitRange creation failure",
			requestBody: models.CreateVDCRequest{
				Name:         "Test VDC with LimitRange failure",
				DisplayName:  "Test VDC with LimitRange failure",
				Description:  "VDC where LimitRange creation fails",
				OrgID:        "org-123",
				CPUQuota:     10,
				MemoryQuota:  32,
				StorageQuota: 100,
				MinCPU:       intPtr(1),
				MaxCPU:       intPtr(8),
				MinMemory:    intPtr(1),
				MaxMemory:    intPtr(16),
			},
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "org-123",
					Name:      "Test Organization",
					Namespace: "org-test",
				}
				ms.On("GetOrganization", "org-123").Return(org, nil)
				ms.On("CreateVDC", mock.AnythingOfType("*models.VirtualDataCenter")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, mock.AnythingOfType("string"), 10, 32, 100).Return(nil)
				moc.On("CreateLimitRange", mock.Anything, mock.AnythingOfType("string"), 1, 8, 1, 16).Return(errors.New("LimitRange creation failed"))
			},
			expectedStatus:   http.StatusCreated,
			expectLimitRange: false, // Should still succeed VDC creation even if LimitRange fails
			description:      "Should create VDC successfully even if LimitRange creation fails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			var mockOpenShiftClient *MockOpenShiftClient

			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			if tt.mockOpenShiftBehavior != nil {
				mockOpenShiftClient = &MockOpenShiftClient{}
				tt.mockOpenShiftBehavior(mockOpenShiftClient)
			}

			// Create handlers
			var handlers *VDCHandlers
			if mockOpenShiftClient != nil {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request
			c, w := setupGinContext("POST", "/vdcs", tt.requestBody, tt.userID, tt.username, tt.role, tt.userOrgID)

			// Execute
			handlers.Create(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}

			// Additional assertions for successful responses
			if tt.expectedStatus == http.StatusCreated {
				var response models.VirtualDataCenter
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.ID)
				assert.Equal(t, tt.requestBody.Name, response.Name)
			}
		})
	}
}

func TestVDCHandlers_Delete_WithProtection(t *testing.T) {
	tests := []struct {
		name                  string
		vdcID                 string
		userID                string
		username              string
		role                  string
		userOrgID             string
		existingVMs           []*models.VirtualMachine
		mockStorageBehavior   func(*MockStorage)
		mockOpenShiftBehavior func(*MockOpenShiftClient)
		expectedStatus        int
		description           string
	}{
		{
			name:        "Successful VDC deletion with no VMs",
			vdcID:       "vdc-123",
			userID:      "admin-123",
			username:    "admin",
			role:        models.RoleSystemAdmin,
			userOrgID:   "",
			existingVMs: []*models.VirtualMachine{},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{}, nil)
				ms.On("DeleteVDC", "vdc-123").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteNamespace", mock.Anything, "org-test-vdc").Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should delete VDC when no VMs exist",
		},
		{
			name:      "VDC deletion blocked by existing VMs",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			existingVMs: []*models.VirtualMachine{
				{
					ID:     "vm-1",
					Name:   "Test VM 1",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-123"),
					Status: models.VMStatusRunning,
				},
				{
					ID:     "vm-2",
					Name:   "Test VM 2",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-123"),
					Status: models.VMStatusStopped,
				},
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{
					{
						ID:     "vm-1",
						Name:   "Test VM 1",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-123"),
						Status: models.VMStatusRunning,
					},
					{
						ID:     "vm-2",
						Name:   "Test VM 2",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-123"),
						Status: models.VMStatusStopped,
					},
				}, nil)
				// Should not call DeleteVDC due to existing VMs
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not call DeleteNamespace due to existing VMs
			},
			expectedStatus: http.StatusConflict,
			description:    "Should block VDC deletion when VMs exist",
		},
		{
			name:      "VDC deletion with VMs in different VDC (should succeed)",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			existingVMs: []*models.VirtualMachine{
				{
					ID:     "vm-1",
					Name:   "VM in different VDC",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-456"), // Different VDC
					Status: models.VMStatusRunning,
				},
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{
					{
						ID:     "vm-1",
						Name:   "VM in different VDC",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-456"), // Different VDC
						Status: models.VMStatusRunning,
					},
				}, nil)
				ms.On("DeleteVDC", "vdc-123").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteNamespace", mock.Anything, "org-test-vdc").Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should delete VDC when VMs exist in different VDCs",
		},
		{
			name:        "VDC deletion without OpenShift client",
			vdcID:       "vdc-123",
			userID:      "admin-123",
			username:    "admin",
			role:        models.RoleSystemAdmin,
			userOrgID:   "",
			existingVMs: []*models.VirtualMachine{},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{}, nil)
				ms.On("DeleteVDC", "vdc-123").Return(nil)
			},
			mockOpenShiftBehavior: nil, // No OpenShift client
			expectedStatus:        http.StatusOK,
			description:           "Should delete VDC even without OpenShift client",
		},
		{
			name:      "Storage error when listing VMs",
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
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return(nil, errors.New("storage error"))
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to storage error
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should handle storage errors when listing VMs",
		},
		{
			name:        "Namespace deletion fails but VDC deletion succeeds",
			vdcID:       "vdc-123",
			userID:      "admin-123",
			username:    "admin",
			role:        models.RoleSystemAdmin,
			userOrgID:   "",
			existingVMs: []*models.VirtualMachine{},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{}, nil)
				ms.On("DeleteVDC", "vdc-123").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteNamespace", mock.Anything, "org-test-vdc").Return(errors.New("namespace deletion failed"))
			},
			expectedStatus: http.StatusOK, // Should still succeed
			description:    "Should continue VDC deletion even if namespace deletion fails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			var mockOpenShiftClient *MockOpenShiftClient

			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			if tt.mockOpenShiftBehavior != nil {
				mockOpenShiftClient = &MockOpenShiftClient{}
				tt.mockOpenShiftBehavior(mockOpenShiftClient)
			}

			// Create handlers
			var handlers *VDCHandlers
			if mockOpenShiftClient != nil {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request
			c, w := setupGinContext("DELETE", "/vdcs/"+tt.vdcID, nil, tt.userID, tt.username, tt.role, tt.userOrgID)
			c.Params = gin.Params{{Key: "id", Value: tt.vdcID}}

			// Execute
			handlers.Delete(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}

			// Additional assertions for specific responses
			if tt.expectedStatus == http.StatusConflict {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "details")
				assert.Equal(t, "Cannot delete VDC with existing VMs", response["error"])
			} else if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "VDC deleted successfully", response["message"])
			}
		})
	}
}

func TestNewVDCHandlers(t *testing.T) {
	mockStorage := &MockStorage{}
	mockOpenShiftClient := &MockOpenShiftClient{}

	handlers := NewVDCHandlers(mockStorage, mockOpenShiftClient)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, mockOpenShiftClient, handlers.openshiftClient)
}

func TestVDCHandlers_Delete_WithCascadeDeletion(t *testing.T) {
	tests := []struct {
		name                  string
		vdcID                 string
		userID                string
		username              string
		role                  string
		userOrgID             string
		forceDelete           bool
		existingVMs           []*models.VirtualMachine
		mockStorageBehavior   func(*MockStorage)
		mockOpenShiftBehavior func(*MockOpenShiftClient)
		expectedStatus        int
		expectCascadeInfo     bool
		description           string
	}{
		{
			name:        "Successful cascade deletion with force=true",
			vdcID:       "vdc-123",
			userID:      "admin-123",
			username:    "admin",
			role:        models.RoleSystemAdmin,
			userOrgID:   "",
			forceDelete: true,
			existingVMs: []*models.VirtualMachine{
				{
					ID:     "vm-1",
					Name:   "Test VM 1",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-123"),
					Status: models.VMStatusRunning,
				},
				{
					ID:     "vm-2",
					Name:   "Test VM 2",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-123"),
					Status: models.VMStatusStopped,
				},
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{
					{
						ID:     "vm-1",
						Name:   "Test VM 1",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-123"),
						Status: models.VMStatusRunning,
					},
					{
						ID:     "vm-2",
						Name:   "Test VM 2",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-123"),
						Status: models.VMStatusStopped,
					},
				}, nil)
				ms.On("DeleteVM", "vm-1").Return(nil)
				ms.On("DeleteVM", "vm-2").Return(nil)
				ms.On("DeleteVDC", "vdc-123").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteNamespace", mock.Anything, "org-test-vdc").Return(nil)
			},
			expectedStatus:    http.StatusOK,
			expectCascadeInfo: true,
			description:       "Should successfully delete VDC and all VMs with force=true",
		},
		{
			name:        "VDC deletion blocked without force parameter",
			vdcID:       "vdc-123",
			userID:      "admin-123",
			username:    "admin",
			role:        models.RoleSystemAdmin,
			userOrgID:   "",
			forceDelete: false,
			existingVMs: []*models.VirtualMachine{
				{
					ID:     "vm-1",
					Name:   "Test VM 1",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-123"),
					Status: models.VMStatusRunning,
				},
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{
					{
						ID:     "vm-1",
						Name:   "Test VM 1",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-123"),
						Status: models.VMStatusRunning,
					},
				}, nil)
				// Should not call DeleteVM or DeleteVDC due to protection
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to blocked deletion
			},
			expectedStatus:    http.StatusConflict,
			expectCascadeInfo: false,
			description:       "Should block deletion with suggestion when VMs exist and force=false",
		},
		{
			name:        "Cascade deletion with some VM deletion failures",
			vdcID:       "vdc-123",
			userID:      "admin-123",
			username:    "admin",
			role:        models.RoleSystemAdmin,
			userOrgID:   "",
			forceDelete: true,
			existingVMs: []*models.VirtualMachine{
				{
					ID:     "vm-1",
					Name:   "Test VM 1",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-123"),
					Status: models.VMStatusRunning,
				},
				{
					ID:     "vm-2",
					Name:   "Test VM 2",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-123"),
					Status: models.VMStatusStopped,
				},
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{
					{
						ID:     "vm-1",
						Name:   "Test VM 1",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-123"),
						Status: models.VMStatusRunning,
					},
					{
						ID:     "vm-2",
						Name:   "Test VM 2",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-123"),
						Status: models.VMStatusStopped,
					},
				}, nil)
				ms.On("DeleteVM", "vm-1").Return(errors.New("VM deletion failed"))
				ms.On("DeleteVM", "vm-2").Return(nil)
				ms.On("DeleteVDC", "vdc-123").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteNamespace", mock.Anything, "org-test-vdc").Return(nil)
			},
			expectedStatus:    http.StatusOK,
			expectCascadeInfo: true,
			description:       "Should continue VDC deletion even if some VM deletions fail",
		},
		{
			name:        "Force deletion with no VMs",
			vdcID:       "vdc-123",
			userID:      "admin-123",
			username:    "admin",
			role:        models.RoleSystemAdmin,
			userOrgID:   "",
			forceDelete: true,
			existingVMs: []*models.VirtualMachine{},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123",
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{}, nil)
				ms.On("DeleteVDC", "vdc-123").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteNamespace", mock.Anything, "org-test-vdc").Return(nil)
			},
			expectedStatus:    http.StatusOK,
			expectCascadeInfo: false,
			description:       "Should delete VDC normally when force=true but no VMs exist",
		},
		{
			name:        "Org admin cascade deletion for own organization",
			vdcID:       "vdc-123",
			userID:      "orgadmin-123",
			username:    "orgadmin",
			role:        models.RoleOrgAdmin,
			userOrgID:   "org-123",
			forceDelete: true,
			existingVMs: []*models.VirtualMachine{
				{
					ID:     "vm-1",
					Name:   "Org VM",
					OrgID:  "org-123",
					VDCID:  stringPtr("vdc-123"),
					Status: models.VMStatusRunning,
				},
			},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Same as user's org
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				ms.On("ListVMs", "org-123").Return([]*models.VirtualMachine{
					{
						ID:     "vm-1",
						Name:   "Org VM",
						OrgID:  "org-123",
						VDCID:  stringPtr("vdc-123"),
						Status: models.VMStatusRunning,
					},
				}, nil)
				ms.On("DeleteVM", "vm-1").Return(nil)
				ms.On("DeleteVDC", "vdc-123").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("DeleteNamespace", mock.Anything, "org-test-vdc").Return(nil)
			},
			expectedStatus:    http.StatusOK,
			expectCascadeInfo: true,
			description:       "Should allow org admin to cascade delete VDCs in their organization",
		},
		{
			name:        "Forbidden cascade deletion for different organization",
			vdcID:       "vdc-123",
			userID:      "orgadmin-123",
			username:    "orgadmin",
			role:        models.RoleOrgAdmin,
			userOrgID:   "org-456", // Different from VDC's org
			forceDelete: true,
			existingVMs: []*models.VirtualMachine{},
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Different from user's org
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
				// Should not call other methods due to permission check
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to permission check
			},
			expectedStatus:    http.StatusForbidden,
			expectCascadeInfo: false,
			description:       "Should forbid org admin from cascade deleting VDCs in different organizations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			var mockOpenShiftClient *MockOpenShiftClient

			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			if tt.mockOpenShiftBehavior != nil {
				mockOpenShiftClient = &MockOpenShiftClient{}
				tt.mockOpenShiftBehavior(mockOpenShiftClient)
			}

			// Create handlers
			var handlers *VDCHandlers
			if mockOpenShiftClient != nil {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &VDCHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request with force parameter
			url := "/vdcs/" + tt.vdcID
			if tt.forceDelete {
				url += "?force=true"
			}
			c, w := setupGinContext("DELETE", url, nil, tt.userID, tt.username, tt.role, tt.userOrgID)
			c.Params = gin.Params{{Key: "id", Value: tt.vdcID}}

			// Execute
			handlers.Delete(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}

			// Additional assertions for successful responses
			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "VDC deleted successfully", response["message"])

				// Check cascade deletion information
				if tt.expectCascadeInfo {
					assert.Contains(t, response, "cascade_deletion")
					cascadeInfo := response["cascade_deletion"].(map[string]interface{})
					assert.Equal(t, float64(len(tt.existingVMs)), cascadeInfo["total_vms_deleted"])
					assert.Equal(t, true, cascadeInfo["force_delete_used"])
					assert.Equal(t, true, cascadeInfo["deletion_successful"])
				} else {
					assert.NotContains(t, response, "cascade_deletion")
				}
			} else if tt.expectedStatus == http.StatusConflict {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Cannot delete VDC with existing VMs", response["error"])
				assert.Contains(t, response, "details")
				details := response["details"].(map[string]interface{})
				assert.Contains(t, details, "suggestion")
				assert.Contains(t, details["suggestion"], "force=true")
			}
		})
	}
}

// Helper functions for tests
func intPtr(i int) *int {
	return &i
}

func TestVDCHandlers_LimitRangeRequestValidation(t *testing.T) {
	// Test JSON binding for LimitRangeRequest
	testCases := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		description    string
	}{
		{
			name: "Valid LimitRangeRequest JSON",
			requestBody: models.LimitRangeRequest{
				MinCPU:    1,
				MaxCPU:    8,
				MinMemory: 1,
				MaxMemory: 16,
			},
			expectedStatus: http.StatusOK,
			description:    "Should accept valid JSON for LimitRangeRequest",
		},
		{
			name:           "Invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject invalid JSON",
		},
		{
			name: "Missing fields (valid - will be zero values)",
			requestBody: map[string]interface{}{
				"min_cpu": 1,
				// Missing other fields
			},
			expectedStatus: http.StatusBadRequest, // Due to validation, not JSON parsing
			description:    "Should handle missing fields appropriately",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockOpenShiftClient := &MockOpenShiftClient{}

			// Setup basic VDC for successful retrieval
			vdc := &models.VirtualDataCenter{
				ID:                "vdc-123",
				Name:              "Test VDC",
				OrgID:             "org-123",
				WorkloadNamespace: "org-test-vdc",
			}
			mockStorage.On("GetVDC", "vdc-123").Return(vdc, nil)

			// For the valid JSON case, we need to set up OpenShift client expectations
			// since it will actually try to update the LimitRange
			if tc.expectedStatus == http.StatusOK {
				// Mock successful UpdateLimitRange call
				mockOpenShiftClient.On("UpdateLimitRange", mock.Anything, "org-test-vdc", 1, 8, 1, 16).Return(nil)

				// Mock successful GetLimitRange call after update
				limitRangeInfo := &models.LimitRangeInfo{
					Exists:    true,
					MinCPU:    1,
					MaxCPU:    8,
					MinMemory: 1,
					MaxMemory: 16,
				}
				mockOpenShiftClient.On("GetLimitRange", mock.Anything, "org-test-vdc").Return(limitRangeInfo, nil)
			}

			handlers := &VDCHandlers{
				storage:         mockStorage,
				openshiftClient: mockOpenShiftClient,
			}

			c, w := setupGinContext("PUT", "/vdcs/vdc-123/limitrange", tc.requestBody, "admin-123", "admin", models.RoleSystemAdmin, "")
			c.Params = gin.Params{{Key: "id", Value: "vdc-123"}}

			handlers.UpdateLimitRange(c)

			// We're mainly testing JSON parsing here, so specific status may vary
			// But bad JSON should definitely return 4xx
			if tc.expectedStatus >= 400 && tc.expectedStatus < 500 {
				assert.True(t, w.Code >= 400 && w.Code < 500, tc.description)
			} else {
				assert.Equal(t, tc.expectedStatus, w.Code, tc.description)
			}

			mockStorage.AssertExpectations(t)
			if tc.expectedStatus == http.StatusOK {
				mockOpenShiftClient.AssertExpectations(t)
			}
		})
	}
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
			name:      "Resource usage retrieval by org admin for own organization",
			vdcID:     "vdc-123",
			userID:    "orgadmin-123",
			username:  "orgadmin",
			role:      models.RoleOrgAdmin,
			userOrgID: "org-123",
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Same as user's org
					WorkloadNamespace: "org-test-vdc",
					CPUQuota:          10,
					MemoryQuota:       32,
					StorageQuota:      200,
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)

				vms := []*models.VirtualMachine{
					{
						ID:       "vm-1",
						Name:     "Test VM 1",
						VDCID:    stringPtr("vdc-123"),
						Status:   "Running",
						CPU:      2,
						Memory:   "4Gi",
						DiskSize: "30Gi",
					},
				}
				ms.On("ListVMs", "org-123").Return(vms, nil)
			},
			expectedStatus:      http.StatusOK,
			expectedCPUUsed:     intPtr(2),
			expectedMemoryUsed:  intPtr(4),
			expectedStorageUsed: intPtr(30),
			expectedVMCount:     intPtr(1),
			description:         "Should allow org admin to view resource usage for VDCs in their organization",
		},
		{
			name:      "Forbidden access by org admin for different organization",
			vdcID:     "vdc-123",
			userID:    "orgadmin-123",
			username:  "orgadmin",
			role:      models.RoleOrgAdmin,
			userOrgID: "org-456", // Different from VDC's org
			mockStorageBehavior: func(ms *MockStorage) {
				vdc := &models.VirtualDataCenter{
					ID:                "vdc-123",
					Name:              "Test VDC",
					OrgID:             "org-123", // Different from user's org
					WorkloadNamespace: "org-test-vdc",
				}
				ms.On("GetVDC", "vdc-123").Return(vdc, nil)
			},
			expectedStatus: http.StatusForbidden,
			description:    "Should deny access to org admin for VDCs in different organization",
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
		{
			name:      "Storage error when getting VDC",
			vdcID:     "vdc-123",
			userID:    "admin-123",
			username:  "admin",
			role:      models.RoleSystemAdmin,
			userOrgID: "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetVDC", "vdc-123").Return(nil, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should return 500 when storage error occurs getting VDC",
		},
		{
			name:      "Storage error when listing VMs",
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
				ms.On("ListVMs", "org-123").Return(nil, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should return 500 when storage error occurs listing VMs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tc.mockStorageBehavior(mockStorage)

			handlers := &VDCHandlers{
				storage:         mockStorage,
				openshiftClient: nil, // Not needed for this test
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
