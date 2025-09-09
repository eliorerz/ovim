package api

import (
	"github.com/eliorerz/ovim-updated/auth"
	"github.com/eliorerz/ovim-updated/config"
	"github.com/eliorerz/ovim-updated/models"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, cfg *config.Config) {
	api := r.Group("/api/v1")

	// Authentication routes (no auth required)
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", loginHandler)
		authGroup.POST("/logout", logoutHandler)
	}

	// Protected routes (auth required)
	protected := api.Group("/")
	protected.Use(auth.AuthMiddleware(cfg.JWTSecret))
	{
		// Organization management (system admin only)
		orgs := protected.Group("/organizations")
		orgs.Use(auth.RequireRole(models.RoleSystemAdmin))
		{
			orgs.GET("/", listOrganizationsHandler)
			orgs.POST("/", createOrganizationHandler)
			orgs.GET("/:id", getOrganizationHandler)
			orgs.PUT("/:id", updateOrganizationHandler)
			orgs.DELETE("/:id", deleteOrganizationHandler)
		}

		// VDC management (system admin and org admin)
		vdcs := protected.Group("/vdcs")
		vdcs.Use(auth.RequireRole(models.RoleSystemAdmin, models.RoleOrgAdmin))
		{
			vdcs.GET("/", listVDCsHandler)
			vdcs.GET("/:id", getVDCHandler)
			vdcs.PUT("/:id", updateVDCHandler)
		}

		// VM catalog (all authenticated users)
		catalog := protected.Group("/catalog")
		{
			catalog.GET("/templates", listTemplatesHandler)
			catalog.GET("/templates/:id", getTemplateHandler)
		}

		// VM management (all authenticated users, filtered by role)
		vms := protected.Group("/vms")
		{
			vms.GET("/", listVMsHandler)
			vms.POST("/", createVMHandler)
			vms.GET("/:id", getVMHandler)
			vms.PUT("/:id/power", powerVMHandler)
			vms.DELETE("/:id", deleteVMHandler)
		}
	}
}
