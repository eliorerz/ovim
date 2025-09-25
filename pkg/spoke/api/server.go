package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
)

// Server implements the LocalAPIServer interface for the spoke agent
type Server struct {
	config     *config.SpokeConfig
	logger     *slog.Logger
	httpServer *http.Server
	hubClient  spoke.HubClient
	agent      spoke.Agent
}

// NewServer creates a new local API server for the spoke agent
func NewServer(cfg *config.SpokeConfig, logger *slog.Logger) *Server {
	return &Server{
		config: cfg,
		logger: logger,
	}
}

// SetHubClient sets the hub client reference for the server
func (s *Server) SetHubClient(client spoke.HubClient) {
	s.hubClient = client
}

// SetAgent sets the agent reference for the server
func (s *Server) SetAgent(agent spoke.Agent) {
	s.agent = agent
}

// Start starts the local API server
func (s *Server) Start(ctx context.Context, addr string) error {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Health endpoint
	router.GET("/health", s.handleHealth)

	// Status endpoint
	router.GET("/status", s.handleStatus)

	// Operations webhook endpoint for push notifications from hub
	router.POST("/operations", s.handleOperations)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: router,
	}

	s.logger.Info("Starting local API server", "address", addr)

	// Start server in goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Local API server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the local API server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	s.logger.Info("Stopping local API server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

// RegisterRoutes registers additional routes (currently no-op)
func (s *Server) RegisterRoutes(routes map[string]interface{}) {
	// TODO: Implement dynamic route registration if needed
	s.logger.Debug("RegisterRoutes called", "routes", len(routes))
}

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	status := "healthy"
	if s.hubClient != nil && !s.hubClient.IsConnected() {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"timestamp": time.Now(),
		"agent_id":  s.config.AgentID,
	})
}

// handleStatus handles status requests
func (s *Server) handleStatus(c *gin.Context) {
	if s.agent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent not available"})
		return
	}

	status := s.agent.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleOperations handles incoming operations from the hub via push notification
func (s *Server) handleOperations(c *gin.Context) {
	var operation spoke.Operation
	if err := c.ShouldBindJSON(&operation); err != nil {
		s.logger.Error("Failed to parse operation from hub", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid operation format"})
		return
	}

	s.logger.Info("Received operation via push notification from hub",
		"operation_id", operation.ID,
		"type", operation.Type)

	// If this is a VDC creation operation, print the message as requested by user
	if operation.Type == spoke.OperationCreateVDC {
		s.logger.Info("🔥 VDC CREATION OPERATION RECEIVED!",
			"operation_id", operation.ID,
			"payload", operation.Payload)

		// Also print to stdout for visibility
		fmt.Printf("🔥 VDC CREATION OPERATION RECEIVED! ID: %s, Payload: %+v\n",
			operation.ID, operation.Payload)
	}

	// Forward the operation to the hub client for processing
	if s.hubClient != nil {
		s.hubClient.ReceiveOperation(&operation)
	} else {
		s.logger.Warn("Hub client not available, cannot forward operation")
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "received",
		"message": "Operation received and forwarded for processing",
	})
}