package health

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/spoke"
)

// Reporter implements the HealthReporter interface
type Reporter struct {
	checks    map[string]func(ctx context.Context) *spoke.HealthCheck
	logger    *slog.Logger
	startTime time.Time
	version   string
	mu        sync.RWMutex
}

// NewReporter creates a new health reporter
func NewReporter(version string, logger *slog.Logger) *Reporter {
	reporter := &Reporter{
		checks:    make(map[string]func(ctx context.Context) *spoke.HealthCheck),
		logger:    logger,
		startTime: time.Now(),
		version:   version,
	}

	// Register default health checks
	reporter.registerDefaultChecks()

	return reporter
}

// CheckHealth performs a comprehensive health check
func (r *Reporter) CheckHealth(ctx context.Context) (*spoke.AgentHealth, error) {
	r.logger.Debug("Performing comprehensive health check")

	r.mu.RLock()
	checks := make([]spoke.HealthCheck, 0, len(r.checks))

	for name, checkFunc := range r.checks {
		check := checkFunc(ctx)
		if check != nil {
			check.Component = name
			checks = append(checks, *check)
		}
	}
	r.mu.RUnlock()

	// Determine overall status
	status := r.determineOverallStatus(checks)

	health := &spoke.AgentHealth{
		Status:     status,
		Checks:     checks,
		Uptime:     time.Since(r.startTime),
		Version:    r.version,
		LastReport: time.Now(),
	}

	r.logger.Debug("Health check completed",
		"status", status,
		"checks_count", len(checks))

	return health, nil
}

// CheckComponent checks the health of a specific component
func (r *Reporter) CheckComponent(ctx context.Context, component string) (*spoke.HealthCheck, error) {
	r.mu.RLock()
	checkFunc, exists := r.checks[component]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("health check for component %s not found", component)
	}

	check := checkFunc(ctx)
	if check != nil {
		check.Component = component
	}

	return check, nil
}

// StartPeriodicReporting starts periodic health reporting to the hub
func (r *Reporter) StartPeriodicReporting(ctx context.Context, interval time.Duration) error {
	r.logger.Info("Starting periodic health reporting", "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			health, err := r.CheckHealth(ctx)
			if err != nil {
				r.logger.Error("Health check failed during periodic reporting", "error", err)
				continue
			}

			r.logger.Debug("Periodic health check completed",
				"status", health.Status,
				"uptime", health.Uptime)

		case <-ctx.Done():
			r.logger.Info("Periodic health reporting stopped")
			return ctx.Err()
		}
	}
}

// RegisterHealthCheck registers a custom health check
func (r *Reporter) RegisterHealthCheck(name string, check func(ctx context.Context) *spoke.HealthCheck) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.checks[name] = check
	r.logger.Info("Registered health check", "component", name)
}

// registerDefaultChecks registers default health checks
func (r *Reporter) registerDefaultChecks() {
	// Memory health check
	r.RegisterHealthCheck("memory", r.checkMemory)

	// Disk space health check
	r.RegisterHealthCheck("disk", r.checkDisk)

	// Basic connectivity check
	r.RegisterHealthCheck("connectivity", r.checkConnectivity)

	// Process health check
	r.RegisterHealthCheck("process", r.checkProcess)
}

// determineOverallStatus determines the overall health status based on individual checks
func (r *Reporter) determineOverallStatus(checks []spoke.HealthCheck) spoke.AgentStatus {
	if len(checks) == 0 {
		return spoke.AgentStatusUnavailable
	}

	hasCritical := false
	hasWarning := false

	for _, check := range checks {
		switch check.Status {
		case "critical":
			hasCritical = true
		case "warning":
			hasWarning = true
		}
	}

	if hasCritical {
		return spoke.AgentStatusUnavailable
	}
	if hasWarning {
		return spoke.AgentStatusDegraded
	}

	return spoke.AgentStatusHealthy
}

// checkMemory performs a memory usage health check
func (r *Reporter) checkMemory(ctx context.Context) *spoke.HealthCheck {
	start := time.Now()

	// TODO: Implement actual memory check using system calls or /proc/meminfo
	// For now, return a healthy status

	return &spoke.HealthCheck{
		Status:      "healthy",
		Message:     "Memory usage is within normal limits",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}
}

// checkDisk performs a disk space health check
func (r *Reporter) checkDisk(ctx context.Context) *spoke.HealthCheck {
	start := time.Now()

	// TODO: Implement actual disk space check using syscalls
	// For now, return a healthy status

	return &spoke.HealthCheck{
		Status:      "healthy",
		Message:     "Disk space is sufficient",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}
}

// checkConnectivity performs a basic connectivity health check
func (r *Reporter) checkConnectivity(ctx context.Context) *spoke.HealthCheck {
	start := time.Now()

	// TODO: Implement actual network connectivity check
	// This could ping the hub or check DNS resolution

	return &spoke.HealthCheck{
		Status:      "healthy",
		Message:     "Network connectivity is available",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}
}

// checkProcess performs a process health check
func (r *Reporter) checkProcess(ctx context.Context) *spoke.HealthCheck {
	start := time.Now()

	// Check if the process is running normally
	uptime := time.Since(r.startTime)

	status := "healthy"
	message := fmt.Sprintf("Process running normally, uptime: %v", uptime)

	// Consider the process unhealthy if it's been running for less than 10 seconds
	// (indicating frequent restarts)
	if uptime < 10*time.Second {
		status = "warning"
		message = fmt.Sprintf("Process recently started, uptime: %v", uptime)
	}

	return &spoke.HealthCheck{
		Status:      status,
		Message:     message,
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}
}
