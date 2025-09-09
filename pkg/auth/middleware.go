package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

const (
	// Context keys for user information
	ContextKeyUserID   = "user_id"
	ContextKeyUsername = "username"
	ContextKeyRole     = "role"
	ContextKeyOrgID    = "org_id"

	// HTTP header constants
	AuthorizationHeader = "Authorization"
	BearerPrefix        = "Bearer "
)

// Middleware provides authentication and authorization middleware for Gin
type Middleware struct {
	tokenManager *TokenManager
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(tokenManager *TokenManager) *Middleware {
	return &Middleware{
		tokenManager: tokenManager,
	}
}

// RequireAuth is a middleware that requires valid authentication
func (m *Middleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(AuthorizationHeader)
		if authHeader == "" {
			klog.V(4).Info("Missing authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, BearerPrefix)
		if tokenString == authHeader {
			klog.V(4).Info("Invalid authorization header format")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		claims, err := m.tokenManager.ValidateToken(tokenString)
		if err != nil {
			klog.V(4).Infof("Token validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set user context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)
		c.Set(ContextKeyOrgID, claims.OrgID)

		klog.V(6).Infof("Authenticated user: %s (role: %s, org: %s)", claims.Username, claims.Role, claims.OrgID)
		c.Next()
	}
}

// RequireRole is a middleware that requires specific roles
func (m *Middleware) RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(ContextKeyRole)
		if !exists {
			klog.Warning("User role not found in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
			c.Abort()
			return
		}

		userRole := role.(string)
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				klog.V(6).Infof("Role check passed: %s", userRole)
				c.Next()
				return
			}
		}

		klog.V(4).Infof("Access denied for role %s, required one of: %v", userRole, allowedRoles)
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}

// RequireOrgAccess ensures users can only access their own organization's resources
func (m *Middleware) RequireOrgAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ContextKeyRole)
		userRole := role.(string)

		// System admins can access all organizations
		if userRole == "system_admin" {
			c.Next()
			return
		}

		userOrgID, _ := c.Get(ContextKeyOrgID)
		requestedOrgID := c.Param("org_id")
		if requestedOrgID == "" {
			requestedOrgID = c.Query("org_id")
		}

		if requestedOrgID != "" && userOrgID != requestedOrgID {
			klog.V(4).Infof("Access denied to organization %s for user in org %s", requestedOrgID, userOrgID)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to organization"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserFromContext extracts user information from gin context
func GetUserFromContext(c *gin.Context) (userID, username, role, orgID string, ok bool) {
	userIDVal, exists1 := c.Get(ContextKeyUserID)
	usernameVal, exists2 := c.Get(ContextKeyUsername)
	roleVal, exists3 := c.Get(ContextKeyRole)
	orgIDVal, _ := c.Get(ContextKeyOrgID)

	if !exists1 || !exists2 || !exists3 {
		return "", "", "", "", false
	}

	userID = userIDVal.(string)
	username = usernameVal.(string)
	role = roleVal.(string)
	if orgIDVal != nil {
		orgID = orgIDVal.(string)
	}

	return userID, username, role, orgID, true
}

// Legacy functions for backward compatibility
func AuthMiddleware(secret string) gin.HandlerFunc {
	tm := NewTokenManager(secret, DefaultTokenDuration)
	m := NewMiddleware(tm)
	return m.RequireAuth()
}

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	// This is a bit of a hack for backward compatibility
	// In practice, you should use the Middleware struct
	return func(c *gin.Context) {
		role, exists := c.Get(ContextKeyRole)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
			c.Abort()
			return
		}

		userRole := role.(string)
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}

func RequireOrgAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ContextKeyRole)
		userRole := role.(string)

		if userRole == "system_admin" {
			c.Next()
			return
		}

		userOrgID, _ := c.Get(ContextKeyOrgID)
		requestedOrgID := c.Param("org_id")
		if requestedOrgID == "" {
			requestedOrgID = c.Query("org_id")
		}

		if requestedOrgID != "" && userOrgID != requestedOrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to organization"})
			c.Abort()
			return
		}

		c.Next()
	}
}
