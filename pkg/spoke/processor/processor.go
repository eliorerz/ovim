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
	}

	// Check if VDC manager is available
	p.mu.RLock()
	vdcManager := p.vdcManager
	p.mu.RUnlock()

	if vdcManager == nil {
		result.Error = "VDC manager not available"
		p.logger.Error("VDC creation failed: VDC manager not available", "operation_id", operation.ID)
		return result
	}

	// Parse VDC creation request from payload
	vdcName, ok := operation.Payload["vdc_name"].(string)
	if !ok {
		result.Error = "missing or invalid vdc_name in payload"
		p.logger.Error("VDC creation failed: missing vdc_name", "operation_id", operation.ID)
		return result
	}

	targetNamespace, ok := operation.Payload["target_namespace"].(string)
	if !ok {
		result.Error = "missing or invalid target_namespace in payload"
		p.logger.Error("VDC creation failed: missing target_namespace", "operation_id", operation.ID)
		return result
	}

	p.logger.Info("Creating VDC in spoke cluster",
		"operation_id", operation.ID,
		"vdc_name", vdcName,
		"target_namespace", targetNamespace)

	// Create VDC request
	vdcRequest := &spoke.VDCCreateRequest{
		Name:             vdcName,
		OrganizationName: getStringFromPayload(operation.Payload, "organization"),
		NetworkPolicy:    getStringFromPayload(operation.Payload, "network_policy"),
	}

	// Parse quota information
	if quota, ok := operation.Payload["quota"].(map[string]interface{}); ok {
		vdcRequest.CPUQuota = getInt64FromPayload(quota, "cpu")
		vdcRequest.MemoryQuota = getInt64FromPayload(quota, "memory")
		vdcRequest.StorageQuota = getInt64FromPayload(quota, "storage")
	}

	// Create VDC through manager
	vdcStatus, err := vdcManager.CreateVDC(ctx, vdcRequest)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create VDC: %v", err)
		p.logger.Error("VDC creation failed", "operation_id", operation.ID, "error", err)
		return result
	}

	// Success
	result.Status = spoke.OperationStatusCompleted
	result.Result = map[string]interface{}{
		"vdc_status": vdcStatus,
		"message":    "VDC created successfully",
	}

	p.logger.Info("VDC created successfully",
		"operation_id", operation.ID,
		"vdc_name", vdcName,
		"namespace", targetNamespace)

	return result
}

func (p *Processor) handleDeleteVDC(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusFailed,
	}

	// Check if VDC manager is available
	p.mu.RLock()
	vdcManager := p.vdcManager
	p.mu.RUnlock()

	if vdcManager == nil {
		result.Error = "VDC manager not available"
		p.logger.Error("VDC deletion failed: VDC manager not available", "operation_id", operation.ID)
		return result
	}

	// Parse VDC deletion request from payload
	vdcName, ok := operation.Payload["vdc_name"].(string)
	if !ok {
		result.Error = "missing or invalid vdc_name in payload"
		p.logger.Error("VDC deletion failed: missing vdc_name", "operation_id", operation.ID)
		return result
	}

	targetNamespace, ok := operation.Payload["target_namespace"].(string)
	if !ok {
		result.Error = "missing or invalid target_namespace in payload"
		p.logger.Error("VDC deletion failed: missing target_namespace", "operation_id", operation.ID)
		return result
	}

	p.logger.Info("Deleting VDC from spoke cluster",
		"operation_id", operation.ID,
		"vdc_name", vdcName,
		"target_namespace", targetNamespace)

	// Delete VDC through manager
	err := vdcManager.DeleteVDC(ctx, targetNamespace)
	if err != nil {
		result.Error = fmt.Sprintf("failed to delete VDC: %v", err)
		p.logger.Error("VDC deletion failed", "operation_id", operation.ID, "error", err)
		return result
	}

	// Success
	result.Status = spoke.OperationStatusCompleted
	result.Result = map[string]interface{}{
		"message": "VDC deleted successfully",
	}

	p.logger.Info("VDC deleted successfully",
		"operation_id", operation.ID,
		"vdc_name", vdcName,
		"namespace", targetNamespace)

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

// Helper functions for payload parsing

func getStringFromPayload(payload map[string]interface{}, key string) string {
	if value, ok := payload[key].(string); ok {
		return value
	}
	return ""
}

func getIntFromPayload(payload map[string]interface{}, key string) int {
	if value, ok := payload[key].(int); ok {
		return value
	}
	if value, ok := payload[key].(float64); ok {
		return int(value)
	}
	return 0
}

func getInt64FromPayload(payload map[string]interface{}, key string) int64 {
	if value, ok := payload[key].(int); ok {
		return int64(value)
	}
	if value, ok := payload[key].(int64); ok {
		return value
	}
	if value, ok := payload[key].(float64); ok {
		return int64(value)
	}
	return 0
}
