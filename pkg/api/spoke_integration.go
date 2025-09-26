package api

import (
	"context"
	"fmt"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"k8s.io/klog/v2"
)

// SpokeIntegration integrates the new dynamic FQDN-based spoke client with existing spoke handlers
type SpokeIntegration struct {
	config       *config.Config
	spokeClient  *SpokeClient
	spokeHandler *SpokeHandlers
	storage      storage.Storage
}

// NewSpokeIntegration creates a new spoke integration that bridges old and new spoke systems
func NewSpokeIntegration(cfg *config.Config, storage storage.Storage) (*SpokeIntegration, error) {
	// Create the new spoke client with dynamic FQDN discovery
	spokeClient := NewSpokeClient(&cfg.Spoke, storage)

	// Keep the existing spoke handlers for backward compatibility
	spokeHandler := NewSpokeHandlers(storage)

	return &SpokeIntegration{
		config:       cfg,
		spokeClient:  spokeClient,
		spokeHandler: spokeHandler,
		storage:      storage,
	}, nil
}

// Start initializes both the new and old spoke systems
func (si *SpokeIntegration) Start() error {
	klog.Info("Starting spoke integration with dynamic FQDN discovery")

	// Start the new dynamic spoke client
	if err := si.spokeClient.Start(); err != nil {
		return fmt.Errorf("failed to start spoke client: %w", err)
	}

	klog.Info("Spoke integration started successfully")
	return nil
}

// Stop gracefully shuts down spoke integration
func (si *SpokeIntegration) Stop() error {
	klog.Info("Stopping spoke integration")

	if err := si.spokeClient.Stop(); err != nil {
		klog.Errorf("Error stopping spoke client: %v", err)
	}

	return nil
}

// GetSpokeHandlers returns the legacy spoke handlers for backward compatibility
func (si *SpokeIntegration) GetSpokeHandlers() *SpokeHandlers {
	return si.spokeHandler
}

// GetSpokeClient returns the new dynamic spoke client
func (si *SpokeIntegration) GetSpokeClient() *SpokeClient {
	return si.spokeClient
}

// QueueVDCCreation queues a VDC creation operation using the new spoke client
func (si *SpokeIntegration) QueueVDCCreation(zoneID string, vdcData map[string]interface{}) (string, error) {
	klog.Infof("Queuing VDC creation for zone %s using dynamic spoke client", zoneID)

	// Map zone ID to cluster ID for spoke client discovery
	clusterID := zoneID // In your setup, zone ID == cluster ID

	// Check if spoke is available
	spoke, exists := si.spokeClient.GetSpokeByClusterID(clusterID)
	if !exists {
		klog.Warningf("Spoke agent not found for cluster %s, falling back to legacy queue", clusterID)
		// Fallback to legacy spoke handlers
		agentID := fmt.Sprintf("spoke-agent-%s", zoneID)
		return si.spokeHandler.QueueVDCCreation(agentID, vdcData), nil
	}

	klog.Infof("Found spoke agent for cluster %s at %s", clusterID, spoke.FQDN)

	// Prepare operation for the spoke agent
	operationID := generateOperationID()
	operation := map[string]interface{}{
		"id":        operationID,
		"type":      "create_vdc",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"payload":   vdcData,
	}

	// Store operation metadata in spoke handlers for result processing
	si.spokeHandler.metadataMutex.Lock()
	si.spokeHandler.operationMetadata[operationID] = map[string]interface{}{
		"vdc_name":    vdcData["vdc_name"],
		"agent_id":    fmt.Sprintf("spoke-agent-%s", zoneID),
		"operation":   "create_vdc",
		"created_at":  time.Now(),
	}
	si.spokeHandler.metadataMutex.Unlock()

	// Send operation directly to spoke via HTTPS
	resp, err := si.spokeClient.SendToSpoke(clusterID, "/operations", "POST", operation)
	if err != nil {
		klog.Errorf("Failed to send VDC creation to spoke %s via HTTPS: %v", clusterID, err)
		// Fallback to legacy queue mechanism
		agentID := fmt.Sprintf("spoke-agent-%s", zoneID)
		operationID := si.spokeHandler.QueueVDCCreation(agentID, vdcData)
		klog.Infof("Fallback: Queued VDC creation operation %s for legacy agent %s", operationID, agentID)
		return operationID, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		klog.Infof("Successfully sent VDC creation operation %s to spoke %s via HTTPS (status: %d)",
			operationID, clusterID, resp.StatusCode)
		return operationID, nil
	} else {
		klog.Warningf("Spoke %s responded with status %d, falling back to legacy queue", clusterID, resp.StatusCode)
		// Fallback to legacy queue mechanism
		agentID := fmt.Sprintf("spoke-agent-%s", zoneID)
		operationID := si.spokeHandler.QueueVDCCreation(agentID, vdcData)
		return operationID, nil
	}
}

// QueueVDCDeletion queues a VDC deletion operation using the new spoke client
func (si *SpokeIntegration) QueueVDCDeletion(zoneID string, vdcData map[string]interface{}) (string, error) {
	klog.Infof("Queuing VDC deletion for zone %s using dynamic spoke client", zoneID)

	// Map zone ID to cluster ID for spoke client discovery
	clusterID := zoneID // In your setup, zone ID == cluster ID

	// Check if spoke is available
	spoke, exists := si.spokeClient.GetSpokeByClusterID(clusterID)
	if !exists {
		klog.Warningf("Spoke agent not found for cluster %s, falling back to legacy queue", clusterID)
		// Fallback to legacy spoke handlers
		agentID := fmt.Sprintf("spoke-agent-%s", zoneID)
		return si.spokeHandler.QueueVDCDeletion(agentID, vdcData), nil
	}

	klog.Infof("Found spoke agent for cluster %s at %s", clusterID, spoke.FQDN)

	// Prepare operation for the spoke agent
	operationID := generateOperationID()
	operation := map[string]interface{}{
		"id":        operationID,
		"type":      "delete_vdc",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"payload":   vdcData,
	}

	// Store operation metadata in spoke handlers for result processing
	si.spokeHandler.metadataMutex.Lock()
	si.spokeHandler.operationMetadata[operationID] = map[string]interface{}{
		"vdc_name":    vdcData["vdc_name"],
		"agent_id":    fmt.Sprintf("spoke-agent-%s", zoneID),
		"operation":   "delete_vdc",
		"created_at":  time.Now(),
	}
	si.spokeHandler.metadataMutex.Unlock()

	// Send operation directly to spoke via HTTPS
	resp, err := si.spokeClient.SendToSpoke(clusterID, "/operations", "POST", operation)
	if err != nil {
		klog.Errorf("Failed to send VDC deletion to spoke %s via HTTPS: %v", clusterID, err)
		// Fallback to legacy queue mechanism
		agentID := fmt.Sprintf("spoke-agent-%s", zoneID)
		operationID := si.spokeHandler.QueueVDCDeletion(agentID, vdcData)
		klog.Infof("Fallback: Queued VDC deletion operation %s for legacy agent %s", operationID, agentID)
		return operationID, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		klog.Infof("Successfully sent VDC deletion operation %s to spoke %s via HTTPS (status: %d)",
			operationID, clusterID, resp.StatusCode)
		return operationID, nil
	} else {
		klog.Warningf("Spoke %s responded with status %d, falling back to legacy queue", clusterID, resp.StatusCode)
		// Fallback to legacy queue mechanism
		agentID := fmt.Sprintf("spoke-agent-%s", zoneID)
		operationID := si.spokeHandler.QueueVDCDeletion(agentID, vdcData)
		return operationID, nil
	}
}

// GetSpokeStatus returns status information for all discovered spokes
func (si *SpokeIntegration) GetSpokeStatus() map[string]interface{} {
	spokes := si.spokeClient.GetSpokes()
	healthStatuses := si.spokeClient.GetHealthStatus()
	legacyStatuses := si.spokeHandler.GetAllZoneStatuses()

	result := make(map[string]interface{})

	// Add dynamic spoke information
	for clusterID, spoke := range spokes {
		status := map[string]interface{}{
			"cluster_id": spoke.ClusterID,
			"zone_id":    spoke.ZoneID,
			"fqdn":       spoke.FQDN,
			"enabled":    spoke.Enabled,
			"type":       "dynamic",
		}

		if health, exists := healthStatuses[clusterID]; exists {
			status["health"] = map[string]interface{}{
				"status":    health.Status,
				"timestamp": health.Timestamp,
				"error":     health.Error,
			}
		}

		result[clusterID] = status
	}

	// Add legacy spoke information
	for zoneID, legacyStatus := range legacyStatuses {
		if _, exists := result[zoneID]; !exists {
			result[zoneID] = map[string]interface{}{
				"zone_id":     legacyStatus.ZoneID,
				"cluster_id":  legacyStatus.ClusterID,
				"agent_id":    legacyStatus.AgentID,
				"status":      legacyStatus.Status,
				"last_report": legacyStatus.ReportTime,
				"type":        "legacy",
			}
		}
	}

	return result
}

// CheckConnectivity tests connectivity to all discovered spokes
func (si *SpokeIntegration) CheckConnectivity(ctx context.Context) map[string]interface{} {
	spokes := si.spokeClient.GetSpokes()
	results := make(map[string]interface{})

	for clusterID, spoke := range spokes {
		klog.V(4).Infof("Testing connectivity to spoke %s at %s", clusterID, spoke.FQDN)

		health, err := si.spokeClient.CheckSpokeHealth(clusterID)
		if err != nil {
			results[clusterID] = map[string]interface{}{
				"status": "failed",
				"error":  err.Error(),
				"fqdn":   spoke.FQDN,
			}
		} else {
			results[clusterID] = map[string]interface{}{
				"status":    health.Status,
				"fqdn":      health.FQDN,
				"timestamp": health.Timestamp,
			}
		}
	}

	return results
}

// GetOperationStatus retrieves status of operations sent to spokes
func (si *SpokeIntegration) GetOperationStatus(operationID string) map[string]interface{} {
	// For legacy operations, we would need to implement a separate method
	// as GetOperationResult expects a gin.Context

	// For dynamic operations, we would need to query the spoke agents directly
	// This could be enhanced to poll spoke agents for operation status
	return map[string]interface{}{
		"operation_id": operationID,
		"status":       "unknown",
		"type":         "dynamic",
		"message":      "Operation status tracking not yet implemented",
	}
}