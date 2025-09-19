package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

func TestUserHandlers_List(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMocks     func(*MockStorage)
		expectedStatus int
		expectUsers    bool
	}{
		{
			name: "successful list",
			setupMocks: func(mockStorage *MockStorage) {
				users := []*models.User{
					{
						ID:       "user1",
						Username: "testuser1",
						Email:    "test1@example.com",
						Role:     "user",
					},
					{
						ID:       "user2",
						Username: "testuser2",
						Email:    "test2@example.com",
						Role:     "org_admin",
					},
				}
				mockStorage.On("ListUsers").Return(users, nil)
			},
			expectedStatus: http.StatusOK,
			expectUsers:    true,
		},
		{
			name: "storage error",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("ListUsers").Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectUsers:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewUserHandlers(mockStorage)

			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.List(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectUsers {
				var users []*models.User
				err := json.Unmarshal(w.Body.Bytes(), &users)
				require.NoError(t, err)
				assert.Len(t, users, 2)
				// Ensure password hashes are not returned
				for _, user := range users {
					assert.Empty(t, user.PasswordHash)
				}
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestUserHandlers_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		request        CreateUserRequest
		setupMocks     func(*MockStorage)
		expectedStatus int
		expectUser     bool
	}{
		{
			name: "successful creation",
			request: CreateUserRequest{
				Username: "newuser",
				Email:    "newuser@example.com",
				Password: "password123",
				Role:     "user",
			},
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("GetUserByUsername", "newuser").Return(nil, storage.ErrNotFound)
				mockStorage.On("CreateUser", mock.MatchedBy(func(user *models.User) bool {
					return user.Username == "newuser" && user.Email == "newuser@example.com"
				})).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			expectUser:     true,
		},
		{
			name: "username already exists",
			request: CreateUserRequest{
				Username: "existinguser",
				Email:    "existing@example.com",
				Password: "password123",
				Role:     "user",
			},
			setupMocks: func(mockStorage *MockStorage) {
				existingUser := &models.User{
					ID:       "existing1",
					Username: "existinguser",
					Email:    "existing@example.com",
				}
				mockStorage.On("GetUserByUsername", "existinguser").Return(existingUser, nil)
			},
			expectedStatus: http.StatusConflict,
			expectUser:     false,
		},
		{
			name: "invalid role",
			request: CreateUserRequest{
				Username: "newuser",
				Email:    "newuser@example.com",
				Password: "password123",
				Role:     "invalid_role",
			},
			setupMocks:     func(*MockStorage) {},
			expectedStatus: http.StatusBadRequest,
			expectUser:     false,
		},
		{
			name: "invalid email format",
			request: CreateUserRequest{
				Username: "newuser",
				Email:    "invalid-email",
				Password: "password123",
				Role:     "user",
			},
			setupMocks:     func(*MockStorage) {},
			expectedStatus: http.StatusBadRequest,
			expectUser:     false,
		},
		{
			name: "weak password",
			request: CreateUserRequest{
				Username: "newuser",
				Email:    "newuser@example.com",
				Password: "123",
				Role:     "user",
			},
			setupMocks:     func(*MockStorage) {},
			expectedStatus: http.StatusBadRequest,
			expectUser:     false,
		},
		{
			name: "missing required fields",
			request: CreateUserRequest{
				Username: "",
				Email:    "test@example.com",
				Password: "password123",
				Role:     "user",
			},
			setupMocks:     func(*MockStorage) {},
			expectedStatus: http.StatusBadRequest,
			expectUser:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewUserHandlers(mockStorage)

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.Create(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectUser {
				var user models.User
				err := json.Unmarshal(w.Body.Bytes(), &user)
				require.NoError(t, err)
				assert.Equal(t, tt.request.Username, user.Username)
				assert.Equal(t, tt.request.Email, user.Email)
				assert.Equal(t, tt.request.Role, user.Role)
				assert.Empty(t, user.PasswordHash) // Should not return password hash
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestUserHandlers_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         string
		setupMocks     func(*MockStorage)
		expectedStatus int
		expectUser     bool
	}{
		{
			name:   "successful get",
			userID: "user1",
			setupMocks: func(mockStorage *MockStorage) {
				user := &models.User{
					ID:           "user1",
					Username:     "testuser",
					Email:        "test@example.com",
					Role:         "user",
					PasswordHash: "hashedpassword",
				}
				mockStorage.On("GetUser", "user1").Return(user, nil)
			},
			expectedStatus: http.StatusOK,
			expectUser:     true,
		},
		{
			name:   "user not found",
			userID: "nonexistent",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("GetUser", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectUser:     false,
		},
		{
			name:   "storage error",
			userID: "user1",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("GetUser", "user1").Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectUser:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewUserHandlers(mockStorage)

			req := httptest.NewRequest(http.MethodGet, "/users/"+tt.userID, nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{
				{Key: "id", Value: tt.userID},
			}

			handlers.Get(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectUser {
				var user models.User
				err := json.Unmarshal(w.Body.Bytes(), &user)
				require.NoError(t, err)
				assert.Equal(t, tt.userID, user.ID)
				assert.Empty(t, user.PasswordHash) // Should not return password hash
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestUserHandlers_Update(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         string
		request        UpdateUserRequest
		setupMocks     func(*MockStorage)
		expectedStatus int
		expectUser     bool
	}{
		{
			name:   "successful update",
			userID: "user1",
			request: UpdateUserRequest{
				Username: "updateduser",
				Email:    "updated@example.com",
				Role:     "org_admin",
			},
			setupMocks: func(mockStorage *MockStorage) {
				existingUser := &models.User{
					ID:           "user1",
					Username:     "olduser",
					Email:        "old@example.com",
					Role:         "user",
					PasswordHash: "hashedpassword",
					CreatedAt:    time.Now().Add(-24 * time.Hour),
					UpdatedAt:    time.Now().Add(-1 * time.Hour),
				}
				mockStorage.On("GetUser", "user1").Return(existingUser, nil)
				mockStorage.On("GetUserByUsername", "updateduser").Return(nil, storage.ErrNotFound)
				mockStorage.On("UpdateUser", mock.MatchedBy(func(user *models.User) bool {
					return user.ID == "user1" && user.Username == "updateduser" && user.Role == "org_admin"
				})).Return(nil)
			},
			expectedStatus: http.StatusOK,
			expectUser:     true,
		},
		{
			name:   "user not found",
			userID: "nonexistent",
			request: UpdateUserRequest{
				Username: "updated",
				Email:    "updated@example.com",
			},
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("GetUser", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectUser:     false,
		},
		{
			name:   "username conflict",
			userID: "user1",
			request: UpdateUserRequest{
				Username: "existinguser",
			},
			setupMocks: func(mockStorage *MockStorage) {
				currentUser := &models.User{
					ID:       "user1",
					Username: "currentuser",
				}
				conflictUser := &models.User{
					ID:       "user2",
					Username: "existinguser",
				}
				mockStorage.On("GetUser", "user1").Return(currentUser, nil)
				mockStorage.On("GetUserByUsername", "existinguser").Return(conflictUser, nil)
			},
			expectedStatus: http.StatusConflict,
			expectUser:     false,
		},
		{
			name:   "invalid role",
			userID: "user1",
			request: UpdateUserRequest{
				Role: "invalid_role",
			},
			setupMocks: func(mockStorage *MockStorage) {
				existingUser := &models.User{
					ID:   "user1",
					Role: "user",
				}
				mockStorage.On("GetUser", "user1").Return(existingUser, nil)
			},
			expectedStatus: http.StatusBadRequest,
			expectUser:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewUserHandlers(mockStorage)

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, "/users/"+tt.userID, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{
				{Key: "id", Value: tt.userID},
			}

			handlers.Update(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectUser {
				var user models.User
				err := json.Unmarshal(w.Body.Bytes(), &user)
				require.NoError(t, err)
				assert.Equal(t, tt.userID, user.ID)
				assert.Empty(t, user.PasswordHash) // Should not return password hash
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestUserHandlers_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         string
		setupMocks     func(*MockStorage)
		expectedStatus int
	}{
		{
			name:   "successful delete",
			userID: "user1",
			setupMocks: func(mockStorage *MockStorage) {
				user := &models.User{
					ID:       "user1",
					Username: "testuser",
				}
				mockStorage.On("GetUser", "user1").Return(user, nil)
				mockStorage.On("DeleteUser", "user1").Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:   "user not found",
			userID: "nonexistent",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("GetUser", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "storage error on delete",
			userID: "user1",
			setupMocks: func(mockStorage *MockStorage) {
				user := &models.User{
					ID:       "user1",
					Username: "testuser",
				}
				mockStorage.On("GetUser", "user1").Return(user, nil)
				mockStorage.On("DeleteUser", "user1").Return(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewUserHandlers(mockStorage)

			req := httptest.NewRequest(http.MethodDelete, "/users/"+tt.userID, nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{
				{Key: "id", Value: tt.userID},
			}

			handlers.Delete(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestUserHandlers_ListByOrganization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		orgID          string
		setupMocks     func(*MockStorage)
		expectedStatus int
		expectUsers    bool
	}{
		{
			name:  "successful list by organization",
			orgID: "org1",
			setupMocks: func(mockStorage *MockStorage) {
				org := &models.Organization{
					ID:   "org1",
					Name: "Test Org",
				}
				users := []*models.User{
					{
						ID:       "user1",
						Username: "orguser1",
						OrgID:    &[]string{"org1"}[0],
					},
					{
						ID:       "user2",
						Username: "orguser2",
						OrgID:    &[]string{"org1"}[0],
					},
				}
				mockStorage.On("GetOrganization", "org1").Return(org, nil)
				mockStorage.On("ListUsers").Return(users, nil)
			},
			expectedStatus: http.StatusOK,
			expectUsers:    true,
		},
		{
			name:  "organization not found",
			orgID: "nonexistent",
			setupMocks: func(mockStorage *MockStorage) {
				mockStorage.On("GetOrganization", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectUsers:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMocks(mockStorage)

			handlers := NewUserHandlers(mockStorage)

			req := httptest.NewRequest(http.MethodGet, "/organizations/"+tt.orgID+"/users", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{
				{Key: "id", Value: tt.orgID},
			}

			handlers.ListByOrganization(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectUsers {
				var users []*models.User
				err := json.Unmarshal(w.Body.Bytes(), &users)
				require.NoError(t, err)
				// All users should belong to the organization
				for _, user := range users {
					assert.NotNil(t, user.OrgID)
					assert.Equal(t, tt.orgID, *user.OrgID)
					assert.Empty(t, user.PasswordHash)
				}
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestNewUserHandlers(t *testing.T) {
	mockStorage := new(MockStorage)
	handlers := NewUserHandlers(mockStorage)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
}

func TestCreateUserRequest_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		request map[string]interface{}
		valid   bool
	}{
		{
			name: "valid request",
			request: map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
				"password": "password123",
				"role":     "user",
			},
			valid: true,
		},
		{
			name: "missing username",
			request: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
				"role":     "user",
			},
			valid: false,
		},
		{
			name: "missing email",
			request: map[string]interface{}{
				"username": "testuser",
				"password": "password123",
				"role":     "user",
			},
			valid: false,
		},
		{
			name: "missing password",
			request: map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
				"role":     "user",
			},
			valid: false,
		},
		{
			name: "missing role",
			request: map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
				"password": "password123",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			var createReq CreateUserRequest
			err := c.ShouldBindJSON(&createReq)

			if tt.valid {
				assert.NoError(t, err)
				assert.NotEmpty(t, createReq.Username)
				assert.NotEmpty(t, createReq.Email)
				assert.NotEmpty(t, createReq.Password)
				assert.NotEmpty(t, createReq.Role)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestUpdateUserRequest_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// UpdateUserRequest doesn't have required fields, so all should be valid
	request := map[string]interface{}{
		"username": "updateduser",
		"email":    "updated@example.com",
		"role":     "org_admin",
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	var updateReq UpdateUserRequest
	err := c.ShouldBindJSON(&updateReq)

	assert.NoError(t, err)
	assert.Equal(t, "updateduser", updateReq.Username)
	assert.Equal(t, "updated@example.com", updateReq.Email)
	assert.Equal(t, "org_admin", updateReq.Role)
}
