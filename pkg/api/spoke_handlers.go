package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// SpokeHandlers handles API requests from spoke agents
type SpokeHandlers struct {
	storage   storage.Storage
	k8sClient client.Client

	// In-memory operation queue for spoke agents
	// In production, this should be replaced with a persistent message queue
	operationQueues map[string][]*SpokeOperation
	operationsMutex sync.RWMutex

	// Store operation results
	operationResults map[string]*SpokeOperationResult
	resultsMutex     sync.RWMutex

	// Store agent status reports
	agentStatuses map[string]*SpokeStatusReport
	statusMutex   sync.RWMutex
}

// SpokeStatusReport represents a status report from a spoke agent
type SpokeStatusReport struct {
	AgentID        string                 `json:"agent_id"`
	ClusterID      string                 `json:"cluster_id"`
	ZoneID         string                 `json:"zone_id"`
	Status         string                 `json:"status"`
	Version        string                 `json:"version"`
	Metrics        map[string]interface{} `json:"metrics"`
	VDCs           []interface{}          `json:"vdcs"`
	VMs            []interface{}          `json:"vms"`
	LastHubContact time.Time              `json:"last_hub_contact"`
	ReportTime     time.Time              `json:"report_time"`
	Errors         []string               `json:"errors,omitempty"`
}

// SpokeOperation represents an operation to be sent to a spoke agent
type SpokeOperation struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	Timestamp   time.Time              `json:"timestamp"`
	RetryCount  int                    `json:"retry_count,omitempty"`
	TimeoutSecs int                    `json:"timeout_seconds,omitempty"`
}

// SpokeOperationResult represents the result of an operation from a spoke agent
type SpokeOperationResult struct {
	OperationID string                 `json:"operation_id"`
	Status      string                 `json:"status"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Duration    time.Duration          `json:"duration,omitempty"`
}

// NewSpokeHandlers creates a new spoke handlers instance
func NewSpokeHandlers(storage storage.Storage, k8sClient client.Client) *SpokeHandlers {
	return &SpokeHandlers{
		storage:          storage,
		k8sClient:        k8sClient,
		operationQueues:  make(map[string][]*SpokeOperation),
		operationResults: make(map[string]*SpokeOperationResult),
		agentStatuses:    make(map[string]*SpokeStatusReport),
	}
}

// GetZoneAgentStatus returns the status of spoke agent for a specific zone
func (h *SpokeHandlers) GetZoneAgentStatus(zoneID string) *SpokeStatusReport {
	h.statusMutex.RLock()
	defer h.statusMutex.RUnlock()

	// Find agent by zone ID
	for _, status := range h.agentStatuses {
		if status.ZoneID == zoneID {
			return status
		}
	}
	return nil
}

// GetAllZoneStatuses returns a map of zone IDs to their spoke agent status
func (h *SpokeHandlers) GetAllZoneStatuses() map[string]*SpokeStatusReport {
	h.statusMutex.RLock()
	defer h.statusMutex.RUnlock()

	zoneStatuses := make(map[string]*SpokeStatusReport)
	for _, status := range h.agentStatuses {
		zoneStatuses[status.ZoneID] = status
	}
	return zoneStatuses
}

// HandleStatusReport handles status reports from spoke agents
// POST /api/v1/spoke/status
func (h *SpokeHandlers) HandleStatusReport(c *gin.Context) {
	var report SpokeStatusReport
	if err := c.ShouldBindJSON(&report); err != nil {
		klog.Errorf("Failed to bind status report from %s: %v", c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate required fields
	if report.AgentID == "" || report.ClusterID == "" || report.ZoneID == "" {
		klog.Warningf("Status report missing required fields from %s: agent_id=%s, cluster_id=%s, zone_id=%s",
			c.ClientIP(), report.AgentID, report.ClusterID, report.ZoneID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields: agent_id, cluster_id, zone_id"})
		return
	}

	// Update timestamp
	report.ReportTime = time.Now()
	report.LastHubContact = time.Now()

	// Store the status report
	h.statusMutex.Lock()
	previousReport := h.agentStatuses[report.AgentID]
	h.agentStatuses[report.AgentID] = &report
	h.statusMutex.Unlock()

	if previousReport == nil {
		klog.Infof("First status report from spoke agent %s (cluster: %s, zone: %s, status: %s, version: %s, vm_count: %d, vdc_count: %d)",
			report.AgentID, report.ClusterID, report.ZoneID, report.Status, report.Version, len(report.VMs), len(report.VDCs))
	} else {
		statusChanged := previousReport.Status != report.Status
		if statusChanged {
			klog.Infof("Status changed for spoke agent %s (cluster: %s, zone: %s): %s -> %s (vm_count: %d, vdc_count: %d)",
				report.AgentID, report.ClusterID, report.ZoneID, previousReport.Status, report.Status, len(report.VMs), len(report.VDCs))
		} else {
			klog.Infof("Status report from spoke agent %s (cluster: %s, zone: %s, status: %s, vm_count: %d, vdc_count: %d)",
				report.AgentID, report.ClusterID, report.ZoneID, report.Status, len(report.VMs), len(report.VDCs))
		}
	}

	if len(report.Errors) > 0 {
		klog.Warningf("Spoke agent %s reported %d errors: %v", report.AgentID, len(report.Errors), report.Errors)
	}

	// TODO: Store in persistent storage/database
	// TODO: Update zone status based on spoke agent reports
	// TODO: Trigger alerts based on agent status

	c.JSON(http.StatusOK, gin.H{
		"status":  "received",
		"message": "Status report processed successfully",
	})
}

// GetOperations returns pending operations for a spoke agent
// GET /api/v1/spoke/operations?agent_id=X
func (h *SpokeHandlers) GetOperations(c *gin.Context) {
	agentID := c.Query("agent_id")
	if agentID == "" {
		klog.Warningf("GetOperations request missing agent_id parameter from %s", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing agent_id parameter"})
		return
	}

	klog.Infof("Spoke agent %s requesting pending operations from %s", agentID, c.ClientIP())

	h.operationsMutex.Lock()
	operations := h.operationQueues[agentID]
	// Clear the queue after reading
	h.operationQueues[agentID] = nil
	h.operationsMutex.Unlock()

	if len(operations) == 0 {
		klog.Infof("No pending operations for spoke agent %s", agentID)
		c.Status(http.StatusNoContent)
		return
	}

	klog.Infof("Sending %d operations to spoke agent %s: %v", len(operations), agentID, getOperationSummary(operations))
	c.JSON(http.StatusOK, operations)
}

// HandleOperationResult handles operation results from spoke agents
// POST /api/v1/spoke/operations/:operationId/result
func (h *SpokeHandlers) HandleOperationResult(c *gin.Context) {
	operationID := c.Param("operationId")
	if operationID == "" {
		klog.Warningf("HandleOperationResult request missing operation ID from %s", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing operation ID"})
		return
	}

	agentID := c.GetString("agent_id")
	klog.Infof("Receiving operation result for %s from spoke agent %s (%s)", operationID, agentID, c.ClientIP())

	var result SpokeOperationResult
	if err := c.ShouldBindJSON(&result); err != nil {
		klog.Errorf("Failed to bind operation result for %s from agent %s: %v", operationID, agentID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Ensure the operation ID matches
	result.OperationID = operationID
	result.Timestamp = time.Now()

	// Store the result
	h.resultsMutex.Lock()
	h.operationResults[operationID] = &result
	h.resultsMutex.Unlock()

	if result.Status == "success" {
		klog.Infof("Operation %s completed successfully by agent %s (duration: %v)", operationID, agentID, result.Duration)
	} else {
		klog.Warningf("Operation %s failed on agent %s: status=%s, error=%s (duration: %v)", operationID, agentID, result.Status, result.Error, result.Duration)
	}

	// Process operation result based on type
	go h.processOperationResult(&result)

	c.JSON(http.StatusOK, gin.H{
		"status":  "received",
		"message": "Operation result processed successfully",
	})
}

// QueueOperation queues an operation for a spoke agent (for testing purposes)
// POST /api/v1/spoke/operations/queue
func (h *SpokeHandlers) QueueOperation(c *gin.Context) {
	var request struct {
		AgentID     string                 `json:"agent_id"`
		Type        string                 `json:"type"`
		Payload     map[string]interface{} `json:"payload"`
		TimeoutSecs int                    `json:"timeout_seconds,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		klog.Errorf("Failed to bind queue operation request from %s: %v", c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if request.AgentID == "" || request.Type == "" {
		klog.Warningf("Queue operation request missing required fields from %s: agent_id=%s, type=%s", c.ClientIP(), request.AgentID, request.Type)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields: agent_id, type"})
		return
	}

	// Create operation
	operation := &SpokeOperation{
		ID:          generateOperationID(),
		Type:        request.Type,
		Payload:     request.Payload,
		Timestamp:   time.Now(),
		TimeoutSecs: request.TimeoutSecs,
	}

	// Queue the operation
	h.operationsMutex.Lock()
	queueLength := len(h.operationQueues[request.AgentID])
	h.operationQueues[request.AgentID] = append(h.operationQueues[request.AgentID], operation)
	h.operationsMutex.Unlock()

	klog.Infof("Queued operation %s for spoke agent %s (type: %s, timeout: %ds, queue_length: %d)",
		operation.ID, request.AgentID, request.Type, request.TimeoutSecs, queueLength+1)

	c.JSON(http.StatusOK, gin.H{
		"operation_id": operation.ID,
		"status":       "queued",
		"message":      "Operation queued successfully",
	})
}

// GetAgentStatus returns the status of spoke agents
// GET /api/v1/spoke/agents
func (h *SpokeHandlers) GetAgentStatus(c *gin.Context) {
	h.statusMutex.RLock()
	defer h.statusMutex.RUnlock()

	agents := make([]gin.H, 0, len(h.agentStatuses))
	for agentID, status := range h.agentStatuses {
		// Calculate if agent is stale (no report in last 5 minutes)
		isStale := time.Since(status.ReportTime) > 5*time.Minute
		agentStatus := status.Status
		if isStale {
			agentStatus = "stale"
		}

		agents = append(agents, gin.H{
			"agent_id":         agentID,
			"cluster_id":       status.ClusterID,
			"zone_id":          status.ZoneID,
			"status":           agentStatus,
			"version":          status.Version,
			"last_report":      status.ReportTime,
			"last_hub_contact": status.LastHubContact,
			"vm_count":         len(status.VMs),
			"vdc_count":        len(status.VDCs),
			"error_count":      len(status.Errors),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"total":  len(agents),
	})
}

// GetOperationResult returns the result of an operation
// GET /api/v1/spoke/operations/:operationId/result
func (h *SpokeHandlers) GetOperationResult(c *gin.Context) {
	operationID := c.Param("operationId")
	if operationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing operation ID"})
		return
	}

	h.resultsMutex.RLock()
	result, exists := h.operationResults[operationID]
	h.resultsMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation result not found"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// generateOperationID generates a unique operation ID
func generateOperationID() string {
	// Simple timestamp-based ID for demo purposes
	// In production, use UUID or similar
	return "op-" + time.Now().Format("20060102-150405") + "-" + time.Now().Format("000")
}

// getOperationSummary creates a summary of operations for logging
func getOperationSummary(operations []*SpokeOperation) []string {
	summary := make([]string, len(operations))
	for i, op := range operations {
		summary[i] = fmt.Sprintf("%s(%s)", op.Type, op.ID)
	}
	return summary
}

// spokeAuthMiddleware validates spoke agent authentication
func (h *SpokeHandlers) spokeAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract authentication headers
		agentID := c.GetHeader("X-Agent-ID")
		clusterID := c.GetHeader("X-Cluster-ID")
		zoneID := c.GetHeader("X-Zone-ID")
		version := c.GetHeader("X-Agent-Version")
		authHeader := c.GetHeader("Authorization")

		klog.Infof("Spoke agent authentication attempt: agent_id=%s, cluster_id=%s, zone_id=%s, version=%s, method=%s, path=%s",
			agentID, clusterID, zoneID, version, c.Request.Method, c.Request.URL.Path)

		// Basic validation
		if agentID == "" || clusterID == "" || zoneID == "" {
			klog.Warningf("Spoke agent authentication failed: missing required headers (agent_id=%s, cluster_id=%s, zone_id=%s)",
				agentID, clusterID, zoneID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing required headers"})
			c.Abort()
			return
		}

		// TODO: Implement proper authentication
		// - Validate JWT token or client certificates
		// - Check if agent is authorized for this cluster/zone
		// - Verify agent version compatibility

		// For now, just check for basic token
		if authHeader != "Bearer spoke-agent-token" {
			klog.Warningf("Spoke agent authentication failed: invalid token from agent %s (zone: %s)", agentID, zoneID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication"})
			c.Abort()
			return
		}

		klog.Infof("Spoke agent authenticated successfully: agent_id=%s, zone_id=%s", agentID, zoneID)

		// Store agent info in context for handlers
		c.Set("agent_id", agentID)
		c.Set("cluster_id", clusterID)
		c.Set("zone_id", zoneID)
		c.Set("agent_version", version)

		c.Next()
	}
}

// VDC operation processing functions for spoke agent communication

// processOperationResult processes operation results from spoke agents
func (h *SpokeHandlers) processOperationResult(result *SpokeOperationResult) {
	switch result.Status {
	case "success":
		klog.Infof("Operation %s completed successfully", result.OperationID)
		// Handle successful operation
	case "error", "failed":
		klog.Errorf("Operation %s failed: %s", result.OperationID, result.Error)
		// Handle failed operation - could trigger retries or alerts
	default:
		klog.Warningf("Unknown operation status for %s: %s", result.OperationID, result.Status)
	}
}

// QueueVDCCreation queues a VDC creation operation for a spoke agent
func (h *SpokeHandlers) QueueVDCCreation(agentID string, vdc *ovimv1.VirtualDataCenter) error {
	if h.k8sClient == nil {
		klog.Errorf("Cannot queue VDC creation operation %s for agent %s: Kubernetes client not available", vdc.Name, agentID)
		return fmt.Errorf("Kubernetes client not available")
	}

	targetNamespace := vdc.Annotations["ovim.io/target-namespace"]
	if targetNamespace == "" {
		targetNamespace = vdc.Status.Namespace
	}

	klog.Infof("Preparing VDC creation operation for agent %s: VDC=%s, target_namespace=%s, zone=%s, org=%s",
		agentID, vdc.Name, targetNamespace, vdc.Spec.ZoneID, vdc.Spec.OrganizationRef)

	operation := &SpokeOperation{
		ID:        generateOperationID(),
		Type:      "create_vdc",
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"vdc_name":         vdc.Name,
			"vdc_namespace":    vdc.Namespace,
			"target_namespace": targetNamespace,
			"organization":     vdc.Spec.OrganizationRef,
			"zone_id":          vdc.Spec.ZoneID,
			"quota": map[string]interface{}{
				"cpu":     vdc.Spec.Quota.CPU,
				"memory":  vdc.Spec.Quota.Memory,
				"storage": vdc.Spec.Quota.Storage,
			},
			"limit_range":           vdc.Spec.LimitRange,
			"network_policy":        vdc.Spec.NetworkPolicy,
			"custom_network_config": vdc.Spec.CustomNetworkConfig,
		},
	}

	h.operationsMutex.Lock()
	queueLength := len(h.operationQueues[agentID])
	h.operationQueues[agentID] = append(h.operationQueues[agentID], operation)
	h.operationsMutex.Unlock()

	klog.Infof("Queued VDC creation operation %s for agent %s (VDC: %s, target_ns: %s, cpu: %s, memory: %s, storage: %s, queue_length: %d)",
		operation.ID, agentID, vdc.Name, targetNamespace, vdc.Spec.Quota.CPU, vdc.Spec.Quota.Memory, vdc.Spec.Quota.Storage, queueLength+1)
	return nil
}

// QueueVDCDeletion queues a VDC deletion operation for a spoke agent
func (h *SpokeHandlers) QueueVDCDeletion(agentID string, vdcName, vdcNamespace, targetNamespace string) error {
	klog.Infof("Preparing VDC deletion operation for agent %s: VDC=%s, target_namespace=%s", agentID, vdcName, targetNamespace)

	operation := &SpokeOperation{
		ID:        generateOperationID(),
		Type:      "delete_vdc",
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"vdc_name":         vdcName,
			"vdc_namespace":    vdcNamespace,
			"target_namespace": targetNamespace,
			"cascade":          true, // Delete all associated resources
		},
	}

	h.operationsMutex.Lock()
	queueLength := len(h.operationQueues[agentID])
	h.operationQueues[agentID] = append(h.operationQueues[agentID], operation)
	h.operationsMutex.Unlock()

	klog.Infof("Queued VDC deletion operation %s for agent %s (VDC: %s, target_ns: %s, cascade: true, queue_length: %d)",
		operation.ID, agentID, vdcName, targetNamespace, queueLength+1)
	return nil
}

// MonitorVDCReplication monitors VDC resources for replication to spoke clusters
func (h *SpokeHandlers) MonitorVDCReplication() {
	if h.k8sClient == nil {
		klog.Errorf("Cannot start VDC replication monitoring: Kubernetes client not available")
		return
	}

	klog.Infof("Starting VDC replication monitoring (interval: 30s)")
	ctx := context.Background()
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.processVDCReplicationRequests(ctx)
		}
	}
}

// processVDCReplicationRequests finds VDCs requiring spoke replication
func (h *SpokeHandlers) processVDCReplicationRequests(ctx context.Context) {
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := h.k8sClient.List(ctx, vdcList); err != nil {
		klog.Errorf("Failed to list VDCs for replication monitoring: %v", err)
		return
	}

	totalVDCs := len(vdcList.Items)
	replicationCount := 0
	deletionCount := 0

	for _, vdc := range vdcList.Items {
		// Skip spoke VDCs to prevent infinite loops - only process hub VDCs
		if vdc.Spec.VDCType == ovimv1.VDCTypeSpoke {
			continue
		}

		// Check if VDC requires spoke replication
		if needsReplication, _ := vdc.Annotations["ovim.io/spoke-replication-required"]; needsReplication == "true" {
			replicationCount++
			klog.Infof("Processing VDC replication request: %s (zone: %s, org: %s)", vdc.Name, vdc.Spec.ZoneID, vdc.Spec.OrganizationRef)
			h.handleVDCReplication(ctx, &vdc)
		}

		// Check if VDC requires spoke deletion
		if needsDeletion, _ := vdc.Annotations["ovim.io/spoke-deletion-required"]; needsDeletion == "true" {
			deletionCount++
			klog.Infof("Processing VDC deletion request: %s (zone: %s, org: %s)", vdc.Name, vdc.Spec.ZoneID, vdc.Spec.OrganizationRef)
			h.handleVDCDeletion(ctx, &vdc)
		}
	}

	if replicationCount > 0 || deletionCount > 0 {
		klog.Infof("VDC replication monitoring cycle completed: %d VDCs checked, %d replication requests, %d deletion requests",
			totalVDCs, replicationCount, deletionCount)
	}
}

// handleVDCReplication handles VDC replication to spoke clusters
func (h *SpokeHandlers) handleVDCReplication(ctx context.Context, vdc *ovimv1.VirtualDataCenter) {
	zoneID := vdc.Spec.ZoneID
	if zoneID == "" {
		klog.Errorf("VDC %s (namespace: %s) has no zone ID for replication - cannot proceed", vdc.Name, vdc.Namespace)
		return
	}

	klog.Infof("Attempting VDC %s replication to zone %s (target_namespace: %s)",
		vdc.Name, zoneID, vdc.Annotations["ovim.io/target-namespace"])

	// Find spoke agent for this zone
	agentStatus := h.GetZoneAgentStatus(zoneID)
	if agentStatus == nil {
		klog.Warningf("No spoke agent found for zone %s, VDC %s replication postponed - waiting for agent to connect", zoneID, vdc.Name)
		return
	}

	klog.Infof("Found spoke agent %s for zone %s (status: %s, last_contact: %v)",
		agentStatus.AgentID, zoneID, agentStatus.Status, time.Since(agentStatus.ReportTime))

	// Check if agent is healthy
	lastContact := time.Since(agentStatus.ReportTime)
	if agentStatus.Status != "active" {
		klog.Warningf("Spoke agent %s for zone %s is not active (status: %s), VDC %s replication postponed",
			agentStatus.AgentID, zoneID, agentStatus.Status, vdc.Name)
		return
	}
	if lastContact > 2*time.Minute {
		klog.Warningf("Spoke agent %s for zone %s is stale (last_contact: %v ago), VDC %s replication postponed",
			agentStatus.AgentID, zoneID, lastContact, vdc.Name)
		return
	}

	klog.Infof("Spoke agent %s for zone %s is healthy, proceeding with VDC %s replication",
		agentStatus.AgentID, zoneID, vdc.Name)

	// Queue VDC creation operation for spoke agent
	if err := h.QueueVDCCreation(agentStatus.AgentID, vdc); err != nil {
		klog.Errorf("Failed to queue VDC creation for agent %s: %v", agentStatus.AgentID, err)
		return
	}

	// Mark replication as in progress
	vdcCopy := vdc.DeepCopy()
	if vdcCopy.Annotations == nil {
		vdcCopy.Annotations = make(map[string]string)
	}
	vdcCopy.Annotations["ovim.io/spoke-replication-required"] = "false"
	vdcCopy.Annotations["ovim.io/spoke-replication-in-progress"] = "true"
	vdcCopy.Annotations["ovim.io/spoke-replication-started"] = time.Now().Format(time.RFC3339)
	vdcCopy.Annotations["ovim.io/spoke-agent-id"] = agentStatus.AgentID

	if err := h.k8sClient.Update(ctx, vdcCopy); err != nil {
		klog.Errorf("Failed to update VDC %s replication status annotations: %v", vdc.Name, err)
		return
	}

	klog.Infof("VDC %s replication initiated successfully for zone %s via agent %s", vdc.Name, zoneID, agentStatus.AgentID)
}

// handleVDCDeletion handles VDC deletion from spoke clusters
func (h *SpokeHandlers) handleVDCDeletion(ctx context.Context, vdc *ovimv1.VirtualDataCenter) {
	zoneID := vdc.Spec.ZoneID
	if zoneID == "" {
		klog.Errorf("VDC %s has no zone ID for deletion", vdc.Name)
		return
	}

	// Find spoke agent for this zone
	agentStatus := h.GetZoneAgentStatus(zoneID)
	if agentStatus == nil {
		klog.Warningf("No spoke agent found for zone %s, proceeding with VDC %s deletion", zoneID, vdc.Name)
		// If no agent, assume spoke resources are already cleaned up
		return
	}

	targetNamespace, _ := vdc.Annotations["ovim.io/target-namespace"]
	if targetNamespace == "" {
		targetNamespace = vdc.Status.Namespace
	}

	// Queue VDC deletion operation for spoke agent
	if err := h.QueueVDCDeletion(agentStatus.AgentID, vdc.Name, vdc.Namespace, targetNamespace); err != nil {
		klog.Errorf("Failed to queue VDC deletion for agent %s: %v", agentStatus.AgentID, err)
		return
	}

	klog.Infof("VDC %s deletion queued for zone %s", vdc.Name, zoneID)
}

// CreateResourceQuotaInSpoke creates ResourceQuota in spoke cluster via agent
func (h *SpokeHandlers) CreateResourceQuotaInSpoke(agentID, namespace string, quota ovimv1.ResourceQuota) error {
	operation := &SpokeOperation{
		ID:        generateOperationID(),
		Type:      "create_resource_quota",
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"namespace": namespace,
			"quota": map[string]interface{}{
				"cpu":     quota.CPU,
				"memory":  quota.Memory,
				"storage": quota.Storage,
			},
		},
	}

	h.operationsMutex.Lock()
	h.operationQueues[agentID] = append(h.operationQueues[agentID], operation)
	h.operationsMutex.Unlock()

	klog.Infof("Queued ResourceQuota creation operation %s for agent %s (namespace: %s)", operation.ID, agentID, namespace)
	return nil
}

// CreateLimitRangeInSpoke creates LimitRange in spoke cluster via agent
func (h *SpokeHandlers) CreateLimitRangeInSpoke(agentID, namespace string, limitRange *ovimv1.LimitRange) error {
	if limitRange == nil {
		return nil // No limit range to create
	}

	operation := &SpokeOperation{
		ID:        generateOperationID(),
		Type:      "create_limit_range",
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"namespace": namespace,
			"limit_range": map[string]interface{}{
				"min_cpu":    limitRange.MinCpu,
				"max_cpu":    limitRange.MaxCpu,
				"min_memory": limitRange.MinMemory,
				"max_memory": limitRange.MaxMemory,
			},
		},
	}

	h.operationsMutex.Lock()
	h.operationQueues[agentID] = append(h.operationQueues[agentID], operation)
	h.operationsMutex.Unlock()

	klog.Infof("Queued LimitRange creation operation %s for agent %s (namespace: %s)", operation.ID, agentID, namespace)
	return nil
}

// StartVDCMonitoring starts the VDC replication monitoring in a separate goroutine
func (h *SpokeHandlers) StartVDCMonitoring() {
	go h.MonitorVDCReplication()
	klog.Info("Started VDC replication monitoring")
}

// GetSpokeAgent gets the spoke agent information for a given zone ID
func (h *SpokeHandlers) GetSpokeAgent(zoneID string) (*SpokeStatusReport, error) {
	h.statusMutex.RLock()
	defer h.statusMutex.RUnlock()

	// Look for spoke agent by zone ID
	for _, status := range h.agentStatuses {
		if status.ZoneID == zoneID {
			return status, nil
		}
	}

	return nil, fmt.Errorf("no spoke agent found for zone %s", zoneID)
}

// VDCStatusResponse represents VDC status information from spoke agent
type VDCStatusResponse struct {
	Namespace     string                 `json:"namespace"`
	Status        string                 `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
	ResourceUsage map[string]interface{} `json:"resource_usage"`
}

// GetVDCStatus queries a spoke agent for VDC status information
func (h *SpokeHandlers) GetVDCStatus(spokeAgent *SpokeStatusReport, namespace string) (*VDCStatusResponse, error) {
	if spokeAgent == nil {
		return nil, fmt.Errorf("spoke agent is nil")
	}

	// For now, we'll look in the VDCs field of the status report
	// In a production system, this might be a separate API call to the spoke agent
	for _, vdcInterface := range spokeAgent.VDCs {
		if vdcData, ok := vdcInterface.(map[string]interface{}); ok {
			if vdcNamespace, ok := vdcData["namespace"].(string); ok && vdcNamespace == namespace {
				response := &VDCStatusResponse{
					Namespace: namespace,
					Status:    "active",
				}

				// Extract status if available
				if status, ok := vdcData["status"].(string); ok {
					response.Status = status
				}

				// Extract creation time if available
				if createdAtStr, ok := vdcData["created_at"].(string); ok {
					if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
						response.CreatedAt = createdAt
					}
				}

				// Extract resource usage if available
				if resourceUsage, ok := vdcData["resource_usage"].(map[string]interface{}); ok {
					response.ResourceUsage = resourceUsage
				}

				return response, nil
			}
		}
	}

	// If VDC not found in status reports, it might not be deployed yet
	return nil, fmt.Errorf("VDC namespace %s not found on spoke agent %s", namespace, spokeAgent.AgentID)
}
