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
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

func TestNewOrganizationHandlers(t *testing.T) {
	mockStorage := &MockStorage{}
	mockK8sClient := &MockK8sClient{}

	handlers := NewOrganizationHandlers(mockStorage, mockK8sClient)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, mockK8sClient, handlers.k8sClient)
}

func TestOrganizationHandlers_List(t *testing.T) {
	tests := []struct {
		name                string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
		expectedOrgs        int
	}{
		{
			name: "successful list",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListOrganizations").Return([]*models.Organization{
					{ID: "org1", Name: "Organization 1"},
					{ID: "org2", Name: "Organization 2"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedOrgs:   2,
		},
		{
			name: "storage error",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListOrganizations").Return([]*models.Organization{}, fmt.Errorf("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedOrgs:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewOrganizationHandlers(mockStorage, nil)
			c, w := setupGinContext("GET", "/organizations", nil, "user1", "admin", models.RoleSystemAdmin, "")

			handlers.List(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, float64(tt.expectedOrgs), response["total"])
			}
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestOrganizationHandlers_Get(t *testing.T) {
	tests := []struct {
		name                string
		orgID               string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
	}{
		{
			name:  "successful get",
			orgID: "org1",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "org1").Return(&models.Organization{
					ID:   "org1",
					Name: "Test Org",
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "organization not found",
			orgID: "nonexistent",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:  "storage error",
			orgID: "org1",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "org1").Return(nil, fmt.Errorf("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewOrganizationHandlers(mockStorage, nil)
			c, w := setupGinContext("GET", fmt.Sprintf("/organizations/%s", tt.orgID), nil, "user1", "admin", models.RoleSystemAdmin, "")
			c.Params = []gin.Param{{Key: "id", Value: tt.orgID}}

			handlers.Get(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestOrganizationHandlers_Create_CRDOnly(t *testing.T) {
	tests := []struct {
		name                string
		requestBody         models.CreateOrganizationRequest
		userID              string
		username            string
		role                string
		orgID               string
		mockStorageBehavior func(*MockStorage)
		mockK8sBehavior     func(*MockK8sClient)
		expectedStatus      int
		description         string
	}{
		{
			name: "Successful organization creation (CRD-only)",
			requestBody: models.CreateOrganizationRequest{
				Name:        "test-org",
				DisplayName: "Test Organization",
				Description: "Test organization description",
				Admins:      []string{"admin"},
				IsEnabled:   true,
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				// No storage calls needed - CRD controller handles sync
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				mk.On("Create", mock.Anything, mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			description:    "Organization should be created via CRD",
		},
		{
			name: "Invalid request body",
			requestBody: models.CreateOrganizationRequest{
				Name: "", // Invalid empty name
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
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
			name: "Kubernetes client error",
			requestBody: models.CreateOrganizationRequest{
				Name:        "test-org",
				DisplayName: "Test Organization",
				Admins:      []string{"admin"},
				IsEnabled:   true,
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				// No storage calls needed
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				mk.On("Create", mock.Anything, mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(fmt.Errorf("k8s error"))
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should handle kubernetes errors",
		},
		{
			name: "Unauthorized user (not system admin)",
			requestBody: models.CreateOrganizationRequest{
				Name:        "test-org",
				DisplayName: "Test Organization",
				Admins:      []string{"admin"},
				IsEnabled:   true,
			},
			userID:   "user-123",
			username: "regularuser",
			role:     models.RoleOrgUser, // Not system admin
			orgID:    "some-org",
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// No calls expected - authorization check happens before K8s calls
			},
			expectedStatus: http.StatusForbidden,
			description:    "Only system admins can create organizations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			mockK8sClient := &MockK8sClient{}

			tt.mockStorageBehavior(mockStorage)
			tt.mockK8sBehavior(mockK8sClient)

			handlers := NewOrganizationHandlers(mockStorage, mockK8sClient)
			c, w := setupGinContext("POST", "/organizations", tt.requestBody, tt.userID, tt.username, tt.role, tt.orgID)

			handlers.Create(c)

			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)
			mockStorage.AssertExpectations(t)
			mockK8sClient.AssertExpectations(t)
		})
	}
}

func TestOrganizationHandlers_Update_CRDOnly(t *testing.T) {
	tests := []struct {
		name                string
		orgID               string
		requestBody         models.UpdateOrganizationRequest
		userID              string
		role                string
		mockStorageBehavior func(*MockStorage)
		mockK8sBehavior     func(*MockK8sClient)
		expectedStatus      int
	}{
		{
			name:  "successful update",
			orgID: "test-org",
			requestBody: models.UpdateOrganizationRequest{
				DisplayName: stringPtr("Updated Organization"),
				Description: stringPtr("Updated description"),
				IsEnabled:   boolPtr(false),
			},
			userID: "user-123",
			role:   models.RoleSystemAdmin,
			mockStorageBehavior: func(ms *MockStorage) {
				// No storage calls needed - CRD controller handles sync
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock getting existing organization
				mk.On("Get", mock.Anything, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					org := args.Get(2).(*ovimv1.Organization)
					org.Name = "test-org"
					org.Spec.DisplayName = "Original Name"
				})
				// Mock update
				mk.On("Update", mock.Anything, mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "organization not found",
			orgID: "nonexistent",
			requestBody: models.UpdateOrganizationRequest{
				DisplayName: stringPtr("Updated Organization"),
			},
			userID: "user-123",
			role:   models.RoleSystemAdmin,
			mockStorageBehavior: func(ms *MockStorage) {
				// No storage calls
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				mk.On("Get", mock.Anything, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(kerrors.NewNotFound(schema.GroupResource{Group: "ovim.io", Resource: "organizations"}, "nonexistent"))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:  "unauthorized access",
			orgID: "test-org",
			requestBody: models.UpdateOrganizationRequest{
				DisplayName: stringPtr("Updated Organization"),
			},
			userID: "user-123",
			role:   models.RoleOrgMember, // Not authorized
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// No calls expected
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

			handlers := NewOrganizationHandlers(mockStorage, mockK8sClient)
			c, w := setupGinContext("PUT", fmt.Sprintf("/organizations/%s", tt.orgID), tt.requestBody, tt.userID, "admin", tt.role, "")
			c.Params = []gin.Param{{Key: "id", Value: tt.orgID}}

			handlers.Update(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
			mockK8sClient.AssertExpectations(t)
		})
	}
}

func TestOrganizationHandlers_Delete_CRDOnly(t *testing.T) {
	tests := []struct {
		name                string
		orgID               string
		userRole            string
		mockStorageBehavior func(*MockStorage)
		mockK8sBehavior     func(*MockK8sClient)
		expectedStatus      int
	}{
		{
			name:     "successful deletion",
			orgID:    "test-org",
			userRole: models.RoleSystemAdmin,
			mockStorageBehavior: func(ms *MockStorage) {
				// Check for dependent VDCs
				ms.On("ListVDCs", "test-org").Return([]*models.VirtualDataCenter{}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// Mock getting existing organization
				mk.On("Get", mock.Anything, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					org := args.Get(2).(*ovimv1.Organization)
					org.Name = "test-org"
				})
				// Mock update for deletion annotations
				mk.On("Update", mock.Anything, mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(nil)
				// Mock delete
				mk.On("Delete", mock.Anything, mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:     "organization has VDCs",
			orgID:    "test-org",
			userRole: models.RoleSystemAdmin,
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVDCs", "test-org").Return([]*models.VirtualDataCenter{
					{ID: "vdc1", Name: "VDC 1"},
				}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// No K8s calls expected
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:     "organization not found",
			orgID:    "nonexistent",
			userRole: models.RoleSystemAdmin,
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("ListVDCs", "nonexistent").Return([]*models.VirtualDataCenter{}, nil)
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				mk.On("Get", mock.Anything, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1.Organization"), mock.Anything).Return(kerrors.NewNotFound(schema.GroupResource{Group: "ovim.io", Resource: "organizations"}, "nonexistent"))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:     "unauthorized access",
			orgID:    "test-org",
			userRole: models.RoleOrgMember,
			mockStorageBehavior: func(ms *MockStorage) {
				// No calls expected
			},
			mockK8sBehavior: func(mk *MockK8sClient) {
				// No calls expected
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

			handlers := NewOrganizationHandlers(mockStorage, mockK8sClient)
			c, w := setupGinContext("DELETE", fmt.Sprintf("/organizations/%s", tt.orgID), nil, "user1", "admin", tt.userRole, "")
			c.Params = []gin.Param{{Key: "id", Value: tt.orgID}}

			handlers.Delete(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
			mockK8sClient.AssertExpectations(t)
		})
	}
}

func TestOrganizationHandlers_GetUserOrganization(t *testing.T) {
	tests := []struct {
		name                string
		userOrgID           string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
	}{
		{
			name:      "successful get user organization",
			userOrgID: "user-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "user-org").Return(&models.Organization{
					ID:   "user-org",
					Name: "User Organization",
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
			name:      "organization not found",
			userOrgID: "nonexistent",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewOrganizationHandlers(mockStorage, nil)
			c, w := setupGinContext("GET", "/user/organization", nil, "user1", "user", models.RoleOrgMember, tt.userOrgID)

			handlers.GetUserOrganization(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestOrganizationHandlers_GetResourceUsage(t *testing.T) {
	tests := []struct {
		name                string
		orgID               string
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
	}{
		{
			name:  "successful get resource usage",
			orgID: "test-org",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "test-org").Return(&models.Organization{
					ID:   "test-org",
					Name: "Test Organization",
				}, nil)
				ms.On("ListVDCs", "test-org").Return([]*models.VirtualDataCenter{
					{ID: "vdc1", Name: "VDC 1", OrgID: "test-org"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "organization not found",
			orgID: "nonexistent",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := &MockStorage{}
			tt.mockStorageBehavior(mockStorage)

			handlers := NewOrganizationHandlers(mockStorage, nil)
			c, w := setupGinContext("GET", fmt.Sprintf("/organizations/%s/usage", tt.orgID), nil, "user1", "admin", models.RoleSystemAdmin, "")
			c.Params = []gin.Param{{Key: "id", Value: tt.orgID}}

			handlers.GetResourceUsage(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestOrganizationHandlers_UpdateResourceQuotas(t *testing.T) {
	mockStorage := &MockStorage{}
	handlers := NewOrganizationHandlers(mockStorage, nil)
	c, w := setupGinContext("PUT", "/organizations/test-org/quotas", nil, "user1", "admin", models.RoleSystemAdmin, "")

	handlers.UpdateResourceQuotas(c)

	// This method is not implemented, should return 501
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestOrganizationHandlers_ValidateResourceAllocation(t *testing.T) {
	mockStorage := &MockStorage{}
	handlers := NewOrganizationHandlers(mockStorage, nil)
	c, w := setupGinContext("POST", "/organizations/test-org/validate", nil, "user1", "admin", models.RoleSystemAdmin, "")

	handlers.ValidateResourceAllocation(c)

	// This method is not implemented, should return 501
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}