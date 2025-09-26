package processor

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	k8sClient  client.Client
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

// SetK8sClient sets the Kubernetes client for the processor
func (p *Processor) SetK8sClient(k8sClient client.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.k8sClient = k8sClient
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
		Status:      spoke.OperationStatusCompleted,
		Result:      map[string]interface{}{"status": "cr_created"},
	}

	// Log detailed VDC creation data
	p.logger.Info("Received VDC creation request",
		"operation_id", operation.ID,
		"payload", operation.Payload)

	// Extract VDC details
	vdcName, _ := operation.Payload["vdc_name"].(string)
	targetNamespace, _ := operation.Payload["target_namespace"].(string)
	organization, _ := operation.Payload["organization"].(string)
	_, _ = operation.Payload["display_name"].(string) // displayName extracted in createVDCCustomResource
	_, _ = operation.Payload["description"].(string)  // description extracted in createVDCCustomResource
	zoneID, _ := operation.Payload["zone_id"].(string)

	p.logger.Info("VDC creation details",
		"vdc_name", vdcName,
		"target_namespace", targetNamespace,
		"organization", organization,
		"zone_id", zoneID,
		"quota", operation.Payload["quota"])

	// Try new CR-based approach first, fallback to direct VDC manager
	if p.k8sClient != nil {
		if err := p.createVDCCustomResource(ctx, operation); err != nil {
			p.logger.Warn("Failed to create VDC CR, falling back to direct VDC manager",
				"operation_id", operation.ID, "error", err)
			return p.handleCreateVDCLegacy(ctx, operation)
		}
		p.logger.Info("VDC CR created successfully, controller will handle the rest",
			"operation_id", operation.ID, "vdc_name", vdcName)
		return result
	}

	// Fallback to legacy approach
	p.logger.Info("Kubernetes client not available, using legacy VDC manager approach",
		"operation_id", operation.ID)
	return p.handleCreateVDCLegacy(ctx, operation)
}

// createVDCCustomResource creates a VirtualDataCenter custom resource
func (p *Processor) createVDCCustomResource(ctx context.Context, operation *spoke.Operation) error {
	// Extract VDC details
	vdcName, _ := operation.Payload["vdc_name"].(string)
	targetNamespace, _ := operation.Payload["target_namespace"].(string)
	organization, _ := operation.Payload["organization"].(string)
	displayName, _ := operation.Payload["display_name"].(string)
	description, _ := operation.Payload["description"].(string)
	zoneID, _ := operation.Payload["zone_id"].(string)

	// Parse quota information
	var cpuQuota, memoryQuota, storageQuota string = "1", "1Gi", "1Ti" // defaults
	if quotaMap, ok := operation.Payload["quota"].(map[string]interface{}); ok {
		if cpu, ok := quotaMap["cpu"].(float64); ok {
			cpuQuota = strconv.FormatInt(int64(cpu), 10)
		}
		if memory, ok := quotaMap["memory"].(float64); ok {
			memoryQuota = fmt.Sprintf("%.0fGi", memory)
		} else if memoryStr, ok := quotaMap["memory"].(string); ok {
			memoryQuota = memoryStr
		}
		if storage, ok := quotaMap["storage"].(float64); ok {
			// Convert storage number to Ti format for CRD validation
			if storage < 1024 {
				storageQuota = "1Ti" // Minimum 1Ti
			} else {
				storageQuota = fmt.Sprintf("%.0fTi", storage/1024.0)
			}
		} else if storageStr, ok := quotaMap["storage"].(string); ok {
			// Convert storage to Ti format if needed
			if strings.HasSuffix(storageStr, "Gi") {
				// Convert Gi to Ti (1024 Gi = 1 Ti)
				giValue := strings.TrimSuffix(storageStr, "Gi")
				if gi, err := strconv.Atoi(giValue); err == nil {
					ti := float64(gi) / 1024.0
					if ti < 1 {
						storageQuota = "1Ti" // Minimum 1Ti
					} else {
						storageQuota = fmt.Sprintf("%.0fTi", ti)
					}
				} else {
					storageQuota = "1Ti"
				}
			} else {
				storageQuota = storageStr
			}
		}
	}

	// Determine organization namespace
	orgNamespace := fmt.Sprintf("org-%s", organization)

	// Ensure organization namespace exists first
	if err := p.ensureOrganizationNamespace(ctx, orgNamespace); err != nil {
		return fmt.Errorf("failed to ensure organization namespace: %w", err)
	}

	// Create VirtualDataCenter CR
	vdc := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vdcName,
			Namespace: orgNamespace,
			Labels: map[string]string{
				"ovim.io/organization": organization,
				"ovim.io/zone-id":      zoneID,
				"ovim.io/managed-by":   "spoke-agent",
			},
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: organization,
			ZoneID:          zoneID,
			DisplayName:     displayName,
			Description:     description,
			Quota: ovimv1.ResourceQuota{
				CPU:     cpuQuota,
				Memory:  memoryQuota,
				Storage: storageQuota,
			},
			VDCType:               ovimv1.VDCTypeSpokeLocal,
			HubOperationID:        operation.ID,
			OrgNamespace:          orgNamespace,
			TargetNamespace:       targetNamespace,
			ReconcileUntilSuccess: true,
		},
	}

	// Create the VDC CR
	if err := p.k8sClient.Create(ctx, vdc); err != nil {
		return fmt.Errorf("failed to create VDC custom resource: %w", err)
	}

	p.logger.Info("VirtualDataCenter CR created",
		"vdc_name", vdcName,
		"namespace", orgNamespace,
		"operation_id", operation.ID)

	return nil
}

// handleCreateVDCLegacy handles VDC creation using the legacy VDC manager approach
func (p *Processor) handleCreateVDCLegacy(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusCompleted,
		Result:      map[string]interface{}{"status": "legacy_created"},
	}

	// Extract VDC details
	vdcName, _ := operation.Payload["vdc_name"].(string)
	targetNamespace, _ := operation.Payload["target_namespace"].(string)
	organization, _ := operation.Payload["organization"].(string)
	displayName, _ := operation.Payload["display_name"].(string)
	description, _ := operation.Payload["description"].(string)

	// Check if VDC manager is available
	if p.vdcManager == nil {
		p.logger.Error("VDC manager not available, cannot create VDC", "operation_id", operation.ID)
		result.Status = spoke.OperationStatusFailed
		result.Error = "VDC manager not configured"
		return result
	}

	// Parse quota information
	var cpuQuota, memoryQuota, storageQuota int64 = 1, 1, 10 // defaults
	if quotaMap, ok := operation.Payload["quota"].(map[string]interface{}); ok {
		if cpu, ok := quotaMap["cpu"].(float64); ok {
			cpuQuota = int64(cpu)
		}
		if memory, ok := quotaMap["memory"].(float64); ok {
			memoryQuota = int64(memory)
		}
		if storage, ok := quotaMap["storage"].(float64); ok {
			storageQuota = int64(storage)
		}
	}

	// Parse LimitRange information
	var minCPU, maxCPU, minMemory, maxMemory *int
	if limitRangeMap, ok := operation.Payload["limit_range"].(map[string]interface{}); ok {
		if val, ok := limitRangeMap["min_cpu"].(float64); ok {
			minCPUVal := int(val)
			minCPU = &minCPUVal
		}
		if val, ok := limitRangeMap["max_cpu"].(float64); ok {
			maxCPUVal := int(val)
			maxCPU = &maxCPUVal
		}
		if val, ok := limitRangeMap["min_memory"].(float64); ok {
			minMemoryVal := int(val)
			minMemory = &minMemoryVal
		}
		if val, ok := limitRangeMap["max_memory"].(float64); ok {
			maxMemoryVal := int(val)
			maxMemory = &maxMemoryVal
		}
	}

	// Create VDC request
	vdcReq := &spoke.VDCCreateRequest{
		Name:             targetNamespace,
		OrganizationName: organization,
		CPUQuota:         cpuQuota,
		MemoryQuota:      memoryQuota,
		StorageQuota:     storageQuota,
		NetworkPolicy:    "isolated", // default policy
		MinCPU:           minCPU,
		MaxCPU:           maxCPU,
		MinMemory:        minMemory,
		MaxMemory:        maxMemory,
		Labels: map[string]string{
			"ovim.io/vdc-name":     vdcName,
			"ovim.io/display-name": sanitizeForLabel(displayName),
		},
		Annotations: map[string]string{
			"ovim.io/description": description,
		},
	}

	// Create VDC using the VDC manager
	vdcStatus, err := p.vdcManager.CreateVDC(ctx, vdcReq)
	if err != nil {
		p.logger.Error("Failed to create VDC", "operation_id", operation.ID, "error", err)
		result.Status = spoke.OperationStatusFailed
		result.Error = err.Error()
		return result
	}

	p.logger.Info("VDC creation completed successfully",
		"operation_id", operation.ID,
		"namespace", vdcStatus.Namespace,
		"status", vdcStatus.Status)

	return result
}

func (p *Processor) handleDeleteVDC(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusCompleted,
		Result:      map[string]interface{}{"status": "cr_deleted"},
	}

	// Log detailed VDC deletion data
	p.logger.Info("Received VDC deletion request",
		"operation_id", operation.ID,
		"payload", operation.Payload)

	// Extract VDC details
	vdcName, _ := operation.Payload["vdc_name"].(string)
	targetNamespace, _ := operation.Payload["target_namespace"].(string)
	organization, _ := operation.Payload["organization"].(string)

	p.logger.Info("VDC deletion details",
		"vdc_name", vdcName,
		"target_namespace", targetNamespace,
		"organization", organization,
		"operation_id", operation.ID)

	// Try new CR-based approach first, fallback to direct VDC manager
	if p.k8sClient != nil {
		if err := p.deleteVDCCustomResource(ctx, operation); err != nil {
			p.logger.Warn("Failed to delete VDC CR, falling back to direct VDC manager",
				"operation_id", operation.ID, "error", err)
			return p.handleDeleteVDCLegacy(ctx, operation)
		}
		p.logger.Info("VDC CR deleted successfully, controller will handle the cleanup",
			"operation_id", operation.ID, "vdc_name", vdcName)
		return result
	}

	// Fallback to legacy approach
	p.logger.Info("Kubernetes client not available, using legacy VDC manager approach",
		"operation_id", operation.ID)
	return p.handleDeleteVDCLegacy(ctx, operation)
}

// deleteVDCCustomResource deletes a VirtualDataCenter custom resource
func (p *Processor) deleteVDCCustomResource(ctx context.Context, operation *spoke.Operation) error {
	// Extract VDC details
	vdcName, _ := operation.Payload["vdc_name"].(string)
	organization, _ := operation.Payload["organization"].(string)
	orgNamespace := fmt.Sprintf("org-%s", organization)

	// Get the VDC CR first to check if it exists
	vdc := &ovimv1.VirtualDataCenter{}
	err := p.k8sClient.Get(ctx, types.NamespacedName{
		Name:      vdcName,
		Namespace: orgNamespace,
	}, vdc)

	if err != nil {
		// If not found, consider it already deleted
		if client.IgnoreNotFound(err) == nil {
			p.logger.Info("VDC CR not found, considering deletion successful",
				"vdc_name", vdcName,
				"namespace", orgNamespace,
				"operation_id", operation.ID)
			return nil
		}
		return fmt.Errorf("failed to get VDC custom resource: %w", err)
	}

	// Update the VDC with the hub operation ID before deletion for tracking
	if vdc.Spec.HubOperationID != operation.ID {
		vdc.Spec.HubOperationID = operation.ID
		if err := p.k8sClient.Update(ctx, vdc); err != nil {
			p.logger.Warn("Failed to update VDC operation ID, proceeding with deletion",
				"operation_id", operation.ID, "error", err)
		}
	}

	// Delete the VDC CR
	if err := p.k8sClient.Delete(ctx, vdc); err != nil {
		return fmt.Errorf("failed to delete VDC custom resource: %w", err)
	}

	p.logger.Info("VirtualDataCenter CR deletion initiated",
		"vdc_name", vdcName,
		"namespace", orgNamespace,
		"operation_id", operation.ID)

	return nil
}

// handleDeleteVDCLegacy handles VDC deletion using the legacy VDC manager approach
func (p *Processor) handleDeleteVDCLegacy(ctx context.Context, operation *spoke.Operation) *spoke.OperationResult {
	result := &spoke.OperationResult{
		OperationID: operation.ID,
		Status:      spoke.OperationStatusCompleted,
		Result:      map[string]interface{}{"status": "legacy_deleted"},
	}

	// Extract VDC details
	vdcName, _ := operation.Payload["vdc_name"].(string)
	targetNamespace, _ := operation.Payload["target_namespace"].(string)
	organization, _ := operation.Payload["organization"].(string)
	vmCount, _ := operation.Payload["vm_count"].(float64)

	p.logger.Info("VDC deletion details",
		"vdc_name", vdcName,
		"target_namespace", targetNamespace,
		"organization", organization,
		"zone_id", operation.Payload["zone_id"],
		"vm_count", int(vmCount))

	// Check if VDC manager is available
	if p.vdcManager == nil {
		p.logger.Error("VDC manager not available, cannot delete VDC", "operation_id", operation.ID)
		result.Status = spoke.OperationStatusFailed
		result.Error = "VDC manager not configured"
		return result
	}

	// Handle VMs in VDC namespace
	if int(vmCount) > 0 {
		// TODO: Decide what to do with VMs in VDC during deletion:
		// Option 1: Force delete all VMs first
		// Option 2: Move VMs to a default VDC
		// Option 3: Fail deletion and require manual VM handling
		// For now, we'll log the issue and proceed with VDC deletion
		p.logger.Warn("VDC deletion requested but namespace contains VMs - TODO: implement VM handling strategy",
			"operation_id", operation.ID,
			"namespace", targetNamespace,
			"vm_count", int(vmCount))

		// Could add result metadata about VMs requiring attention
		result.Result = map[string]interface{}{
			"status":   "deleted_with_warnings",
			"warnings": []string{fmt.Sprintf("VDC contained %d VMs - manual cleanup may be required", int(vmCount))},
		}
	}

	// Delete VDC using the VDC manager
	err := p.vdcManager.DeleteVDC(ctx, targetNamespace)
	if err != nil {
		// Check if the error is due to namespace not found - treat this as successful deletion
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "NotFound") {
			p.logger.Info("VDC namespace not found, treating as successful deletion",
				"operation_id", operation.ID,
				"namespace", targetNamespace,
				"error", err.Error())

			// Update result to indicate successful deletion with note
			result.Result = map[string]interface{}{
				"status": "deleted",
				"note":   "VDC resources were not present on spoke cluster",
			}
		} else {
			// For other errors, fail the operation
			p.logger.Error("Failed to delete VDC", "operation_id", operation.ID, "error", err)
			result.Status = spoke.OperationStatusFailed
			result.Error = err.Error()
			return result
		}
	}

	p.logger.Info("VDC deletion completed successfully",
		"operation_id", operation.ID,
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

// sanitizeForLabel sanitizes a string to be valid as a Kubernetes label value
// Kubernetes labels must be empty or alphanumeric, with '-', '_', or '.' allowed,
// and must start and end with an alphanumeric character
func sanitizeForLabel(input string) string {
	if input == "" {
		return ""
	}

	// Replace spaces and other invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-zA-Z0-9\-_.]+`)
	result := reg.ReplaceAllString(input, "-")

	// Remove leading and trailing non-alphanumeric characters
	result = strings.Trim(result, "-_.")

	// Ensure it starts and ends with alphanumeric
	if result == "" {
		return "unknown"
	}

	// Limit length to 63 characters (Kubernetes limit)
	if len(result) > 63 {
		result = result[:63]
		// Ensure it still ends with alphanumeric
		result = strings.TrimRight(result, "-_.")
	}

	// If empty after trimming, return a default
	if result == "" {
		return "unknown"
	}

	return result
}

// ensureOrganizationNamespace creates an organization namespace if it doesn't exist
func (p *Processor) ensureOrganizationNamespace(ctx context.Context, orgNamespace string) error {
	if p.k8sClient == nil {
		return fmt.Errorf("kubernetes client not available")
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: orgNamespace,
			Labels: map[string]string{
				"ovim.io/namespace-type": "organization",
				"ovim.io/managed-by":     "spoke-agent",
			},
		},
	}

	if err := p.k8sClient.Create(ctx, ns); err != nil {
		if errors.IsAlreadyExists(err) {
			p.logger.Debug("Organization namespace already exists", "namespace", orgNamespace)
			return nil
		}
		return fmt.Errorf("failed to create organization namespace %s: %w", orgNamespace, err)
	}

	p.logger.Info("Created organization namespace", "namespace", orgNamespace)
	return nil
}
