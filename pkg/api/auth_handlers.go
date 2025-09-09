package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// AuthHandlers handles authentication-related requests
type AuthHandlers struct {
	storage      storage.Storage
	tokenManager *auth.TokenManager
	oidcProvider *auth.OIDCProvider
}

// NewAuthHandlers creates a new auth handlers instance
func NewAuthHandlers(storage storage.Storage, tokenManager *auth.TokenManager, oidcProvider *auth.OIDCProvider) *AuthHandlers {
	return &AuthHandlers{
		storage:      storage,
		tokenManager: tokenManager,
		oidcProvider: oidcProvider,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// Login handles user authentication
func (h *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid login request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user by username
	user, err := h.storage.GetUserByUsername(req.Username)
	if err != nil {
		if err == storage.ErrNotFound {
			klog.V(4).Infof("Login attempt for non-existent user: %s", req.Username)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		klog.Errorf("Failed to get user %s: %v", req.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Verify password
	valid, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		klog.Errorf("Password verification error for user %s: %v", req.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if !valid {
		klog.V(4).Infof("Invalid password for user: %s", req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT token
	orgID := ""
	if user.OrgID != nil {
		orgID = *user.OrgID
	}

	token, err := h.tokenManager.GenerateToken(user.ID, user.Username, user.Role, orgID)
	if err != nil {
		klog.Errorf("Failed to generate token for user %s: %v", req.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Prepare user response (without password hash)
	userResponse := *user
	userResponse.PasswordHash = ""

	klog.Infof("User %s logged in successfully (role: %s)", user.Username, user.Role)

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  &userResponse,
	})
}

// Logout handles user logout
func (h *AuthHandlers) Logout(c *gin.Context) {
	// In a stateless JWT system, logout is typically handled client-side
	// by simply discarding the token. We could implement token blacklisting
	// here if needed in the future.

	userID, username, _, _, ok := auth.GetUserFromContext(c)
	if ok {
		klog.Infof("User %s (%s) logged out", username, userID)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logout successful",
	})
}

// GetOIDCAuthURL handles OIDC authentication initiation
func (h *AuthHandlers) GetOIDCAuthURL(c *gin.Context) {
	if h.oidcProvider == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC authentication is not configured"})
		return
	}

	state := h.oidcProvider.GenerateState()
	authURL := h.oidcProvider.GetAuthURL(state)

	// Store state in session or cache for validation
	// For simplicity, we'll return it to the client to send back
	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

// OIDCCallbackRequest represents the OIDC callback request
type OIDCCallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// HandleOIDCCallback handles the OIDC callback
func (h *AuthHandlers) HandleOIDCCallback(c *gin.Context) {
	if h.oidcProvider == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC authentication is not configured"})
		return
	}

	var req OIDCCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid OIDC callback request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Exchange code for tokens
	token, err := h.oidcProvider.ExchangeCode(ctx, req.Code)
	if err != nil {
		klog.Errorf("Failed to exchange OIDC code: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to authenticate with OIDC provider"})
		return
	}

	// Extract and verify ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		klog.Error("No ID token found in OIDC response")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OIDC response"})
		return
	}

	idToken, err := h.oidcProvider.VerifyIDToken(ctx, rawIDToken)
	if err != nil {
		klog.Errorf("Failed to verify OIDC ID token: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid ID token"})
		return
	}

	// Get user info from ID token
	userInfo, err := h.oidcProvider.GetUserInfo(ctx, idToken)
	if err != nil {
		klog.Errorf("Failed to extract user info from ID token: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to extract user information"})
		return
	}

	// Map OIDC user to OVIM user
	ovimRole := h.oidcProvider.MapOIDCRolesToOVIM(userInfo)
	username := userInfo.PreferredUsername
	if username == "" {
		username = userInfo.Email
	}
	if username == "" {
		username = userInfo.Subject
	}

	// Create or update user in our system
	user, err := h.getOrCreateOIDCUser(userInfo, ovimRole)
	if err != nil {
		klog.Errorf("Failed to create/update OIDC user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user account"})
		return
	}

	// Generate our own JWT token for the user
	orgID := ""
	if user.OrgID != nil {
		orgID = *user.OrgID
	}

	jwtToken, err := h.tokenManager.GenerateToken(user.ID, user.Username, user.Role, orgID)
	if err != nil {
		klog.Errorf("Failed to generate JWT token for OIDC user %s: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	// Prepare user response (without password hash)
	userResponse := *user
	userResponse.PasswordHash = ""

	klog.Infof("OIDC user %s logged in successfully (role: %s)", user.Username, user.Role)

	c.JSON(http.StatusOK, LoginResponse{
		Token: jwtToken,
		User:  &userResponse,
	})
}

// getOrCreateOIDCUser creates or updates a user from OIDC information
func (h *AuthHandlers) getOrCreateOIDCUser(userInfo *auth.UserInfo, role string) (*models.User, error) {
	username := userInfo.PreferredUsername
	if username == "" {
		username = userInfo.Email
	}
	if username == "" {
		username = userInfo.Subject
	}

	// Try to find existing user
	user, err := h.storage.GetUserByUsername(username)
	if err != nil && err != storage.ErrNotFound {
		return nil, err
	}

	if user != nil {
		// Update existing user
		user.Email = userInfo.Email
		user.Role = role
		// Don't update password hash for OIDC users
		return user, h.storage.UpdateUser(user)
	}

	// Create new user
	user = &models.User{
		ID:           userInfo.Subject, // Use OIDC subject as user ID
		Username:     username,
		Email:        userInfo.Email,
		Role:         role,
		PasswordHash: "", // No password for OIDC users
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// For org_admin and user roles, we might want to assign them to a default org
	// This depends on your business logic
	if role != "system_admin" {
		// You might want to extract organization from OIDC claims
		// For now, we'll leave OrgID as nil
	}

	return user, h.storage.CreateUser(user)
}

// GetAuthInfo returns information about available authentication methods
func (h *AuthHandlers) GetAuthInfo(c *gin.Context) {
	authInfo := gin.H{
		"local_auth_enabled": true,
		"oidc_enabled":       h.oidcProvider != nil,
	}

	c.JSON(http.StatusOK, authInfo)
}
