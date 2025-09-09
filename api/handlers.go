package api

import (
	"net/http"

	"github.com/eliorerz/ovim-updated/auth"
	"github.com/eliorerz/ovim-updated/models"
	"github.com/eliorerz/ovim-updated/storage"
	"github.com/gin-gonic/gin"
)

var store *storage.MemoryStorage

func InitStorage() {
	store = storage.NewMemoryStorage()
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := store.GetUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	valid, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	orgID := ""
	if user.OrgID != nil {
		orgID = *user.OrgID
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role, orgID, "ovim-jwt-secret")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	userResponse := *user
	userResponse.PasswordHash = ""

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  &userResponse,
	})
}

func logoutHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Logout successful",
	})
}

func listOrganizationsHandler(c *gin.Context) {
	orgs, err := store.ListOrganizations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list organizations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"organizations": orgs,
	})
}

func createOrganizationHandler(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"message": "Create organization endpoint - to be implemented",
	})
}

func getOrganizationHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Get organization endpoint - to be implemented",
	})
}

func updateOrganizationHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Update organization endpoint - to be implemented",
	})
}

func deleteOrganizationHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Delete organization endpoint - to be implemented",
	})
}

func listVDCsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"vdcs":    []interface{}{},
		"message": "List VDCs endpoint - to be implemented",
	})
}

func getVDCHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Get VDC endpoint - to be implemented",
	})
}

func updateVDCHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Update VDC endpoint - to be implemented",
	})
}

func listTemplatesHandler(c *gin.Context) {
	templates, err := store.ListTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
	})
}

func getTemplateHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Get template endpoint - to be implemented",
	})
}

func listVMsHandler(c *gin.Context) {
	role, _ := c.Get("role")
	orgID := ""

	if role != "system_admin" {
		userOrgID, _ := c.Get("org_id")
		if userOrgID != nil {
			orgID = userOrgID.(string)
		}
	}

	vms, err := store.ListVMs(orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list VMs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"vms": vms,
	})
}

func createVMHandler(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"message": "Create VM endpoint - to be implemented",
	})
}

func getVMHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Get VM endpoint - to be implemented",
	})
}

func powerVMHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Power VM endpoint - to be implemented",
	})
}

func deleteVMHandler(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "Delete VM endpoint - to be implemented",
	})
}
