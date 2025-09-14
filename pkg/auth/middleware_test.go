package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestGin() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestNewMiddleware(t *testing.T) {
	tm := NewTokenManager("secret", time.Hour)
	middleware := NewMiddleware(tm)
	assert.Equal(t, tm, middleware.tokenManager)
}

func TestMiddleware_RequireAuth(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)
	middleware := NewMiddleware(tm)

	// Generate a valid token for testing
	validToken, err := tm.GenerateToken("user-123", "testuser", "user", "org-456")
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectAbort    bool
	}{
		{
			name:           "ValidToken",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
			expectAbort:    false,
		},
		{
			name:           "MissingHeader",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectAbort:    true,
		},
		{
			name:           "InvalidHeaderFormat",
			authHeader:     "InvalidFormat " + validToken,
			expectedStatus: http.StatusUnauthorized,
			expectAbort:    true,
		},
		{
			name:           "MissingBearerPrefix",
			authHeader:     validToken,
			expectedStatus: http.StatusUnauthorized,
			expectAbort:    true,
		},
		{
			name:           "InvalidToken",
			authHeader:     "Bearer invalid.token.string",
			expectedStatus: http.StatusUnauthorized,
			expectAbort:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestGin()

			handlerCalled := false
			router.GET("/test", middleware.RequireAuth(), func(c *gin.Context) {
				handlerCalled = true

				if !tt.expectAbort {
					// Verify context values are set correctly
					userID, exists := c.Get(ContextKeyUserID)
					assert.True(t, exists)
					assert.Equal(t, "user-123", userID)

					username, exists := c.Get(ContextKeyUsername)
					assert.True(t, exists)
					assert.Equal(t, "testuser", username)

					role, exists := c.Get(ContextKeyRole)
					assert.True(t, exists)
					assert.Equal(t, "user", role)

					orgID, exists := c.Get(ContextKeyOrgID)
					assert.True(t, exists)
					assert.Equal(t, "org-456", orgID)
				}

				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set(AuthorizationHeader, tt.authHeader)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, !tt.expectAbort, handlerCalled)
		})
	}
}

func TestMiddleware_RequireRole(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)
	middleware := NewMiddleware(tm)

	tests := []struct {
		name           string
		userRole       string
		allowedRoles   []string
		expectedStatus int
		expectAbort    bool
	}{
		{
			name:           "AllowedRole",
			userRole:       "admin",
			allowedRoles:   []string{"admin", "user"},
			expectedStatus: http.StatusOK,
			expectAbort:    false,
		},
		{
			name:           "RoleNotAllowed",
			userRole:       "user",
			allowedRoles:   []string{"admin"},
			expectedStatus: http.StatusForbidden,
			expectAbort:    true,
		},
		{
			name:           "MultipleAllowedRoles",
			userRole:       "moderator",
			allowedRoles:   []string{"admin", "moderator", "staff"},
			expectedStatus: http.StatusOK,
			expectAbort:    false,
		},
		{
			name:           "EmptyAllowedRoles",
			userRole:       "admin",
			allowedRoles:   []string{},
			expectedStatus: http.StatusForbidden,
			expectAbort:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestGin()

			handlerCalled := false
			router.GET("/test", func(c *gin.Context) {
				// Set role in context (simulating auth middleware)
				c.Set(ContextKeyRole, tt.userRole)
				c.Next()
			}, middleware.RequireRole(tt.allowedRoles...), func(c *gin.Context) {
				handlerCalled = true
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, !tt.expectAbort, handlerCalled)
		})
	}

	t.Run("RoleNotInContext", func(t *testing.T) {
		router := setupTestGin()

		handlerCalled := false
		router.GET("/test", middleware.RequireRole("admin"), func(c *gin.Context) {
			handlerCalled = true
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.False(t, handlerCalled)
	})
}

func TestMiddleware_RequireOrgAccess(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)
	middleware := NewMiddleware(tm)

	tests := []struct {
		name           string
		userRole       string
		userOrgID      string
		requestedOrgID string
		paramType      string // "param" or "query"
		expectedStatus int
		expectAbort    bool
	}{
		{
			name:           "SystemAdminAccess",
			userRole:       "system_admin",
			userOrgID:      "org-1",
			requestedOrgID: "org-2",
			paramType:      "param",
			expectedStatus: http.StatusOK,
			expectAbort:    false,
		},
		{
			name:           "OrgUserSameOrg",
			userRole:       "org_user",
			userOrgID:      "org-1",
			requestedOrgID: "org-1",
			paramType:      "param",
			expectedStatus: http.StatusOK,
			expectAbort:    false,
		},
		{
			name:           "OrgUserDifferentOrg",
			userRole:       "org_user",
			userOrgID:      "org-1",
			requestedOrgID: "org-2",
			paramType:      "param",
			expectedStatus: http.StatusForbidden,
			expectAbort:    true,
		},
		{
			name:           "QueryParameter",
			userRole:       "org_admin",
			userOrgID:      "org-1",
			requestedOrgID: "org-1",
			paramType:      "query",
			expectedStatus: http.StatusOK,
			expectAbort:    false,
		},
		{
			name:           "NoOrgIDRequested",
			userRole:       "org_user",
			userOrgID:      "org-1",
			requestedOrgID: "",
			paramType:      "query",
			expectedStatus: http.StatusOK,
			expectAbort:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestGin()

			handlerCalled := false
			var route string

			if tt.paramType == "param" {
				route = "/test/:org_id"
				router.GET(route, func(c *gin.Context) {
					// Set user context
					c.Set(ContextKeyRole, tt.userRole)
					c.Set(ContextKeyOrgID, tt.userOrgID)
					c.Next()
				}, middleware.RequireOrgAccess(), func(c *gin.Context) {
					handlerCalled = true
					c.JSON(http.StatusOK, gin.H{"message": "success"})
				})
			} else {
				route = "/test"
				router.GET(route, func(c *gin.Context) {
					// Set user context
					c.Set(ContextKeyRole, tt.userRole)
					c.Set(ContextKeyOrgID, tt.userOrgID)
					c.Next()
				}, middleware.RequireOrgAccess(), func(c *gin.Context) {
					handlerCalled = true
					c.JSON(http.StatusOK, gin.H{"message": "success"})
				})
			}

			var reqURL string
			if tt.paramType == "param" && tt.requestedOrgID != "" {
				reqURL = "/test/" + tt.requestedOrgID
			} else if tt.paramType == "query" && tt.requestedOrgID != "" {
				reqURL = "/test?org_id=" + tt.requestedOrgID
			} else {
				reqURL = "/test"
			}

			req := httptest.NewRequest("GET", reqURL, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, !tt.expectAbort, handlerCalled)
		})
	}
}

func TestGetUserFromContext(t *testing.T) {
	router := setupTestGin()

	router.GET("/test", func(c *gin.Context) {
		c.Set(ContextKeyUserID, "user-123")
		c.Set(ContextKeyUsername, "testuser")
		c.Set(ContextKeyRole, "admin")
		c.Set(ContextKeyOrgID, "org-456")

		userID, username, role, orgID, ok := GetUserFromContext(c)
		assert.True(t, ok)
		assert.Equal(t, "user-123", userID)
		assert.Equal(t, "testuser", username)
		assert.Equal(t, "admin", role)
		assert.Equal(t, "org-456", orgID)

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	t.Run("MissingContext", func(t *testing.T) {
		router := setupTestGin()

		router.GET("/test", func(c *gin.Context) {
			// Don't set any context values
			userID, username, role, orgID, ok := GetUserFromContext(c)
			assert.False(t, ok)
			assert.Empty(t, userID)
			assert.Empty(t, username)
			assert.Empty(t, role)
			assert.Empty(t, orgID)

			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MissingOrgID", func(t *testing.T) {
		router := setupTestGin()

		router.GET("/test", func(c *gin.Context) {
			c.Set(ContextKeyUserID, "user-123")
			c.Set(ContextKeyUsername, "testuser")
			c.Set(ContextKeyRole, "system_admin")
			// Don't set org ID

			userID, username, role, orgID, ok := GetUserFromContext(c)
			assert.True(t, ok)
			assert.Equal(t, "user-123", userID)
			assert.Equal(t, "testuser", username)
			assert.Equal(t, "system_admin", role)
			assert.Empty(t, orgID) // Should be empty

			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestLegacyMiddlewareFunctions(t *testing.T) {
	secret := "legacy-secret"
	tm := NewTokenManager(secret, time.Hour)
	validToken, err := tm.GenerateToken("user-123", "testuser", "admin", "org-456")
	require.NoError(t, err)

	t.Run("AuthMiddleware", func(t *testing.T) {
		router := setupTestGin()

		handlerCalled := false
		router.GET("/test", AuthMiddleware(secret), func(c *gin.Context) {
			handlerCalled = true
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set(AuthorizationHeader, "Bearer "+validToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, handlerCalled)
	})

	t.Run("LegacyRequireRole", func(t *testing.T) {
		router := setupTestGin()

		handlerCalled := false
		router.GET("/test", func(c *gin.Context) {
			c.Set(ContextKeyRole, "admin")
			c.Next()
		}, RequireRole("admin", "superuser"), func(c *gin.Context) {
			handlerCalled = true
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, handlerCalled)
	})

	t.Run("LegacyRequireOrgAccess", func(t *testing.T) {
		router := setupTestGin()

		handlerCalled := false
		router.GET("/test/:org_id", func(c *gin.Context) {
			c.Set(ContextKeyRole, "org_admin")
			c.Set(ContextKeyOrgID, "org-123")
			c.Next()
		}, RequireOrgAccess(), func(c *gin.Context) {
			handlerCalled = true
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("GET", "/test/org-123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, handlerCalled)
	})
}

func TestMiddlewareConstants(t *testing.T) {
	assert.Equal(t, "user_id", ContextKeyUserID)
	assert.Equal(t, "username", ContextKeyUsername)
	assert.Equal(t, "role", ContextKeyRole)
	assert.Equal(t, "org_id", ContextKeyOrgID)
	assert.Equal(t, "Authorization", AuthorizationHeader)
	assert.Equal(t, "Bearer ", BearerPrefix)
}

func TestMiddlewareIntegration(t *testing.T) {
	// Test full middleware chain
	tm := NewTokenManager("integration-secret", time.Hour)
	middleware := NewMiddleware(tm)

	// Generate tokens for different users
	adminToken, err := tm.GenerateToken("admin-123", "admin", "admin", "org-1")
	require.NoError(t, err)

	userToken, err := tm.GenerateToken("user-456", "regularuser", "user", "org-1")
	require.NoError(t, err)

	router := setupTestGin()

	// Admin-only endpoint
	adminCalled := false
	router.GET("/admin/:org_id",
		middleware.RequireAuth(),
		middleware.RequireRole("admin"),
		middleware.RequireOrgAccess(),
		func(c *gin.Context) {
			adminCalled = true
			c.JSON(http.StatusOK, gin.H{"message": "admin success"})
		})

	// User endpoint
	userCalled := false
	router.GET("/user/:org_id",
		middleware.RequireAuth(),
		middleware.RequireRole("user", "admin"),
		middleware.RequireOrgAccess(),
		func(c *gin.Context) {
			userCalled = true
			c.JSON(http.StatusOK, gin.H{"message": "user success"})
		})

	// Test admin access to admin endpoint
	req := httptest.NewRequest("GET", "/admin/org-1", nil)
	req.Header.Set(AuthorizationHeader, "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, adminCalled)

	// Reset flags
	adminCalled = false
	userCalled = false

	// Test user access to user endpoint
	req = httptest.NewRequest("GET", "/user/org-1", nil)
	req.Header.Set(AuthorizationHeader, "Bearer "+userToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, userCalled)

	// Reset flags
	adminCalled = false
	userCalled = false

	// Test user trying to access admin endpoint (should fail)
	req = httptest.NewRequest("GET", "/admin/org-1", nil)
	req.Header.Set(AuthorizationHeader, "Bearer "+userToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, adminCalled)
}
