package processor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
)

// Processor implements the OperationProcessor interface
type Processor struct {
	config   *config.SpokeConfig
	logger   *slog.Logger
	handlers map[spoke.OperationType]spoke.OperationHandler
	mu       sync.RWMutex

	// Components
	vmManager  spoke.VMManager
	vdcManager spoke.VDCManager
}

// NewProcessor creates a new operation processor
func NewProcessor(cfg *config.SpokeConfig, logger *slog.Logger) *Processor {
	return &Processor{
		config:   cfg,
		logger:   logger.With("component", "operation-processor"),
		handlers: make(map[spoke.OperationType]spoke.OperationHandler),
	}
}

// SetVMManager sets the VM manager for the processor
func (p *Processor) SetVMManager(vmManager spoke.VMManager) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.vmManager = vmManager
}

// SetVDCManager sets the VDC manager for the processor
func (p *Processor) SetVDCManager(vdcManager spoke.VDCManager) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.vdcManager = vdcManager
}

// ProcessOperation processes a single operation from the hub
func (p *Processor) ProcessOperation(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	startTime := time.Now()
	p.logger.Info("Processing operation", "operation_id", operation.ID, "type", operation.Type)

	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusRunning,
		Timestamp:   startTime,
	}

	// Check if we have a custom handler for this operation type
	p.mu.RLock()
	handler, hasHandler := p.handlers[operation.Type]
	p.mu.RUnlock()

	if hasHandler {
		p.logger.Debug("Using custom handler", "operation_type", operation.Type)
		result = handler.Handle(ctx, operation)
	} else {
		// Use built-in handlers
		switch operation.Type {
		case spoke.OperationCreateVM:
			result = p.handleCreateVM(ctx, operation)
		case spoke.OperationDeleteVM:
			result = p.handleDeleteVM(ctx, operation)
		case spoke.OperationStartVM:
			result = p.handleStartVM(ctx, operation)
		case spoke.OperationStopVM:
			result = p.handleStopVM(ctx, operation)
		case spoke.OperationGetVMStatus:
			result = p.handleGetVMStatus(ctx, operation)
		case spoke.OperationListVMs:
			result = p.handleListVMs(ctx, operation)
		case spoke.OperationGetHealth:
			result = p.handleGetHealth(ctx, operation)
		case spoke.OperationGetMetrics:
			result = p.handleGetMetrics(ctx, operation)
		case spoke.OperationCreateVDC:
			result = p.handleCreateVDC(ctx, operation)
		case spoke.OperationDeleteVDC:
			result = p.handleDeleteVDC(ctx, operation)
		case spoke.OperationSyncTemplates:
			result = p.handleSyncTemplates(ctx, operation)
		default:
			result.Status = spoke.OperationStatusFailed
			result.Error = fmt.Sprintf("unsupported operation type: %s", operation.Type)
		}
	}

	// Set completion timestamp and duration
	result.Timestamp = time.Now()
	result.Duration = time.Since(startTime)

	p.logger.Info("Operation completed",
		"operation_id", operation.ID,
		"status", result.Status,
		"duration", result.Duration)

	return result
}

// StartProcessing starts processing operations from the operations channel
func (p *Processor) StartProcessing(ctx context.Context, operations <-chan *spoke.Operation, results chan<- *spoke.OperationResult) error {
	p.logger.Info("Starting operation processing")

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Operation processing stopped due to context cancellation")
			return ctx.Err()
		case operation, ok := <-operations:
			if !ok {
				p.logger.Info("Operations channel closed, stopping processor")
				return nil
			}

			// Process operation in a goroutine to avoid blocking
			go func(op *spoke.Operation) {
				result := p.ProcessOperation(ctx, op)
				select {
				case results <- result:
					// Result sent successfully
				case <-ctx.Done():
					p.logger.Warn("Failed to send operation result due to context cancellation", "operation_id", op.ID)
				}
			}(operation)
		}
	}
}

// RegisterHandler registers a handler for a specific operation type
func (p *Processor) RegisterHandler(operationType spoke.OperationType, handler spoke.OperationHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[operationType] = handler
	p.logger.Info("Registered operation handler", "operation_type", operationType)
}

// Built-in operation handlers

func (p *Processor) handleCreateVM(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusFailed,
		Error:       "VM creation not yet implemented",
	}

	// TODO: Parse payload and call VM manager
	// For now, just return a placeholder
	p.logger.Info("VM creation requested but not implemented", "operation_id", operation.ID)

	return result
}

func (p *Processor) handleDeleteVM(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusFailed,
		Error:       "VM deletion not yet implemented",
	}

	// TODO: Parse payload and call VM manager
	p.logger.Info("VM deletion requested but not implemented", "operation_id", operation.ID)

	return result
}

func (p *Processor) handleStartVM(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusFailed,
		Error:       "VM start not yet implemented",
	}

	// TODO: Parse payload and call VM manager
	p.logger.Info("VM start requested but not implemented", "operation_id", operation.ID)

	return result
}

func (p *Processor) handleStopVM(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusFailed,
		Error:       "VM stop not yet implemented",
	}

	// TODO: Parse payload and call VM manager
	p.logger.Info("VM stop requested but not implemented", "operation_id", operation.ID)

	return result
}

func (p *Processor) handleGetVMStatus(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusFailed,
		Error:       "VM status not yet implemented",
	}

	// TODO: Parse payload and call VM manager
	p.logger.Info("VM status requested but not implemented", "operation_id", operation.ID)

	return result
}

func (p *Processor) handleListVMs(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusCompleted,
		Result:      map[string]interface{}{"vms": []spoke.VMStatus{}},
	}

	// TODO: Call VM manager to get actual VMs
	p.logger.Info("VM list requested, returning empty list", "operation_id", operation.ID)

	return result
}

func (p *Processor) handleGetHealth(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusCompleted,
		Result: map[string]interface{}{
			"status":  "healthy",
			"message": "spoke agent is running",
		},
	}

	p.logger.Debug("Health check requested", "operation_id", operation.ID)
	return result
}

func (p *Processor) handleGetMetrics(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusCompleted,
		Result: map[string]interface{}{
			"cluster_metrics": map[string]interface{}{
				"cluster_id": p.config.ClusterID,
				"zone_id":    p.config.ZoneID,
				"node_count": 0,
				"vm_count":   0,
			},
		},
	}

	// TODO: Collect actual metrics
	p.logger.Debug("Metrics requested, returning placeholder", "operation_id", operation.ID)
	return result
}

func (p *Processor) handleCreateVDC(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusFailed,
		Error:       "VDC creation not yet implemented",
	}

	// TODO: Parse payload and call VDC manager
	p.logger.Info("VDC creation requested but not implemented", "operation_id", operation.ID)

	return result
}

func (p *Processor) handleDeleteVDC(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusFailed,
		Error:       "VDC deletion not yet implemented",
	}

	// TODO: Parse payload and call VDC manager
	p.logger.Info("VDC deletion requested but not implemented", "operation_id", operation.ID)

	return result
}

func (p *Processor) handleSyncTemplates(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusCompleted,
		Result:      map[string]interface{}{"synced_templates": 0},
	}

	// TODO: Parse payload and sync templates
	p.logger.Info("Template sync requested but not implemented", "operation_id", operation.ID)

	return result
}
