package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// MockStorage is a mock implementation of storage.Storage interface
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) CreateOrganization(org *models.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockStorage) GetOrganization(id string) (*models.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *MockStorage) UpdateOrganization(org *models.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockStorage) DeleteOrganization(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListOrganizations() ([]*models.Organization, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Organization), args.Error(1)
}

func (m *MockStorage) ListVDCs(orgID string) ([]*models.VirtualDataCenter, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.VirtualDataCenter), args.Error(1)
}

// Additional storage methods needed for interface compliance
func (m *MockStorage) CreateUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockStorage) GetUserByID(id string) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockStorage) GetUserByUsername(username string) (*models.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockStorage) UpdateUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockStorage) DeleteUser(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListUsers() ([]*models.User, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *MockStorage) ListUsersByOrg(orgID string) ([]*models.User, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *MockStorage) CreateVDC(vdc *models.VirtualDataCenter) error {
	args := m.Called(vdc)
	return args.Error(0)
}

func (m *MockStorage) GetVDC(id string) (*models.VirtualDataCenter, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.VirtualDataCenter), args.Error(1)
}

func (m *MockStorage) UpdateVDC(vdc *models.VirtualDataCenter) error {
	args := m.Called(vdc)
	return args.Error(0)
}

func (m *MockStorage) DeleteVDC(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) CreateVM(vm *models.VirtualMachine) error {
	args := m.Called(vm)
	return args.Error(0)
}

func (m *MockStorage) GetVM(id string) (*models.VirtualMachine, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.VirtualMachine), args.Error(1)
}

func (m *MockStorage) UpdateVM(vm *models.VirtualMachine) error {
	args := m.Called(vm)
	return args.Error(0)
}

func (m *MockStorage) DeleteVM(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListVMs(orgID string) ([]*models.VirtualMachine, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.VirtualMachine), args.Error(1)
}

func (m *MockStorage) CreateTemplate(template *models.Template) error {
	args := m.Called(template)
	return args.Error(0)
}

func (m *MockStorage) GetTemplate(id string) (*models.Template, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Template), args.Error(1)
}

func (m *MockStorage) UpdateTemplate(template *models.Template) error {
	args := m.Called(template)
	return args.Error(0)
}

func (m *MockStorage) DeleteTemplate(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListTemplates() ([]*models.Template, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Template), args.Error(1)
}

func (m *MockStorage) ListTemplatesByOrg(orgID string) ([]*models.Template, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Template), args.Error(1)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStorage) Ping() error {
	args := m.Called()
	return args.Error(0)
}

// MockOpenShiftClient is a mock implementation for testing namespace operations
type MockOpenShiftClient struct {
	mock.Mock
}

func (m *MockOpenShiftClient) CreateNamespace(ctx context.Context, name string, labels map[string]string, annotations map[string]string) error {
	args := m.Called(ctx, name, labels, annotations)
	return args.Error(0)
}

func (m *MockOpenShiftClient) CreateResourceQuota(ctx context.Context, namespace string, cpuQuota, memoryQuota, storageQuota int) error {
	args := m.Called(ctx, namespace, cpuQuota, memoryQuota, storageQuota)
	return args.Error(0)
}

func (m *MockOpenShiftClient) CreateLimitRange(ctx context.Context, namespace string, minCPU, maxCPU, minMemory, maxMemory int) error {
	args := m.Called(ctx, namespace, minCPU, maxCPU, minMemory, maxMemory)
	return args.Error(0)
}

func (m *MockOpenShiftClient) DeleteNamespace(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockOpenShiftClient) NamespaceExists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockOpenShiftClient) IsConnected(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

// Helper function to set up a Gin context with authentication
func setupGinContext(method, url string, body interface{}, userID, username, role, orgID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)

	var jsonBody []byte
	if body != nil {
		jsonBody, _ = json.Marshal(body)
	}

	req, _ := http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Set authentication context
	c.Set(auth.ContextKeyUserID, userID)
	c.Set(auth.ContextKeyUsername, username)
	c.Set(auth.ContextKeyRole, role)
	c.Set(auth.ContextKeyOrgID, orgID)

	return c, w
}

func TestOrganizationHandlers_Create_WithNamespace(t *testing.T) {
	tests := []struct {
		name                     string
		requestBody              models.CreateOrganizationRequest
		userID                   string
		username                 string
		role                     string
		orgID                    string
		mockStorageBehavior      func(*MockStorage)
		mockOpenShiftBehavior    func(*MockOpenShiftClient)
		expectedStatus           int
		expectedNamespaceCreated bool
		expectedRollback         bool
		description              string
	}{
		{
			name: "Organization creation with custom resource quotas",
			requestBody: models.CreateOrganizationRequest{
				Name:         "Custom Quota Corp",
				Description:  "Organization with custom quotas",
				IsEnabled:    true,
				CPUQuota:     intPtr(25),  // Custom CPU quota
				MemoryQuota:  intPtr(50),  // Custom memory quota
				StorageQuota: intPtr(200), // Custom storage quota
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("CreateOrganization", mock.MatchedBy(func(org *models.Organization) bool {
					// Verify that custom quotas are set correctly
					return org.CPUQuota == 25 && org.MemoryQuota == 50 && org.StorageQuota == 200
				})).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "custom-quota-corp").Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, "custom-quota-corp", mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, "custom-quota-corp", 25, 50, 200).Return(nil)
			},
			expectedStatus:           http.StatusCreated,
			expectedNamespaceCreated: true,
			description:              "Should create organization with custom resource quotas",
		},
		{
			name: "Organization creation with partial custom quotas",
			requestBody: models.CreateOrganizationRequest{
				Name:         "Partial Quota Corp",
				Description:  "Organization with some custom quotas",
				IsEnabled:    true,
				CPUQuota:     intPtr(15),  // Custom CPU quota
				MemoryQuota:  intPtr(20),  // Required value
				StorageQuota: intPtr(300), // Custom storage quota
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("CreateOrganization", mock.MatchedBy(func(org *models.Organization) bool {
					// Verify that custom quotas are set correctly
					return org.CPUQuota == 15 && org.MemoryQuota == 20 && org.StorageQuota == 300
				})).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "partial-quota-corp").Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, "partial-quota-corp", mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, "partial-quota-corp", 15, 20, 300).Return(nil)
			},
			expectedStatus:           http.StatusCreated,
			expectedNamespaceCreated: true,
			description:              "Should create organization with mix of custom and default quotas",
		},
		{
			name: "Organization creation with zero quotas (rejected)",
			requestBody: models.CreateOrganizationRequest{
				Name:         "Zero Quota Corp",
				Description:  "Organization with zero quotas (should be rejected)",
				IsEnabled:    true,
				CPUQuota:     intPtr(0), // Zero CPU quota - should be rejected
				MemoryQuota:  intPtr(0), // Zero memory quota - should be rejected
				StorageQuota: intPtr(0), // Zero storage quota - should be rejected
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				// Should not be called due to validation failure
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to validation failure
			},
			expectedStatus:           http.StatusBadRequest,
			expectedNamespaceCreated: false,
			description:              "Should reject zero quotas",
		},
		{
			name: "Organization creation with negative quotas (rejected)",
			requestBody: models.CreateOrganizationRequest{
				Name:         "Negative Quota Corp",
				Description:  "Organization with negative quotas (should be rejected)",
				IsEnabled:    true,
				CPUQuota:     intPtr(-5),  // Negative CPU quota - should be rejected
				MemoryQuota:  intPtr(-10), // Negative memory quota - should be rejected
				StorageQuota: intPtr(-20), // Negative storage quota - should be rejected
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				// Should not be called due to validation failure
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to validation failure
			},
			expectedStatus:           http.StatusBadRequest,
			expectedNamespaceCreated: false,
			description:              "Should reject negative quotas",
		},
		{
			name: "Successful organization and namespace creation",
			requestBody: models.CreateOrganizationRequest{
				Name:         "Acme Corporation",
				Description:  "Test organization",
				IsEnabled:    true,
				CPUQuota:     intPtr(10),
				MemoryQuota:  intPtr(20),
				StorageQuota: intPtr(100),
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("CreateOrganization", mock.AnythingOfType("*models.Organization")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "acme-corporation").Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, "acme-corporation", mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, "acme-corporation", 10, 20, 100).Return(nil)
			},
			expectedStatus:           http.StatusCreated,
			expectedNamespaceCreated: true,
			description:              "Should create organization in database and namespace in cluster",
		},
		{
			name: "Organization creation with existing namespace",
			requestBody: models.CreateOrganizationRequest{
				Name:         "Existing Corp",
				Description:  "Organization with existing namespace",
				IsEnabled:    true,
				CPUQuota:     intPtr(10),
				MemoryQuota:  intPtr(20),
				StorageQuota: intPtr(100),
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("CreateOrganization", mock.AnythingOfType("*models.Organization")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "existing-corp").Return(true, nil)
				// Should not call CreateNamespace since it exists
			},
			expectedStatus:           http.StatusCreated,
			expectedNamespaceCreated: false, // Not created because it exists
			description:              "Should create organization but skip namespace creation if it exists",
		},
		{
			name: "Namespace creation fails with rollback",
			requestBody: models.CreateOrganizationRequest{
				Name:         "Failed Corp",
				Description:  "Organization that fails namespace creation",
				IsEnabled:    true,
				CPUQuota:     intPtr(10),
				MemoryQuota:  intPtr(20),
				StorageQuota: intPtr(100),
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("CreateOrganization", mock.AnythingOfType("*models.Organization")).Return(nil)
				ms.On("DeleteOrganization", "failed-corp").Return(nil) // Rollback
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "failed-corp").Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, "failed-corp", mock.Anything, mock.Anything).Return(errors.New("namespace creation failed"))
			},
			expectedStatus:   http.StatusInternalServerError,
			expectedRollback: true,
			description:      "Should rollback organization creation if namespace creation fails",
		},
		{
			name: "Organization creation without OpenShift client",
			requestBody: models.CreateOrganizationRequest{
				Name:         "No Client Corp",
				Description:  "Organization created without OpenShift client",
				IsEnabled:    true,
				CPUQuota:     intPtr(10),
				MemoryQuota:  intPtr(20),
				StorageQuota: intPtr(100),
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("CreateOrganization", mock.AnythingOfType("*models.Organization")).Return(nil)
			},
			mockOpenShiftBehavior: nil, // No OpenShift client
			expectedStatus:        http.StatusCreated,
			description:           "Should create organization even without OpenShift client",
		},
		{
			name: "Database creation fails",
			requestBody: models.CreateOrganizationRequest{
				Name:         "DB Fail Corp",
				Description:  "Organization that fails database creation",
				IsEnabled:    true,
				CPUQuota:     intPtr(10),
				MemoryQuota:  intPtr(20),
				StorageQuota: intPtr(100),
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("CreateOrganization", mock.AnythingOfType("*models.Organization")).Return(errors.New("database error"))
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called since database creation fails first
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should fail organization creation if database creation fails",
		},
		{
			name: "Resource quota creation fails (non-fatal)",
			requestBody: models.CreateOrganizationRequest{
				Name:         "Quota Fail Corp",
				Description:  "Organization where quota creation fails",
				IsEnabled:    true,
				CPUQuota:     intPtr(10),
				MemoryQuota:  intPtr(20),
				StorageQuota: intPtr(100),
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("CreateOrganization", mock.AnythingOfType("*models.Organization")).Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "quota-fail-corp").Return(false, nil)
				moc.On("CreateNamespace", mock.Anything, "quota-fail-corp", mock.Anything, mock.Anything).Return(nil)
				moc.On("CreateResourceQuota", mock.Anything, "quota-fail-corp", 10, 20, 100).Return(errors.New("quota creation failed"))
			},
			expectedStatus:           http.StatusCreated,
			expectedNamespaceCreated: true,
			description:              "Should succeed even if resource quota creation fails (can be fixed later)",
		},
		{
			name: "Invalid request body",
			requestBody: models.CreateOrganizationRequest{
				Name:        "", // Empty name
				Description: "Invalid organization",
				IsEnabled:   true,
			},
			userID:   "user-123",
			username: "admin",
			role:     models.RoleSystemAdmin,
			orgID:    "",
			mockStorageBehavior: func(ms *MockStorage) {
				// Should not be called due to validation
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to validation
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should fail with bad request for invalid input",
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
			var handlers *OrganizationHandlers
			if mockOpenShiftClient != nil {
				handlers = &OrganizationHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &OrganizationHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request
			c, w := setupGinContext("POST", "/organizations", tt.requestBody, tt.userID, tt.username, tt.role, tt.orgID)

			// Execute
			handlers.Create(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}

			// Additional assertions based on expected behavior
			if tt.expectedStatus == http.StatusCreated {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response["id"])
				assert.Equal(t, tt.requestBody.Name, response["name"])
			}
		})
	}
}

func TestOrganizationHandlers_Delete_WithNamespace(t *testing.T) {
	tests := []struct {
		name                  string
		organizationID        string
		userID                string
		username              string
		role                  string
		orgID                 string
		mockStorageBehavior   func(*MockStorage)
		mockOpenShiftBehavior func(*MockOpenShiftClient)
		expectedStatus        int
		description           string
	}{
		{
			name:           "Successful organization and namespace deletion",
			organizationID: "test-org",
			userID:         "user-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "test-org",
					Name:      "Test Organization",
					Namespace: "test-org",
				}
				ms.On("GetOrganization", "test-org").Return(org, nil)

				// Mock cascade deletion calls
				ms.On("ListVMs", "test-org").Return([]*models.VirtualMachine{}, nil)
				ms.On("ListVDCs", "test-org").Return([]*models.VirtualDataCenter{}, nil)
				ms.On("ListTemplatesByOrg", "test-org").Return([]*models.Template{}, nil)
				ms.On("ListUsersByOrg", "test-org").Return([]*models.User{}, nil)

				ms.On("DeleteOrganization", "test-org").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "test-org").Return(true, nil)
				moc.On("DeleteNamespace", mock.Anything, "test-org").Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should delete organization from database and namespace from cluster",
		},
		{
			name:           "Organization deletion with non-existent namespace",
			organizationID: "test-org-2",
			userID:         "user-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "test-org-2",
					Name:      "Test Organization 2",
					Namespace: "test-org-2",
				}
				ms.On("GetOrganization", "test-org-2").Return(org, nil)

				// Mock cascade deletion calls
				ms.On("ListVMs", "test-org-2").Return([]*models.VirtualMachine{}, nil)
				ms.On("ListVDCs", "test-org-2").Return([]*models.VirtualDataCenter{}, nil)
				ms.On("ListTemplatesByOrg", "test-org-2").Return([]*models.Template{}, nil)
				ms.On("ListUsersByOrg", "test-org-2").Return([]*models.User{}, nil)

				ms.On("DeleteOrganization", "test-org-2").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "test-org-2").Return(false, nil)
				// Should not call DeleteNamespace since it doesn't exist
			},
			expectedStatus: http.StatusOK,
			description:    "Should delete organization even if namespace doesn't exist",
		},
		{
			name:           "Organization deletion without OpenShift client",
			organizationID: "test-org-3",
			userID:         "user-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "test-org-3",
					Name:      "Test Organization 3",
					Namespace: "test-org-3",
				}
				ms.On("GetOrganization", "test-org-3").Return(org, nil)

				// Mock cascade deletion calls
				ms.On("ListVMs", "test-org-3").Return([]*models.VirtualMachine{}, nil)
				ms.On("ListVDCs", "test-org-3").Return([]*models.VirtualDataCenter{}, nil)
				ms.On("ListTemplatesByOrg", "test-org-3").Return([]*models.Template{}, nil)
				ms.On("ListUsersByOrg", "test-org-3").Return([]*models.User{}, nil)

				ms.On("DeleteOrganization", "test-org-3").Return(nil)
			},
			mockOpenShiftBehavior: nil, // No OpenShift client
			expectedStatus:        http.StatusOK,
			description:           "Should delete organization even without OpenShift client",
		},
		{
			name:           "Organization not found",
			organizationID: "non-existent",
			userID:         "user-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "non-existent").Return(nil, storage.ErrNotFound)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called since organization doesn't exist
			},
			expectedStatus: http.StatusNotFound,
			description:    "Should return not found for non-existent organization",
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
			var handlers *OrganizationHandlers
			if mockOpenShiftClient != nil {
				handlers = &OrganizationHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &OrganizationHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request
			c, w := setupGinContext("DELETE", "/organizations/"+tt.organizationID, nil, tt.userID, tt.username, tt.role, tt.orgID)
			c.Params = gin.Params{{Key: "id", Value: tt.organizationID}}

			// Execute
			handlers.Delete(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}
		})
	}
}

func TestNewOrganizationHandlers(t *testing.T) {
	mockStorage := &MockStorage{}
	mockOpenShiftClient := &MockOpenShiftClient{}

	handlers := NewOrganizationHandlers(mockStorage, mockOpenShiftClient)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, mockOpenShiftClient, handlers.openshiftClient)
}

func TestOrganizationHandlers_NamespaceLabelsAndAnnotations(t *testing.T) {
	// Test that the correct labels and annotations are set when creating namespaces
	mockStorage := &MockStorage{}
	mockOpenShiftClient := &MockOpenShiftClient{}

	mockStorage.On("CreateOrganization", mock.AnythingOfType("*models.Organization")).Return(nil)
	mockOpenShiftClient.On("NamespaceExists", mock.Anything, "test-labels").Return(false, nil)

	// Verify the labels and annotations passed to CreateNamespace
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "ovim",
		"app.kubernetes.io/component":  "organization",
		"app.kubernetes.io/managed-by": "ovim",
		"ovim.io/organization-id":      "test-labels",
		"ovim.io/organization-name":    "test-labels",
	}

	mockOpenShiftClient.On("CreateNamespace", mock.Anything, "test-labels", mock.MatchedBy(func(labels map[string]string) bool {
		// Verify required labels are present
		for key, expectedValue := range expectedLabels {
			if value, exists := labels[key]; !exists || value != expectedValue {
				return false
			}
		}
		return true
	}), mock.MatchedBy(func(annotations map[string]string) bool {
		// Verify required annotations are present
		requiredAnnotations := []string{
			"ovim.io/organization-description",
			"ovim.io/created-by",
			"ovim.io/created-at",
		}
		for _, key := range requiredAnnotations {
			if _, exists := annotations[key]; !exists {
				return false
			}
		}
		return true
	})).Return(nil)

	mockOpenShiftClient.On("CreateResourceQuota", mock.Anything, "test-labels", 10, 20, 100).Return(nil)

	handlers := &OrganizationHandlers{
		storage:         mockStorage,
		openshiftClient: mockOpenShiftClient,
	}

	requestBody := models.CreateOrganizationRequest{
		Name:         "Test Labels",
		Description:  "Test organization for labels",
		IsEnabled:    true,
		CPUQuota:     intPtr(10),
		MemoryQuota:  intPtr(20),
		StorageQuota: intPtr(100),
	}

	c, w := setupGinContext("POST", "/organizations", requestBody, "user-123", "admin", models.RoleSystemAdmin, "")

	handlers.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockStorage.AssertExpectations(t)
	mockOpenShiftClient.AssertExpectations(t)
}

func TestOrganizationHandlers_Delete_WithCascadeResources(t *testing.T) {
	tests := []struct {
		name                  string
		organizationID        string
		userID                string
		username              string
		role                  string
		orgID                 string
		existingVMs           []*models.VirtualMachine
		existingVDCs          []*models.VirtualDataCenter
		existingTemplates     []*models.Template
		existingUsers         []*models.User
		mockStorageBehavior   func(*MockStorage)
		mockOpenShiftBehavior func(*MockOpenShiftClient)
		expectedStatus        int
		description           string
	}{
		{
			name:           "Successful organization deletion with cascade resources",
			organizationID: "test-org-cascade",
			userID:         "user-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			existingVMs: []*models.VirtualMachine{
				{ID: "vm-1", Name: "Test VM 1", OrgID: "test-org-cascade"},
				{ID: "vm-2", Name: "Test VM 2", OrgID: "test-org-cascade"},
			},
			existingVDCs: []*models.VirtualDataCenter{
				{ID: "vdc-1", Name: "Test VDC 1", OrgID: "test-org-cascade"},
				{ID: "vdc-2", Name: "Test VDC 2", OrgID: "test-org-cascade"},
			},
			existingTemplates: []*models.Template{
				{ID: "template-1", Name: "Test Template 1", OrgID: "test-org-cascade"},
			},
			existingUsers: []*models.User{
				{ID: "user-1", Username: "orguser1", OrgID: stringPtr("test-org-cascade")},
				{ID: "user-2", Username: "orguser2", OrgID: stringPtr("test-org-cascade")},
			},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "test-org-cascade",
					Name:      "Test Organization Cascade",
					Namespace: "test-org-cascade",
				}
				ms.On("GetOrganization", "test-org-cascade").Return(org, nil)

				// Mock cascade deletion calls
				ms.On("ListVMs", "test-org-cascade").Return([]*models.VirtualMachine{
					{ID: "vm-1", Name: "Test VM 1", OrgID: "test-org-cascade"},
					{ID: "vm-2", Name: "Test VM 2", OrgID: "test-org-cascade"},
				}, nil)
				ms.On("DeleteVM", "vm-1").Return(nil)
				ms.On("DeleteVM", "vm-2").Return(nil)

				ms.On("ListVDCs", "test-org-cascade").Return([]*models.VirtualDataCenter{
					{ID: "vdc-1", Name: "Test VDC 1", OrgID: "test-org-cascade"},
					{ID: "vdc-2", Name: "Test VDC 2", OrgID: "test-org-cascade"},
				}, nil)
				ms.On("DeleteVDC", "vdc-1").Return(nil)
				ms.On("DeleteVDC", "vdc-2").Return(nil)

				ms.On("ListTemplatesByOrg", "test-org-cascade").Return([]*models.Template{
					{ID: "template-1", Name: "Test Template 1", OrgID: "test-org-cascade"},
				}, nil)
				ms.On("DeleteTemplate", "template-1").Return(nil)

				ms.On("ListUsersByOrg", "test-org-cascade").Return([]*models.User{
					{ID: "user-1", Username: "orguser1", OrgID: stringPtr("test-org-cascade")},
					{ID: "user-2", Username: "orguser2", OrgID: stringPtr("test-org-cascade")},
				}, nil)
				ms.On("UpdateUser", mock.MatchedBy(func(user *models.User) bool {
					return user.ID == "user-1" && user.OrgID == nil
				})).Return(nil)
				ms.On("UpdateUser", mock.MatchedBy(func(user *models.User) bool {
					return user.ID == "user-2" && user.OrgID == nil
				})).Return(nil)

				ms.On("DeleteOrganization", "test-org-cascade").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "test-org-cascade").Return(true, nil)
				moc.On("DeleteNamespace", mock.Anything, "test-org-cascade").Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should delete organization and all attached resources",
		},
		{
			name:              "Organization deletion with empty resources",
			organizationID:    "empty-org",
			userID:            "user-123",
			username:          "admin",
			role:              models.RoleSystemAdmin,
			orgID:             "",
			existingVMs:       []*models.VirtualMachine{},
			existingVDCs:      []*models.VirtualDataCenter{},
			existingTemplates: []*models.Template{},
			existingUsers:     []*models.User{},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "empty-org",
					Name:      "Empty Organization",
					Namespace: "empty-org",
				}
				ms.On("GetOrganization", "empty-org").Return(org, nil)

				// Mock empty lists
				ms.On("ListVMs", "empty-org").Return([]*models.VirtualMachine{}, nil)
				ms.On("ListVDCs", "empty-org").Return([]*models.VirtualDataCenter{}, nil)
				ms.On("ListTemplatesByOrg", "empty-org").Return([]*models.Template{}, nil)
				ms.On("ListUsersByOrg", "empty-org").Return([]*models.User{}, nil)

				ms.On("DeleteOrganization", "empty-org").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "empty-org").Return(true, nil)
				moc.On("DeleteNamespace", mock.Anything, "empty-org").Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should handle organization with no attached resources",
		},
		{
			name:           "Organization deletion with VM deletion failure",
			organizationID: "fail-vm-org",
			userID:         "user-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			existingVMs: []*models.VirtualMachine{
				{ID: "vm-fail", Name: "Failing VM", OrgID: "fail-vm-org"},
				{ID: "vm-success", Name: "Success VM", OrgID: "fail-vm-org"},
			},
			existingVDCs:      []*models.VirtualDataCenter{},
			existingTemplates: []*models.Template{},
			existingUsers:     []*models.User{},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "fail-vm-org",
					Name:      "Fail VM Organization",
					Namespace: "fail-vm-org",
				}
				ms.On("GetOrganization", "fail-vm-org").Return(org, nil)

				ms.On("ListVMs", "fail-vm-org").Return([]*models.VirtualMachine{
					{ID: "vm-fail", Name: "Failing VM", OrgID: "fail-vm-org"},
					{ID: "vm-success", Name: "Success VM", OrgID: "fail-vm-org"},
				}, nil)
				ms.On("DeleteVM", "vm-fail").Return(errors.New("VM deletion failed"))
				ms.On("DeleteVM", "vm-success").Return(nil)

				ms.On("ListVDCs", "fail-vm-org").Return([]*models.VirtualDataCenter{}, nil)
				ms.On("ListTemplatesByOrg", "fail-vm-org").Return([]*models.Template{}, nil)
				ms.On("ListUsersByOrg", "fail-vm-org").Return([]*models.User{}, nil)

				ms.On("DeleteOrganization", "fail-vm-org").Return(nil)
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				moc.On("NamespaceExists", mock.Anything, "fail-vm-org").Return(true, nil)
				moc.On("DeleteNamespace", mock.Anything, "fail-vm-org").Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should continue deletion even if some VM deletions fail",
		},
		{
			name:           "Organization deletion with storage listing failure",
			organizationID: "fail-list-org",
			userID:         "user-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:        "fail-list-org",
					Name:      "Fail List Organization",
					Namespace: "fail-list-org",
				}
				ms.On("GetOrganization", "fail-list-org").Return(org, nil)

				// Mock listing failure
				ms.On("ListVMs", "fail-list-org").Return(nil, errors.New("storage listing failed"))
			},
			mockOpenShiftBehavior: func(moc *MockOpenShiftClient) {
				// Should not be called due to cascade failure
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should fail if resource listing fails",
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
			var handlers *OrganizationHandlers
			if mockOpenShiftClient != nil {
				handlers = &OrganizationHandlers{
					storage:         mockStorage,
					openshiftClient: mockOpenShiftClient,
				}
			} else {
				handlers = &OrganizationHandlers{
					storage:         mockStorage,
					openshiftClient: nil,
				}
			}

			// Setup request
			c, w := setupGinContext("DELETE", "/organizations/"+tt.organizationID, nil, tt.userID, tt.username, tt.role, tt.orgID)
			c.Params = gin.Params{{Key: "id", Value: tt.organizationID}}

			// Execute
			handlers.Delete(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			if mockOpenShiftClient != nil {
				mockOpenShiftClient.AssertExpectations(t)
			}

			// Additional assertions for successful deletions
			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "Organization deleted successfully", response["message"])
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}

// Test UpdateResourceQuotas endpoint
func TestOrganizationHandlers_UpdateResourceQuotas(t *testing.T) {
	tests := []struct {
		name                string
		organizationID      string
		userID              string
		username            string
		role                string
		orgID               string
		requestBody         interface{}
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
		description         string
	}{
		{
			name:           "Successful resource quota update by system admin",
			organizationID: "test-org",
			userID:         "admin-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			requestBody: map[string]interface{}{
				"cpu_quota":     20,
				"memory_quota":  40,
				"storage_quota": 200,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:           "test-org",
					Name:         "Test Organization",
					CPUQuota:     10,
					MemoryQuota:  20,
					StorageQuota: 100,
				}
				ms.On("GetOrganization", "test-org").Return(org, nil)
				ms.On("UpdateOrganization", mock.MatchedBy(func(updatedOrg *models.Organization) bool {
					return updatedOrg.CPUQuota == 20 && updatedOrg.MemoryQuota == 40 && updatedOrg.StorageQuota == 200
				})).Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should update resource quotas successfully for system admin",
		},
		{
			name:           "Forbidden update by non-system admin",
			organizationID: "test-org",
			userID:         "user-123",
			username:       "user",
			role:           models.RoleOrgAdmin,
			orgID:          "test-org",
			requestBody: map[string]interface{}{
				"cpu_quota":     20,
				"memory_quota":  40,
				"storage_quota": 200,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				// Should not be called due to permission check
			},
			expectedStatus: http.StatusForbidden,
			description:    "Should forbid quota updates for non-system admin",
		},
		{
			name:           "Organization not found",
			organizationID: "non-existent",
			userID:         "admin-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			requestBody: map[string]interface{}{
				"cpu_quota":     20,
				"memory_quota":  40,
				"storage_quota": 200,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "non-existent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			description:    "Should return not found for non-existent organization",
		},
		{
			name:           "Invalid request with negative quotas",
			organizationID: "test-org",
			userID:         "admin-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			requestBody: map[string]interface{}{
				"cpu_quota":     -5, // Negative quota
				"memory_quota":  40,
				"storage_quota": 200,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:           "test-org",
					Name:         "Test Organization",
					CPUQuota:     10,
					MemoryQuota:  20,
					StorageQuota: 100,
				}
				ms.On("GetOrganization", "test-org").Return(org, nil)
				// Should not call UpdateOrganization due to validation
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject negative resource quotas",
		},
		{
			name:           "Valid request with zero quotas",
			organizationID: "test-org",
			userID:         "admin-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			requestBody: map[string]interface{}{
				"cpu_quota":     0, // Zero quota (valid)
				"memory_quota":  0, // Zero quota (valid)
				"storage_quota": 0, // Zero quota (valid)
			},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:           "test-org",
					Name:         "Test Organization",
					CPUQuota:     10,
					MemoryQuota:  20,
					StorageQuota: 100,
				}
				ms.On("GetOrganization", "test-org").Return(org, nil)
				ms.On("UpdateOrganization", mock.MatchedBy(func(updatedOrg *models.Organization) bool {
					return updatedOrg.CPUQuota == 0 && updatedOrg.MemoryQuota == 0 && updatedOrg.StorageQuota == 0
				})).Return(nil)
			},
			expectedStatus: http.StatusOK,
			description:    "Should accept zero resource quotas (valid for unlimited)",
		},
		{
			name:           "Database update failure",
			organizationID: "test-org",
			userID:         "admin-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			requestBody: map[string]interface{}{
				"cpu_quota":     20,
				"memory_quota":  40,
				"storage_quota": 200,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:           "test-org",
					Name:         "Test Organization",
					CPUQuota:     10,
					MemoryQuota:  20,
					StorageQuota: 100,
				}
				ms.On("GetOrganization", "test-org").Return(org, nil)
				ms.On("UpdateOrganization", mock.Anything).Return(errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should handle database update failures",
		},
		{
			name:           "Invalid JSON request",
			organizationID: "test-org",
			userID:         "admin-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			requestBody:    "invalid json",
			mockStorageBehavior: func(ms *MockStorage) {
				// Should not be called due to JSON parsing error, but we still need to set up
				// a basic expectation in case GetOrganization is called during validation
				org := &models.Organization{
					ID:           "test-org",
					Name:         "Test Organization",
					CPUQuota:     10,
					MemoryQuota:  20,
					StorageQuota: 100,
				}
				ms.On("GetOrganization", "test-org").Return(org, nil).Maybe()
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject invalid JSON request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			// Create handlers
			handlers := &OrganizationHandlers{
				storage:         mockStorage,
				openshiftClient: nil,
			}

			// Setup request
			c, w := setupGinContext("PUT", "/organizations/"+tt.organizationID+"/quotas", tt.requestBody, tt.userID, tt.username, tt.role, tt.orgID)
			c.Params = gin.Params{{Key: "id", Value: tt.organizationID}}

			// Execute
			handlers.UpdateResourceQuotas(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)

			// Additional assertions for successful updates
			if tt.expectedStatus == http.StatusOK {
				var response models.Organization
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.ID)
			}
		})
	}
}

// Test ValidateResourceAllocation endpoint
func TestOrganizationHandlers_ValidateResourceAllocation(t *testing.T) {
	tests := []struct {
		name                string
		organizationID      string
		userID              string
		username            string
		role                string
		orgID               string
		requestBody         interface{}
		mockStorageBehavior func(*MockStorage)
		expectedStatus      int
		expectedAllocation  *bool // nil means don't check
		description         string
	}{
		{
			name:           "Valid allocation request within limits",
			organizationID: "test-org",
			userID:         "user-123",
			username:       "user",
			role:           models.RoleOrgUser,
			orgID:          "test-org",
			requestBody: map[string]interface{}{
				"cpu_request":     5,
				"memory_request":  10,
				"storage_request": 50,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:           "test-org",
					Name:         "Test Organization",
					CPUQuota:     20,
					MemoryQuota:  40,
					StorageQuota: 200,
				}
				ms.On("GetOrganization", "test-org").Return(org, nil)
				ms.On("ListVDCs", "test-org").Return([]*models.VirtualDataCenter{}, nil) // No existing VDCs
			},
			expectedStatus:     http.StatusOK,
			expectedAllocation: boolPtr(true),
			description:        "Should allow allocation within quota limits",
		},
		{
			name:           "Invalid allocation request exceeding limits",
			organizationID: "test-org",
			userID:         "user-123",
			username:       "user",
			role:           models.RoleOrgUser,
			orgID:          "test-org",
			requestBody: map[string]interface{}{
				"cpu_request":     25, // Exceeds quota of 20
				"memory_request":  10,
				"storage_request": 50,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:           "test-org",
					Name:         "Test Organization",
					CPUQuota:     20,
					MemoryQuota:  40,
					StorageQuota: 200,
				}
				ms.On("GetOrganization", "test-org").Return(org, nil)
				ms.On("ListVDCs", "test-org").Return([]*models.VirtualDataCenter{}, nil) // No existing VDCs
			},
			expectedStatus:     http.StatusOK,
			expectedAllocation: boolPtr(false),
			description:        "Should deny allocation exceeding quota limits",
		},
		{
			name:           "Forbidden validation for different organization",
			organizationID: "other-org",
			userID:         "user-123",
			username:       "user",
			role:           models.RoleOrgUser,
			orgID:          "test-org", // User belongs to different org
			requestBody: map[string]interface{}{
				"cpu_request":     5,
				"memory_request":  10,
				"storage_request": 50,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				// Should not be called due to permission check
			},
			expectedStatus: http.StatusForbidden,
			description:    "Should forbid validation for different organization",
		},
		{
			name:           "System admin can validate any organization",
			organizationID: "other-org",
			userID:         "admin-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "test-org", // Admin belongs to different org but has system role
			requestBody: map[string]interface{}{
				"cpu_request":     5,
				"memory_request":  10,
				"storage_request": 50,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				org := &models.Organization{
					ID:           "other-org",
					Name:         "Other Organization",
					CPUQuota:     20,
					MemoryQuota:  40,
					StorageQuota: 200,
				}
				ms.On("GetOrganization", "other-org").Return(org, nil)
				ms.On("ListVDCs", "other-org").Return([]*models.VirtualDataCenter{}, nil)
			},
			expectedStatus:     http.StatusOK,
			expectedAllocation: boolPtr(true),
			description:        "Should allow system admin to validate any organization",
		},
		{
			name:           "Organization not found",
			organizationID: "non-existent",
			userID:         "admin-123",
			username:       "admin",
			role:           models.RoleSystemAdmin,
			orgID:          "",
			requestBody: map[string]interface{}{
				"cpu_request":     5,
				"memory_request":  10,
				"storage_request": 50,
			},
			mockStorageBehavior: func(ms *MockStorage) {
				ms.On("GetOrganization", "non-existent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			description:    "Should return not found for non-existent organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			// Create handlers
			handlers := &OrganizationHandlers{
				storage:         mockStorage,
				openshiftClient: nil,
			}

			// Setup request
			c, w := setupGinContext("POST", "/organizations/"+tt.organizationID+"/validate-allocation", tt.requestBody, tt.userID, tt.username, tt.role, tt.orgID)
			c.Params = gin.Params{{Key: "id", Value: tt.organizationID}}

			// Execute
			handlers.ValidateResourceAllocation(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)

			// Additional assertions for successful validations
			if tt.expectedStatus == http.StatusOK && tt.expectedAllocation != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, *tt.expectedAllocation, response["can_allocate"])
				assert.Contains(t, response, "requested")
				assert.Contains(t, response, "current_usage")
			}
		})
	}
}
