package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)


// MockOIDCProvider for testing OIDC functionality
type MockOIDCProvider struct {
	mock.Mock
}

func (m *MockOIDCProvider) GenerateState() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockOIDCProvider) GetAuthURL(state string) string {
	args := m.Called(state)
	return args.String(0)
}

func (m *MockOIDCProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2.Token), args.Error(1)
}

func (m *MockOIDCProvider) VerifyIDToken(ctx context.Context, rawIDToken string) (*auth.IDToken, error) {
	args := m.Called(ctx, rawIDToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.IDToken), args.Error(1)
}

func (m *MockOIDCProvider) GetUserInfo(ctx context.Context, idToken *auth.IDToken) (*auth.UserInfo, error) {
	args := m.Called(ctx, idToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.UserInfo), args.Error(1)
}

func (m *MockOIDCProvider) MapOIDCRolesToOVIM(userInfo *auth.UserInfo) string {
	args := m.Called(userInfo)
	return args.String(0)
}

func TestAuthHandlers_Login(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		request        LoginRequest
		setupMocks     func(*MockStorage, *auth.TokenManager)
		expectedStatus int
		expectToken    bool
		expectUser     bool
	}{
		{
			name: "successful login",
			request: LoginRequest{
				Username: "testuser",
				Password: "password123",
			},
			setupMocks: func(mockStorage *MockStorage, tokenManager *auth.TokenManager) {
				passwordHash, _ := auth.HashPassword("password123")
				user := &models.User{
					ID:           "user-1",
					Username:     "testuser",
					Email:        "test@example.com",
					Role:         "user",
					PasswordHash: passwordHash,
				}
				mockStorage.On("GetUserByUsername", "testuser").Return(user, nil)
			},
			expectedStatus: http.StatusOK,
			expectToken:    true,
			expectUser:     true,
		},
		{
			name: "invalid credentials - user not found",
			request: LoginRequest{
				Username: "nonexistent",
				Password: "password123",
			},
			setupMocks: func(mockStorage *MockStorage, tokenManager *auth.TokenManager) {
				mockStorage.On("GetUserByUsername", "nonexistent").Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
			expectUser:     false,
		},
		{
			name: "invalid credentials - wrong password",
			request: LoginRequest{
				Username: "testuser",
				Password: "wrongpassword",
			},
			setupMocks: func(mockStorage *MockStorage, tokenManager *auth.TokenManager) {
				passwordHash, _ := auth.HashPassword("correctpassword")
				user := &models.User{
					ID:           "user-1",
					Username:     "testuser",
					Email:        "test@example.com",
					Role:         "user",
					PasswordHash: passwordHash,
				}
				mockStorage.On("GetUserByUsername", "testuser").Return(user, nil)
			},
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
			expectUser:     false,
		},
		{
			name: "invalid request format",
			request: LoginRequest{
				Username: "", // Missing required field
				Password: "password123",
			},
			setupMocks:     func(*MockStorage, *auth.TokenManager) {},
			expectedStatus: http.StatusBadRequest,
			expectToken:    false,
			expectUser:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tokenManager := auth.NewTokenManager("test-secret", 24*time.Hour)

			tt.setupMocks(mockStorage, tokenManager)

			handlers := NewAuthHandlers(mockStorage, tokenManager, nil)

			// Create request
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handlers.Login(c)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectToken || tt.expectUser {
				var response LoginResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				if tt.expectToken {
					assert.NotEmpty(t, response.Token)
				}

				if tt.expectUser {
					assert.NotNil(t, response.User)
					assert.NotEmpty(t, response.User.Username)
					assert.Empty(t, response.User.PasswordHash) // Should not return password hash
				}
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestAuthHandlers_Logout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStorage := new(MockStorage)
	tokenManager := auth.NewTokenManager("test-secret", 24*time.Hour)
	handlers := NewAuthHandlers(mockStorage, tokenManager, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handlers.Logout(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Logout successful", response["message"])
}

func TestAuthHandlers_GetOIDCAuthURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		oidcProvider   *MockOIDCProvider
		expectedStatus int
		expectAuthURL  bool
	}{
		{
			name: "OIDC enabled",
			oidcProvider: func() *MockOIDCProvider {
				mock := new(MockOIDCProvider)
				mock.On("GenerateState").Return("test-state-123")
				mock.On("GetAuthURL", "test-state-123").Return("https://oidc.example.com/auth?state=test-state-123")
				return mock
			}(),
			expectedStatus: http.StatusOK,
			expectAuthURL:  true,
		},
		{
			name:           "OIDC not configured",
			oidcProvider:   nil,
			expectedStatus: http.StatusNotImplemented,
			expectAuthURL:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tokenManager := auth.NewTokenManager("test-secret", 24*time.Hour)

			var oidcProvider *auth.OIDCProvider
			if tt.oidcProvider != nil {
				// In a real test, you'd need to properly mock the OIDC provider
				// For now, we'll test the nil case
			}

			handlers := NewAuthHandlers(mockStorage, tokenManager, oidcProvider)

			req := httptest.NewRequest(http.MethodGet, "/auth/oidc/auth-url", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetOIDCAuthURL(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectAuthURL {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "auth_url")
				assert.Contains(t, response, "state")
			}

			if tt.oidcProvider != nil {
				tt.oidcProvider.AssertExpectations(t)
			}
		})
	}
}

func TestAuthHandlers_GetAuthInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		oidcProvider *auth.OIDCProvider
		expectOIDC   bool
	}{
		{
			name:         "with OIDC provider",
			oidcProvider: &auth.OIDCProvider{}, // Not nil
			expectOIDC:   true,
		},
		{
			name:         "without OIDC provider",
			oidcProvider: nil,
			expectOIDC:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tokenManager := auth.NewTokenManager("test-secret", 24*time.Hour)
			handlers := NewAuthHandlers(mockStorage, tokenManager, tt.oidcProvider)

			req := httptest.NewRequest(http.MethodGet, "/auth/info", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetAuthInfo(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, true, response["local_auth_enabled"])
			assert.Equal(t, tt.expectOIDC, response["oidc_enabled"])
		})
	}
}

func TestNewAuthHandlers(t *testing.T) {
	mockStorage := new(MockStorage)
	tokenManager := auth.NewTokenManager("test-secret", 24*time.Hour)
	oidcProvider := &auth.OIDCProvider{}

	handlers := NewAuthHandlers(mockStorage, tokenManager, oidcProvider)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockStorage, handlers.storage)
	assert.Equal(t, tokenManager, handlers.tokenManager)
	assert.Equal(t, oidcProvider, handlers.oidcProvider)
}

func TestLoginRequest_Validation(t *testing.T) {
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
				"password": "password123",
			},
			valid: true,
		},
		{
			name: "missing username",
			request: map[string]interface{}{
				"password": "password123",
			},
			valid: false,
		},
		{
			name: "missing password",
			request: map[string]interface{}{
				"username": "testuser",
			},
			valid: false,
		},
		{
			name: "empty username",
			request: map[string]interface{}{
				"username": "",
				"password": "password123",
			},
			valid: false,
		},
		{
			name: "empty password",
			request: map[string]interface{}{
				"username": "testuser",
				"password": "",
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

			var loginReq LoginRequest
			err := c.ShouldBindJSON(&loginReq)

			if tt.valid {
				assert.NoError(t, err)
				assert.NotEmpty(t, loginReq.Username)
				assert.NotEmpty(t, loginReq.Password)
			} else {
				assert.Error(t, err)
			}
		})
	}
}