package operations

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/spoke"
)

// Processor implements the OperationProcessor interface
type Processor struct {
	handlers map[spoke.OperationType]spoke.OperationHandler
	logger   *slog.Logger
	mu       sync.RWMutex
}

// NewProcessor creates a new operation processor
func NewProcessor(logger *slog.Logger) *Processor {
	return &Processor{
		handlers: make(map[spoke.OperationType]spoke.OperationHandler),
		logger:   logger,
	}
}

// ProcessOperation processes a single operation from the hub
func (p *Processor) ProcessOperation(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	start := time.Now()

	p.logger.Debug("Processing operation",
		"operation_id", operation.ID,
		"type", operation.Type,
		"retry_count", operation.RetryCount)

	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusRunning,
		Timestamp:   time.Now(),
	}

	// Find handler for operation type
	p.mu.RLock()
	handler, exists := p.handlers[operation.Type]
	p.mu.RUnlock()

	if !exists {
		result.Status = spoke.OperationStatusFailed
		result.Error = fmt.Sprintf("no handler registered for operation type: %s", operation.Type)
		result.Duration = time.Since(start)

		p.logger.Error("No handler for operation type",
			"operation_id", operation.ID,
			"type", operation.Type)
		return result
	}

	// Create timeout context if specified
	opCtx := ctx
	if operation.TimeoutSecs > 0 {
		var cancel context.CancelFunc
		opCtx, cancel = context.WithTimeout(ctx, time.Duration(operation.TimeoutSecs)*time.Second)
		defer cancel()
	}

	// Process the operation
	result = handler.Handle(opCtx, operation)
	result.Duration = time.Since(start)

	p.logger.Info("Operation processed",
		"operation_id", operation.ID,
		"type", operation.Type,
		"status", result.Status,
		"duration", result.Duration,
		"error", result.Error)

	return result
}

// StartProcessing starts processing operations from the operations channel
func (p *Processor) StartProcessing(ctx context.Context, operations <-chan *spoke.Operation, results chan<- *spoke.OperationResult) error {
	p.logger.Info("Starting operation processing")

	for {
		select {
		case operation := <-operations:
			if operation == nil {
				continue
			}

			// Process operation and send result
			result := p.ProcessOperation(ctx, operation)

			select {
			case results <- result:
				// Result sent successfully
			case <-ctx.Done():
				return ctx.Err()
			}

		case <-ctx.Done():
			p.logger.Info("Operation processing stopped")
			return ctx.Err()
		}
	}
}

// RegisterHandler registers a handler for a specific operation type
func (p *Processor) RegisterHandler(operationType spoke.OperationType, handler spoke.OperationHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.handlers[operationType] = handler
	p.logger.Info("Registered operation handler", "type", operationType)
}

// BaseHandler provides common functionality for operation handlers
type BaseHandler struct {
	operationType spoke.OperationType
	logger        *slog.Logger
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(operationType spoke.OperationType, logger *slog.Logger) *BaseHandler {
	return &BaseHandler{
		operationType: operationType,
		logger:        logger,
	}
}

// CanHandle returns true if this handler can process the given operation type
func (h *BaseHandler) CanHandle(operationType spoke.OperationType) bool {
	return h.operationType == operationType
}

// createErrorResult creates an error result for an operation
func (h *BaseHandler) createErrorResult(operationID, error string) *spoke.OperationResult {
	return &spoke.OperationResult{
		OperationID: operationID,
		Status:      spoke.OperationStatusFailed,
		Error:       error,
		Timestamp:   time.Now(),
	}
}

// createSuccessResult creates a success result for an operation
func (h *BaseHandler) createSuccessResult(operationID string, result map[string]interface{}) *spoke.OperationResult {
	return &spoke.OperationResult{
		OperationID: operationID,
		Status:      spoke.OperationStatusCompleted,
		Result:      result,
		Timestamp:   time.Now(),
	}
}

// HealthHandler handles health check operations
type HealthHandler struct {
	*BaseHandler
	healthReporter spoke.HealthReporter
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(healthReporter spoke.HealthReporter, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		BaseHandler:    NewBaseHandler(spoke.OperationGetHealth, logger),
		healthReporter: healthReporter,
	}
}

// Handle processes health check operations
func (h *HealthHandler) Handle(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	h.logger.Debug("Processing health check operation", "operation_id", operation.ID)

	if h.healthReporter == nil {
		return h.createErrorResult(operation.ID, "health reporter not available")
	}

	health, err := h.healthReporter.CheckHealth(ctx)
	if err != nil {
		return h.createErrorResult(operation.ID, fmt.Sprintf("health check failed: %v", err))
	}

	result := map[string]interface{}{
		"status":      health.Status,
		"uptime":      health.Uptime.String(),
		"version":     health.Version,
		"checks":      health.Checks,
		"last_report": health.LastReport,
	}

	return h.createSuccessResult(operation.ID, result)
}

// MetricsHandler handles metrics collection operations
type MetricsHandler struct {
	*BaseHandler
	metricsCollector spoke.MetricsCollector
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(metricsCollector spoke.MetricsCollector, logger *slog.Logger) *MetricsHandler {
	return &MetricsHandler{
		BaseHandler:      NewBaseHandler(spoke.OperationGetMetrics, logger),
		metricsCollector: metricsCollector,
	}
}

// Handle processes metrics collection operations
func (h *MetricsHandler) Handle(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	h.logger.Debug("Processing metrics collection operation", "operation_id", operation.ID)

	if h.metricsCollector == nil {
		return h.createErrorResult(operation.ID, "metrics collector not available")
	}

	metrics, err := h.metricsCollector.CollectClusterMetrics(ctx)
	if err != nil {
		return h.createErrorResult(operation.ID, fmt.Sprintf("metrics collection failed: %v", err))
	}

	result := map[string]interface{}{
		"cluster_id":     metrics.ClusterID,
		"zone_id":        metrics.ZoneID,
		"resources":      metrics.Resources,
		"node_status":    metrics.NodeStatus,
		"last_collected": metrics.LastCollected,
	}

	return h.createSuccessResult(operation.ID, result)
}

// VMListHandler handles VM listing operations
type VMListHandler struct {
	*BaseHandler
	vmManager spoke.VMManager
}

// NewVMListHandler creates a new VM list handler
func NewVMListHandler(vmManager spoke.VMManager, logger *slog.Logger) *VMListHandler {
	return &VMListHandler{
		BaseHandler: NewBaseHandler(spoke.OperationListVMs, logger),
		vmManager:   vmManager,
	}
}

// Handle processes VM listing operations
func (h *VMListHandler) Handle(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	h.logger.Debug("Processing VM list operation", "operation_id", operation.ID)

	if h.vmManager == nil {
		return h.createErrorResult(operation.ID, "VM manager not available")
	}

	vms, err := h.vmManager.ListVMs(ctx)
	if err != nil {
		return h.createErrorResult(operation.ID, fmt.Sprintf("failed to list VMs: %v", err))
	}

	result := map[string]interface{}{
		"vms":   vms,
		"count": len(vms),
	}

	return h.createSuccessResult(operation.ID, result)
}
