package api

import (
	"net/http"

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
}

// NewAuthHandlers creates a new auth handlers instance
func NewAuthHandlers(storage storage.Storage, tokenManager *auth.TokenManager) *AuthHandlers {
	return &AuthHandlers{
		storage:      storage,
		tokenManager: tokenManager,
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
