package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
)

// Agent represents the main spoke agent
type Agent struct {
	config           *config.SpokeConfig
	hubClient        spoke.HubClient
	vmManager        spoke.VMManager
	vdcManager       spoke.VDCManager
	metricsCollector spoke.MetricsCollector
	healthReporter   spoke.HealthReporter
	templateManager  spoke.TemplateManager
	processor        spoke.OperationProcessor
	localAPI         spoke.LocalAPIServer

	// Internal state
	status      spoke.AgentStatus
	startTime   time.Time
	lastContact time.Time
	mu          sync.RWMutex
	logger      *slog.Logger

	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAgent creates a new spoke agent with the given configuration
func NewAgent(cfg *config.SpokeConfig, logger *slog.Logger) *Agent {
	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		config:    cfg,
		status:    spoke.AgentStatusHealthy,
		startTime: time.Now(),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}

	return agent
}

// SetHubClient sets the hub client for the agent
func (a *Agent) SetHubClient(client spoke.HubClient) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.hubClient = client
}

// SetVMManager sets the VM manager for the agent
func (a *Agent) SetVMManager(manager spoke.VMManager) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.vmManager = manager
}

// SetVDCManager sets the VDC manager for the agent
func (a *Agent) SetVDCManager(manager spoke.VDCManager) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.vdcManager = manager
}

// SetMetricsCollector sets the metrics collector for the agent
func (a *Agent) SetMetricsCollector(collector spoke.MetricsCollector) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.metricsCollector = collector
}

// SetHealthReporter sets the health reporter for the agent
func (a *Agent) SetHealthReporter(reporter spoke.HealthReporter) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.healthReporter = reporter
}

// SetTemplateManager sets the template manager for the agent
func (a *Agent) SetTemplateManager(manager spoke.TemplateManager) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.templateManager = manager
}

// SetOperationProcessor sets the operation processor for the agent
func (a *Agent) SetOperationProcessor(processor spoke.OperationProcessor) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.processor = processor
}

// SetLocalAPIServer sets the local API server for the agent
func (a *Agent) SetLocalAPIServer(server spoke.LocalAPIServer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.localAPI = server
}

// Start starts the agent with the given context
func (a *Agent) Start(ctx context.Context) error {
	a.logger.Info("Starting OVIM spoke agent",
		"agent_id", a.config.AgentID,
		"cluster_id", a.config.ClusterID,
		"zone_id", a.config.ZoneID,
		"version", a.config.Version)

	// Validate that required components are set
	if err := a.validateComponents(); err != nil {
		return fmt.Errorf("component validation failed: %w", err)
	}

	// Start hub connection
	if err := a.startHubConnection(); err != nil {
		a.logger.Error("Failed to connect to hub", "error", err)
		a.setStatus(spoke.AgentStatusDegraded)
		// Continue starting other components even if hub is unavailable
	}

	// Start local API server if enabled
	if a.config.Features.LocalAPI && a.localAPI != nil {
		if err := a.startLocalAPI(); err != nil {
			a.logger.Error("Failed to start local API server", "error", err)
			// Non-critical, continue
		}
	}

	// Start metrics collection
	if a.config.Metrics.Enabled && a.metricsCollector != nil {
		if err := a.startMetricsCollection(); err != nil {
			a.logger.Error("Failed to start metrics collection", "error", err)
			// Non-critical, continue
		}
	}

	// Start health reporting
	if a.config.Health.Enabled && a.healthReporter != nil {
		if err := a.startHealthReporting(); err != nil {
			a.logger.Error("Failed to start health reporting", "error", err)
			// Non-critical, continue
		}
	}

	// Start operation processing
	if err := a.startOperationProcessing(); err != nil {
		a.logger.Error("Failed to start operation processing", "error", err)
		// This is critical for functionality
		return fmt.Errorf("failed to start operation processing: %w", err)
	}

	// Start periodic status reporting
	a.startStatusReporting()

	a.logger.Info("OVIM spoke agent started successfully")
	return nil
}

// Stop gracefully stops the agent
func (a *Agent) Stop() error {
	a.logger.Info("Stopping OVIM spoke agent")

	// Cancel context to signal all goroutines to stop
	a.cancel()

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		a.logger.Info("All components stopped gracefully")
	case <-time.After(30 * time.Second):
		a.logger.Warn("Timeout waiting for components to stop")
	}

	// Stop local API server
	if a.localAPI != nil {
		if err := a.localAPI.Stop(); err != nil {
			a.logger.Error("Failed to stop local API server", "error", err)
		}
	}

	// Disconnect from hub
	if a.hubClient != nil {
		if err := a.hubClient.Disconnect(); err != nil {
			a.logger.Error("Failed to disconnect from hub", "error", err)
		}
	}

	a.setStatus(spoke.AgentStatusUnavailable)
	a.logger.Info("OVIM spoke agent stopped")
	return nil
}

// GetStatus returns the current agent status
func (a *Agent) GetStatus() *spoke.StatusReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &spoke.StatusReport{
		AgentID:        a.config.AgentID,
		ClusterID:      a.config.ClusterID,
		ZoneID:         a.config.ZoneID,
		Status:         a.status,
		Version:        a.config.Version,
		LastHubContact: a.lastContact,
		ReportTime:     time.Now(),
	}

	// Collect current metrics if available
	if a.metricsCollector != nil {
		if metrics, err := a.metricsCollector.CollectClusterMetrics(a.ctx); err == nil {
			report.Metrics = *metrics
		} else {
			a.logger.Warn("Failed to collect metrics for status report", "error", err)
		}
	}

	// Collect VDC status if available
	if a.vdcManager != nil {
		if vdcs, err := a.vdcManager.ListVDCs(a.ctx); err == nil {
			report.VDCs = vdcs
		} else {
			a.logger.Warn("Failed to list VDCs for status report", "error", err)
		}
	}

	// Collect VM status if available
	if a.vmManager != nil {
		if vms, err := a.vmManager.ListVMs(a.ctx); err == nil {
			report.VMs = vms
		} else {
			a.logger.Warn("Failed to list VMs for status report", "error", err)
		}
	}

	return report
}

// GetHealth returns the current health status
func (a *Agent) GetHealth() *spoke.AgentHealth {
	a.mu.RLock()
	defer a.mu.RUnlock()

	health := &spoke.AgentHealth{
		Status:     a.status,
		Uptime:     time.Since(a.startTime),
		Version:    a.config.Version,
		LastReport: time.Now(),
	}

	// Get detailed health checks if available
	if a.healthReporter != nil {
		if agentHealth, err := a.healthReporter.CheckHealth(a.ctx); err == nil {
			health.Checks = agentHealth.Checks
		} else {
			a.logger.Warn("Failed to get health checks", "error", err)
		}
	}

	return health
}

// validateComponents checks that all required components are set
func (a *Agent) validateComponents() error {
	if a.hubClient == nil {
		return fmt.Errorf("hub client is required")
	}
	if a.vmManager == nil {
		return fmt.Errorf("VM manager is required")
	}
	if a.processor == nil {
		return fmt.Errorf("operation processor is required")
	}
	return nil
}

// startHubConnection establishes connection to the hub
func (a *Agent) startHubConnection() error {
	a.logger.Info("Connecting to hub", "endpoint", a.config.Hub.Endpoint)

	if err := a.hubClient.Connect(a.ctx); err != nil {
		return fmt.Errorf("failed to connect to hub: %w", err)
	}

	a.updateLastContact()
	a.logger.Info("Connected to hub successfully")
	return nil
}

// startLocalAPI starts the local API server
func (a *Agent) startLocalAPI() error {
	addr := fmt.Sprintf("%s:%d", a.config.API.Address, a.config.API.Port)
	a.logger.Info("Starting local API server", "address", addr)

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.localAPI.Start(a.ctx, addr); err != nil {
			a.logger.Error("Local API server error", "error", err)
		}
	}()

	return nil
}

// startMetricsCollection starts periodic metrics collection
func (a *Agent) startMetricsCollection() error {
	a.logger.Info("Starting metrics collection",
		"interval", a.config.Metrics.CollectionInterval)

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.metricsCollector.StartPeriodicCollection(a.ctx, a.config.Metrics.CollectionInterval); err != nil {
			a.logger.Error("Metrics collection error", "error", err)
		}
	}()

	return nil
}

// startHealthReporting starts periodic health reporting
func (a *Agent) startHealthReporting() error {
	a.logger.Info("Starting health reporting",
		"interval", a.config.Health.ReportInterval)

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.healthReporter.StartPeriodicReporting(a.ctx, a.config.Health.ReportInterval); err != nil {
			a.logger.Error("Health reporting error", "error", err)
		}
	}()

	return nil
}

// startOperationProcessing starts processing operations from the hub
func (a *Agent) startOperationProcessing() error {
	a.logger.Info("Starting operation processing")

	operations := a.hubClient.ReceiveOperations()
	results := make(chan *spoke.OperationResult, 100)

	// Start processing operations
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.processor.StartProcessing(a.ctx, operations, results); err != nil {
			a.logger.Error("Operation processing error", "error", err)
		}
	}()

	// Start sending results back to hub
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case result := <-results:
				if err := a.hubClient.SendOperationResult(a.ctx, result); err != nil {
					a.logger.Error("Failed to send operation result to hub",
						"operation_id", result.OperationID, "error", err)
				} else {
					a.updateLastContact()
				}
			case <-a.ctx.Done():
				return
			}
		}
	}()

	return nil
}

// startStatusReporting starts periodic status reporting to the hub
func (a *Agent) startStatusReporting() {
	a.logger.Info("Starting status reporting",
		"interval", a.config.Metrics.ReportingInterval)

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(a.config.Metrics.ReportingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				status := a.GetStatus()
				if err := a.hubClient.SendStatusReport(a.ctx, status); err != nil {
					a.logger.Error("Failed to send status report to hub", "error", err)
					a.setStatus(spoke.AgentStatusDegraded)
				} else {
					a.updateLastContact()
					if a.status == spoke.AgentStatusDegraded {
						a.setStatus(spoke.AgentStatusHealthy)
					}
				}
			case <-a.ctx.Done():
				return
			}
		}
	}()
}

// setStatus updates the agent status
func (a *Agent) setStatus(status spoke.AgentStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.status != status {
		a.logger.Info("Agent status changed", "old_status", a.status, "new_status", status)
		a.status = status
	}
}

// updateLastContact updates the last hub contact time
func (a *Agent) updateLastContact() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastContact = time.Now()
}
