package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// SpokeHandlers handles API requests from spoke agents
type SpokeHandlers struct {
	storage storage.Storage

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

	// Store agent endpoints for push notifications
	agentEndpoints map[string]string
	endpointsMutex sync.RWMutex

	// Store operation metadata for tracking VDC names and other info
	operationMetadata map[string]map[string]interface{}
	metadataMutex     sync.RWMutex
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
	CallbackURL    string                 `json:"callback_url,omitempty"`
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
func NewSpokeHandlers(storage storage.Storage) *SpokeHandlers {
	return &SpokeHandlers{
		storage:           storage,
		operationQueues:   make(map[string][]*SpokeOperation),
		operationResults:  make(map[string]*SpokeOperationResult),
		agentStatuses:     make(map[string]*SpokeStatusReport),
		agentEndpoints:    make(map[string]string),
		operationMetadata: make(map[string]map[string]interface{}),
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
		klog.Errorf("Failed to bind status report: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate required fields
	if report.AgentID == "" || report.ClusterID == "" || report.ZoneID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields: agent_id, cluster_id, zone_id"})
		return
	}

	// Update timestamp
	report.ReportTime = time.Now()
	report.LastHubContact = time.Now()

	// Store the status report
	h.statusMutex.Lock()
	h.agentStatuses[report.AgentID] = &report
	h.statusMutex.Unlock()

	// Store agent callback endpoint if provided
	if report.CallbackURL != "" {
		// Transform localhost URLs to proper FQDN URLs for cross-cluster communication
		callbackURL := h.transformCallbackURL(report.AgentID, report.CallbackURL, report.ClusterID)

		h.endpointsMutex.Lock()
		h.agentEndpoints[report.AgentID] = callbackURL
		h.endpointsMutex.Unlock()
		klog.Infof("Registered callback endpoint for agent %s: %s (original: %s)", report.AgentID, callbackURL, report.CallbackURL)
	}

	klog.Infof("Received status report from spoke agent %s (cluster: %s, zone: %s, status: %s)",
		report.AgentID, report.ClusterID, report.ZoneID, report.Status)

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing agent_id parameter"})
		return
	}

	h.operationsMutex.Lock()
	operations := h.operationQueues[agentID]
	// Clear the queue after reading
	h.operationQueues[agentID] = nil
	h.operationsMutex.Unlock()

	if len(operations) == 0 {
		// No operations available
		c.Status(http.StatusNoContent)
		return
	}

	klog.Infof("Sending %d operations to spoke agent %s", len(operations), agentID)
	c.JSON(http.StatusOK, operations)
}

// HandleOperationResult handles operation results from spoke agents
// POST /api/v1/spoke/operations/:operationId/result
func (h *SpokeHandlers) HandleOperationResult(c *gin.Context) {
	operationID := c.Param("operationId")
	if operationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing operation ID"})
		return
	}

	var result SpokeOperationResult
	if err := c.ShouldBindJSON(&result); err != nil {
		klog.Errorf("Failed to bind operation result: %v", err)
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

	klog.Infof("Received operation result for %s: status=%s", operationID, result.Status)

	// Process operation result based on operation type
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if request.AgentID == "" || request.Type == "" {
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
	h.operationQueues[request.AgentID] = append(h.operationQueues[request.AgentID], operation)
	h.operationsMutex.Unlock()

	klog.Infof("Queued operation %s for spoke agent %s (type: %s)", operation.ID, request.AgentID, request.Type)

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

// QueueVDCCreation queues a VDC creation operation for a spoke agent
func (h *SpokeHandlers) QueueVDCCreation(agentID string, vdcData map[string]interface{}) string {
	operation := &SpokeOperation{
		ID:        generateOperationID(),
		Type:      "create_vdc",
		Timestamp: time.Now(),
		Payload:   vdcData,
	}

	// Try to push operation directly to agent first
	h.endpointsMutex.RLock()
	endpoint, hasEndpoint := h.agentEndpoints[agentID]
	h.endpointsMutex.RUnlock()

	if hasEndpoint {
		// Push operation directly to agent
		go h.pushOperationToAgent(agentID, endpoint, operation)
	} else {
		// Fallback to queue-based approach
		h.operationsMutex.Lock()
		h.operationQueues[agentID] = append(h.operationQueues[agentID], operation)
		h.operationsMutex.Unlock()
		klog.Infof("Agent %s endpoint not available, queued VDC creation operation %s", agentID, operation.ID)
	}

	return operation.ID
}

// QueueVDCDeletion queues a VDC deletion operation for a spoke agent
func (h *SpokeHandlers) QueueVDCDeletion(agentID string, vdcData map[string]interface{}) string {
	operation := &SpokeOperation{
		ID:        generateOperationID(),
		Type:      "delete_vdc",
		Timestamp: time.Now(),
		Payload:   vdcData,
	}

	// Store operation metadata for later retrieval
	h.metadataMutex.Lock()
	h.operationMetadata[operation.ID] = map[string]interface{}{
		"vdc_name":   vdcData["vdc_name"],
		"agent_id":   agentID,
		"operation":  "delete_vdc",
		"created_at": time.Now(),
	}
	h.metadataMutex.Unlock()

	// Try to push operation directly to agent first
	h.endpointsMutex.RLock()
	endpoint, hasEndpoint := h.agentEndpoints[agentID]
	h.endpointsMutex.RUnlock()

	if hasEndpoint {
		// Push operation directly to agent
		go h.pushOperationToAgent(agentID, endpoint, operation)
	} else {
		// Fallback to queue-based approach
		h.operationsMutex.Lock()
		h.operationQueues[agentID] = append(h.operationQueues[agentID], operation)
		h.operationsMutex.Unlock()
		klog.Infof("Agent %s endpoint not available, queued VDC deletion operation %s", agentID, operation.ID)
	}

	return operation.ID
}

// generateOperationID generates a unique operation ID
func generateOperationID() string {
	// Simple timestamp-based ID for demo purposes
	// In production, use UUID or similar
	return "op-" + time.Now().Format("20060102-150405") + "-" + time.Now().Format("000")
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

		// Basic validation
		if agentID == "" || clusterID == "" || zoneID == "" {
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
			klog.Warningf("Invalid authentication from agent %s", agentID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication"})
			c.Abort()
			return
		}

		// Store agent info in context for handlers
		c.Set("agent_id", agentID)
		c.Set("cluster_id", clusterID)
		c.Set("zone_id", zoneID)
		c.Set("agent_version", version)

		c.Next()
	}
}

// pushOperationToAgent pushes an operation directly to a spoke agent
func (h *SpokeHandlers) pushOperationToAgent(agentID, endpoint string, operation *SpokeOperation) {
	// Create JSON payload
	data, err := json.Marshal(operation)
	if err != nil {
		klog.Errorf("Failed to marshal operation %s for agent %s: %v", operation.ID, agentID, err)
		return
	}

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Agent callback URL should have /operations endpoint
	url := fmt.Sprintf("%s/operations", endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		klog.Errorf("Failed to create push request for operation %s to agent %s: %v", operation.ID, agentID, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Operation-ID", operation.ID)

	// Send the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		klog.Errorf("Failed to push operation %s to agent %s at %s: %v", operation.ID, agentID, url, err)
		// Fallback to queue
		h.operationsMutex.Lock()
		h.operationQueues[agentID] = append(h.operationQueues[agentID], operation)
		h.operationsMutex.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		klog.Errorf("Agent %s rejected operation %s (status %d)", agentID, operation.ID, resp.StatusCode)
		// Fallback to queue
		h.operationsMutex.Lock()
		h.operationQueues[agentID] = append(h.operationQueues[agentID], operation)
		h.operationsMutex.Unlock()
		return
	}

	klog.Infof("Successfully pushed VDC creation operation %s to spoke agent %s", operation.ID, agentID)
}

// processOperationResult processes operation results and updates corresponding resource status
func (h *SpokeHandlers) processOperationResult(result *SpokeOperationResult) {
	klog.Infof("Processing operation result %s with status %s", result.OperationID, result.Status)

	// Extract operation type from the operation ID pattern or result data
	// We need to track operation types properly to handle them
	h.operationsMutex.RLock()
	var operationType string
	// Look up operation type from stored operations
	for _, operations := range h.operationQueues {
		for _, op := range operations {
			if op.ID == result.OperationID {
				operationType = op.Type
				break
			}
		}
		if operationType != "" {
			break
		}
	}
	h.operationsMutex.RUnlock()

	// Process based on operation type
	switch operationType {
	case "create_vdc":
		h.processVDCCreationResult(result)
	case "delete_vdc":
		h.processVDCDeletionResult(result)
	default:
		// Fallback: check result data for VDC operations
		if result.Result != nil {
			if _, hasNamespace := result.Result["namespace"]; hasNamespace {
				h.processVDCCreationResult(result)
			} else if status, hasStatus := result.Result["status"]; hasStatus {
				if statusStr, ok := status.(string); ok && (statusStr == "deleted" || statusStr == "deleted_with_warnings") {
					h.processVDCDeletionResult(result)
				}
			}
		}
	}

	// TODO: Add other operation types (VM operations, etc.)
}

// processVDCCreationResult processes VDC creation operation results
func (h *SpokeHandlers) processVDCCreationResult(result *SpokeOperationResult) {
	klog.Infof("Processing VDC creation result %s", result.OperationID)

	// For VDC creation, we typically just log the result
	// The VDC CRD status is managed by the controller
	if result.Status == "completed" {
		klog.Infof("VDC creation completed successfully: %s", result.OperationID)
	} else {
		klog.Errorf("VDC creation failed: %s (status: %s, error: %s)", result.OperationID, result.Status, result.Error)
	}
}

// processVDCDeletionResult processes VDC deletion operation results
func (h *SpokeHandlers) processVDCDeletionResult(result *SpokeOperationResult) {
	klog.Infof("Processing VDC deletion result %s", result.OperationID)

	// Step 5: When deletion is complete, we need to delete the VDC CRD
	if result.Status == "completed" && result.Result != nil {
		status, _ := result.Result["status"].(string)
		if status == "deleted" || status == "deleted_with_warnings" {
			// Extract VDC name from the operation data
			// We need to track the VDC name from the original operation
			h.completeVDCDeletion(result.OperationID, result.Result)
		} else {
			klog.Errorf("VDC deletion completed but with unexpected status: %s", status)
		}
	} else {
		klog.Errorf("VDC deletion failed: %s (status: %s, error: %s)", result.OperationID, result.Status, result.Error)
		// TODO: Update VDC status to DeletionFailed
	}
}

// completeVDCDeletion completes the VDC deletion by removing the CRD
func (h *SpokeHandlers) completeVDCDeletion(operationID string, resultData map[string]interface{}) {
	klog.Infof("Completing VDC deletion for operation %s", operationID)

	// Extract VDC name from stored operation data
	vdcName := h.getVDCNameFromOperation(operationID)
	if vdcName == "" {
		klog.Errorf("Could not find VDC name for operation %s", operationID)
		return
	}

	// Extract warnings if present
	var warnings []string
	if warningsData, ok := resultData["warnings"]; ok {
		if warningsList, ok := warningsData.([]interface{}); ok {
			for _, w := range warningsList {
				if wStr, ok := w.(string); ok {
					warnings = append(warnings, wStr)
				}
			}
		}
	}

	// Call the VDC deletion completion endpoint to finalize deletion
	callbackData := map[string]interface{}{
		"status": resultData["status"],
	}
	if len(warnings) > 0 {
		callbackData["warnings"] = warnings
	}

	// Make internal API call to complete VDC deletion
	h.callVDCDeletionComplete(vdcName, callbackData)

	klog.Infof("VDC deletion operation %s completed successfully", operationID)
	if len(warnings) > 0 {
		klog.Warningf("VDC deletion operation %s completed with warnings: %v", operationID, warnings)
	}

	// Clean up operation metadata
	h.metadataMutex.Lock()
	delete(h.operationMetadata, operationID)
	h.metadataMutex.Unlock()
}

// getVDCNameFromOperation extracts VDC name from stored operation metadata
func (h *SpokeHandlers) getVDCNameFromOperation(operationID string) string {
	h.metadataMutex.RLock()
	defer h.metadataMutex.RUnlock()

	// Look up operation metadata to extract VDC name
	if metadata, exists := h.operationMetadata[operationID]; exists {
		if vdcName, ok := metadata["vdc_name"].(string); ok {
			return vdcName
		}
	}
	return ""
}

// callVDCDeletionComplete makes an internal API call to complete VDC deletion
func (h *SpokeHandlers) callVDCDeletionComplete(vdcName string, callbackData map[string]interface{}) {
	jsonData, err := json.Marshal(callbackData)
	if err != nil {
		klog.Errorf("Failed to marshal VDC deletion callback data for %s: %v", vdcName, err)
		return
	}

	// Make internal API call to the VDC deletion completion endpoint
	url := fmt.Sprintf("http://localhost:8443/api/v1/vdcs/%s/deletion-complete", vdcName)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		klog.Errorf("Failed to create VDC deletion completion request for %s: %v", vdcName, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	// Add internal API authentication if needed
	// req.Header.Set("Authorization", "Bearer " + internalToken)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		klog.Errorf("Failed to call VDC deletion completion for %s: %v", vdcName, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		klog.Infof("Successfully completed VDC deletion for %s", vdcName)
	} else {
		klog.Errorf("VDC deletion completion failed for %s: status %d", vdcName, resp.StatusCode)
	}
}

// transformCallbackURL converts localhost callback URLs to proper FQDN URLs for cross-cluster communication
func (h *SpokeHandlers) transformCallbackURL(agentID, originalURL, clusterID string) string {
	// If the URL contains localhost or 127.0.0.1, transform it to the proper FQDN
	if strings.Contains(originalURL, "localhost") || strings.Contains(originalURL, "127.0.0.1") {
		// Map cluster IDs to their external FQDN
		clusterFQDNMap := map[string]string{
			"test-infra-cluster-bf2fb343": "agent-ovim.apps.test-infra-cluster-bf2fb343.redhat.com",
			"test-infra-cluster-d4e82f9b": "agent-ovim.apps.test-infra-cluster-d4e82f9b.redhat.com",
			"local-cluster":               "agent-ovim.apps.ostest.test.metalkube.org",
		}

		if fqdn, exists := clusterFQDNMap[clusterID]; exists {
			// Extract port from original URL if present
			if strings.Contains(originalURL, ":8081") {
				return fmt.Sprintf("https://%s", fqdn)
			} else if strings.Contains(originalURL, ":8080") {
				return fmt.Sprintf("https://%s", fqdn)
			} else {
				return fmt.Sprintf("https://%s", fqdn)
			}
		} else {
			klog.Warningf("Unknown cluster ID %s for agent %s, using original callback URL %s", clusterID, agentID, originalURL)
		}
	}

	// Return original URL if no transformation needed
	return originalURL
}
