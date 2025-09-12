package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

func TestNewOrganizationHandlers(t *testing.T) {
	mockStorage := &MockStorage{}

	handlers := NewOrganizationHandlers(mockStorage, nil)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
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
				Name:        "Test Organization",
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
				mk.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			description:    "Should create organization CRD successfully",
		},
		{
			name: "Organization creation fails",
			requestBody: models.CreateOrganizationRequest{
				Name:        "Failed Organization",
				DisplayName: "Failed Organization",
				Description: "Test organization that fails",
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
				mk.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Should handle CRD creation failures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := &MockStorage{}
			mockK8sClient := &MockK8sClient{}

			if tt.mockStorageBehavior != nil {
				tt.mockStorageBehavior(mockStorage)
			}

			if tt.mockK8sBehavior != nil {
				tt.mockK8sBehavior(mockK8sClient)
			}

			// Create handlers
			handlers := NewOrganizationHandlers(mockStorage, mockK8sClient)

			// Setup request
			c, w := setupGinContext("POST", "/organizations", tt.requestBody, tt.userID, tt.username, tt.role, tt.orgID)

			// Execute
			handlers.Create(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify mock expectations
			mockStorage.AssertExpectations(t)
			mockK8sClient.AssertExpectations(t)

			// Additional assertions for successful creation
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
