package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/openshift"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/eliorerz/ovim-updated/pkg/version"
)

const (
	// API version constants
	APIVersion = "v1"
	APIPrefix  = "/api/" + APIVersion
)

// Server represents the HTTP server for the OVIM API
type Server struct {
	config          *config.Config
	storage         storage.Storage
	provisioner     kubevirt.VMProvisioner
	authManager     *auth.Middleware
	tokenManager    *auth.TokenManager
	oidcProvider    *auth.OIDCProvider
	openshiftClient *openshift.Client
	router          *gin.Engine
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config, storage storage.Storage, provisioner kubevirt.VMProvisioner) *Server {
	// Set gin mode based on environment
	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create token manager
	tokenManager := auth.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.TokenDuration)

	// Create auth middleware
	authManager := auth.NewMiddleware(tokenManager)

	// Create OIDC provider if enabled
	var oidcProvider *auth.OIDCProvider
	if cfg.Auth.OIDC.Enabled {
		var err error
		authOIDCConfig := &auth.OIDCConfig{
			Enabled:      cfg.Auth.OIDC.Enabled,
			IssuerURL:    cfg.Auth.OIDC.IssuerURL,
			ClientID:     cfg.Auth.OIDC.ClientID,
			ClientSecret: cfg.Auth.OIDC.ClientSecret,
			RedirectURL:  cfg.Auth.OIDC.RedirectURL,
			Scopes:       cfg.Auth.OIDC.Scopes,
		}
		oidcProvider, err = auth.NewOIDCProvider(authOIDCConfig)
		if err != nil {
			klog.Errorf("Failed to initialize OIDC provider: %v", err)
			// Don't fail server startup, just disable OIDC
			oidcProvider = nil
		} else {
			klog.Infof("OIDC provider initialized successfully for issuer: %s", cfg.Auth.OIDC.IssuerURL)
		}
	}

	// Create OpenShift client if enabled
	var openshiftClient *openshift.Client
	if cfg.OpenShift.Enabled {
		var err error
		openshiftClient, err = openshift.NewClient(&cfg.OpenShift)
		if err != nil {
			klog.Errorf("Failed to initialize OpenShift client: %v", err)
			// Don't fail server startup, just disable OpenShift integration
			openshiftClient = nil
		} else {
			klog.Infof("OpenShift client initialized successfully")
		}
	}

	server := &Server{
		config:          cfg,
		storage:         storage,
		provisioner:     provisioner,
		authManager:     authManager,
		tokenManager:    tokenManager,
		oidcProvider:    oidcProvider,
		openshiftClient: openshiftClient,
		router:          gin.New(),
	}

	server.setupMiddleware()
	server.setupRoutes()

	return server
}

// Handler returns the HTTP handler for the server
func (s *Server) Handler() http.Handler {
	return s.router
}

// setupMiddleware configures global middleware
func (s *Server) setupMiddleware() {
	// Recovery middleware
	s.router.Use(gin.Recovery())

	// Logging middleware
	if s.config.Server.Environment != "production" {
		s.router.Use(gin.Logger())
	}

	// CORS middleware
	s.router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health endpoint (no authentication required)
	s.router.GET("/health", s.healthHandler)
	s.router.GET("/version", s.versionHandler)

	// API routes
	api := s.router.Group(APIPrefix)
	{
		// Authentication routes (no auth required)
		auth := api.Group("/auth")
		{
			authHandlers := NewAuthHandlers(s.storage, s.tokenManager, s.oidcProvider)
			auth.POST("/login", authHandlers.Login)
			auth.POST("/logout", authHandlers.Logout)
			auth.GET("/info", authHandlers.GetAuthInfo)

			// OIDC endpoints
			if s.oidcProvider != nil {
				auth.GET("/oidc/auth-url", authHandlers.GetOIDCAuthURL)
				auth.POST("/oidc/callback", authHandlers.HandleOIDCCallback)
			}
		}

		// Protected routes (authentication required)
		protected := api.Group("/")
		protected.Use(s.authManager.RequireAuth())
		{
			// Organization management (system admin only)
			orgs := protected.Group("/organizations")
			orgs.Use(s.authManager.RequireRole("system_admin"))
			{
				orgHandlers := NewOrganizationHandlers(s.storage)
				catalogHandlers := NewCatalogHandlers(s.storage)
				orgs.GET("/", orgHandlers.List)
				orgs.POST("/", orgHandlers.Create)
				orgs.GET("/:id", orgHandlers.Get)
				orgs.PUT("/:id", orgHandlers.Update)
				orgs.DELETE("/:id", orgHandlers.Delete)
				orgs.GET("/:id/templates", catalogHandlers.ListTemplatesByOrg)
			}

			// VDC management (system admin and org admin)
			vdcs := protected.Group("/vdcs")
			vdcs.Use(s.authManager.RequireRole("system_admin", "org_admin"))
			{
				vdcHandlers := NewVDCHandlers(s.storage)
				vdcs.GET("/", vdcHandlers.List)
				vdcs.POST("/", vdcHandlers.Create)
				vdcs.GET("/:id", vdcHandlers.Get)
				vdcs.PUT("/:id", vdcHandlers.Update)
				vdcs.DELETE("/:id", vdcHandlers.Delete)
			}

			// VM catalog (all authenticated users)
			catalog := protected.Group("/catalog")
			{
				catalogHandlers := NewCatalogHandlers(s.storage)
				catalog.GET("/templates", catalogHandlers.ListTemplates)
				catalog.GET("/templates/:id", catalogHandlers.GetTemplate)
			}

			// VM management (all authenticated users, filtered by role)
			vms := protected.Group("/vms")
			{
				vmHandlers := NewVMHandlers(s.storage, s.provisioner)
				vms.GET("/", vmHandlers.List)
				vms.POST("/", vmHandlers.Create)
				vms.GET("/:id", vmHandlers.Get)
				vms.GET("/:id/status", vmHandlers.GetStatus)
				vms.PUT("/:id/power", vmHandlers.UpdatePower)
				vms.DELETE("/:id", vmHandlers.Delete)
			}

			// OpenShift integration (all authenticated users)
			if s.openshiftClient != nil {
				openshift := protected.Group("/openshift")
				{
					osHandlers := NewOpenShiftHandlers(s.openshiftClient)
					openshift.GET("/status", osHandlers.GetOpenShiftStatus)
					openshift.GET("/templates", osHandlers.GetOpenShiftTemplates)
					openshift.GET("/vms", osHandlers.GetOpenShiftVMs)
					openshift.POST("/vms", osHandlers.DeployVMFromTemplate)
				}
			}
		}
	}

	klog.Infof("API routes configured with prefix %s", APIPrefix)
}

// healthHandler handles health check requests
func (s *Server) healthHandler(c *gin.Context) {
	status := "healthy"
	httpStatus := http.StatusOK

	// Check storage health
	if err := s.storage.Ping(); err != nil {
		klog.Errorf("Storage health check failed: %v", err)
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":  status,
		"service": "OVIM Backend",
		"version": version.Get().GitVersion,
	})
}

// versionHandler handles version information requests
func (s *Server) versionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, version.Get())
}
