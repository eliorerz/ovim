package api

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/eliorerz/ovim-updated/pkg/util"
)

// UserHandlers handles user-related requests
type UserHandlers struct {
	storage storage.Storage
}

// NewUserHandlers creates a new user handlers instance
func NewUserHandlers(storage storage.Storage) *UserHandlers {
	return &UserHandlers{
		storage: storage,
	}
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Username string  `json:"username" binding:"required"`
	Email    string  `json:"email" binding:"required"`
	Password string  `json:"password" binding:"required"`
	Role     string  `json:"role" binding:"required"`
	OrgID    *string `json:"org_id"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Username string  `json:"username"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	OrgID    *string `json:"org_id"`
}

// List handles listing all users (system admin only)
func (h *UserHandlers) List(c *gin.Context) {
	users, err := h.storage.ListUsers()
	if err != nil {
		klog.Errorf("Failed to list users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	klog.V(6).Infof("Listed %d users", len(users))
	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
	})
}

// Get handles getting a specific user
func (h *UserHandlers) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	user, err := h.storage.GetUserByID(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		klog.Errorf("Failed to get user %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	klog.V(6).Infof("Retrieved user: %s", user.Username)
	c.JSON(http.StatusOK, user)
}

// Create handles creating a new user (system admin only)
func (h *UserHandlers) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate username
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username must be at least 3 characters long"})
		return
	}
	if len(req.Username) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username must be less than 50 characters long"})
		return
	}

	// Validate email
	req.Email = strings.TrimSpace(req.Email)
	if !isValidEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	// Validate role
	if req.Role != models.RoleSystemAdmin && req.Role != models.RoleOrgAdmin && req.Role != models.RoleOrgUser {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role. Must be 'system_admin', 'org_admin', or 'org_user'"})
		return
	}

	// Validate organization assignment for non-system admins
	if req.Role != models.RoleSystemAdmin && req.OrgID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required for non-system admin users"})
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		klog.Errorf("Failed to hash password: %v", err)
		// Check for specific password validation errors
		if err == auth.ErrPasswordTooShort {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters long"})
			return
		}
		if err == auth.ErrPasswordTooLong {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be less than 128 characters long"})
			return
		}
		// Generic error for other cases (salt generation, etc.)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Generate user ID
	userID, err := util.GenerateID(16)
	if err != nil {
		klog.Errorf("Failed to generate user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate user ID"})
		return
	}

	// Create user
	user := &models.User{
		ID:           userID,
		Username:     strings.TrimSpace(req.Username),
		Email:        strings.TrimSpace(req.Email),
		PasswordHash: hashedPassword,
		Role:         req.Role,
		OrgID:        req.OrgID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.storage.CreateUser(user); err != nil {
		if err == storage.ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
			return
		}
		klog.Errorf("Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	klog.Infof("Created user: %s (role: %s)", user.Username, user.Role)
	c.JSON(http.StatusCreated, user)
}

// Update handles updating a user
func (h *UserHandlers) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing user
	user, err := h.storage.GetUserByID(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		klog.Errorf("Failed to get user %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Update fields if provided
	if req.Username != "" {
		user.Username = strings.TrimSpace(req.Username)
	}
	if req.Email != "" {
		user.Email = strings.TrimSpace(req.Email)
	}
	if req.Role != "" {
		// Validate role
		if req.Role != models.RoleSystemAdmin && req.Role != models.RoleOrgAdmin && req.Role != models.RoleOrgUser {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
			return
		}
		user.Role = req.Role
	}
	if req.OrgID != nil {
		user.OrgID = req.OrgID
	}

	// Validate organization assignment for non-system admins
	if user.Role != models.RoleSystemAdmin && user.OrgID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required for non-system admin users"})
		return
	}

	user.UpdatedAt = time.Now()

	if err := h.storage.UpdateUser(user); err != nil {
		klog.Errorf("Failed to update user %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	klog.Infof("Updated user: %s", user.Username)
	c.JSON(http.StatusOK, user)
}

// Delete handles deleting a user (system admin only)
func (h *UserHandlers) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	// Check if user exists first
	user, err := h.storage.GetUserByID(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		klog.Errorf("Failed to get user %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	if err := h.storage.DeleteUser(id); err != nil {
		klog.Errorf("Failed to delete user %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	klog.Infof("Deleted user: %s", user.Username)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// ListByOrganization handles listing users in a specific organization
func (h *UserHandlers) ListByOrganization(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Verify organization exists
	_, err := h.storage.GetOrganization(orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	users, err := h.storage.ListUsersByOrg(orgID)
	if err != nil {
		klog.Errorf("Failed to list users for organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	klog.V(6).Infof("Listed %d users for organization %s", len(users), orgID)
	c.JSON(http.StatusOK, gin.H{
		"users":  users,
		"total":  len(users),
		"org_id": orgID,
	})
}

// AssignToOrganization handles assigning a user to an organization
func (h *UserHandlers) AssignToOrganization(c *gin.Context) {
	userID := c.Param("userId")
	orgID := c.Param("id")

	if userID == "" || orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID and Organization ID required"})
		return
	}

	// Get user
	user, err := h.storage.GetUserByID(userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		klog.Errorf("Failed to get user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify organization exists
	_, err = h.storage.GetOrganization(orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to get organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	// Update user's organization
	user.OrgID = &orgID
	user.UpdatedAt = time.Now()

	if err := h.storage.UpdateUser(user); err != nil {
		klog.Errorf("Failed to assign user %s to organization %s: %v", userID, orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign user to organization"})
		return
	}

	klog.Infof("Assigned user %s to organization %s", user.Username, orgID)
	c.JSON(http.StatusOK, user)
}

// RemoveFromOrganization handles removing a user from an organization
func (h *UserHandlers) RemoveFromOrganization(c *gin.Context) {
	userID := c.Param("userId")
	orgID := c.Param("id")

	if userID == "" || orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID and Organization ID required"})
		return
	}

	// Get user
	user, err := h.storage.GetUserByID(userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		klog.Errorf("Failed to get user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Check if user belongs to the organization
	if user.OrgID == nil || *user.OrgID != orgID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User does not belong to this organization"})
		return
	}

	// System admins cannot be removed from organizations via this endpoint
	if user.Role == models.RoleSystemAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot remove system administrator from organization"})
		return
	}

	// Remove user from organization
	user.OrgID = nil
	user.UpdatedAt = time.Now()

	if err := h.storage.UpdateUser(user); err != nil {
		klog.Errorf("Failed to remove user %s from organization %s: %v", userID, orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove user from organization"})
		return
	}

	klog.Infof("Removed user %s from organization %s", user.Username, orgID)
	c.JSON(http.StatusOK, user)
}

// isValidEmail validates email format using a regular expression
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
