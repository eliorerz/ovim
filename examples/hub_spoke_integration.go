package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/api"
	"github.com/eliorerz/ovim-updated/pkg/config"
)

// HubSpokeManager demonstrates how to integrate spoke communication into the hub service
type HubSpokeManager struct {
	config      *config.Config
	spokeClient *api.SpokeClient
}

// NewHubSpokeManager creates a new hub-spoke manager
func NewHubSpokeManager() (*HubSpokeManager, error) {
	// Load configuration from environment variables
	cfg, err := config.Load("")
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create spoke client
	spokeClient := api.NewSpokeClient(&cfg.Spoke, nil)

	return &HubSpokeManager{
		config:      cfg,
		spokeClient: spokeClient,
	}, nil
}

// Start initializes the hub-spoke communication
func (h *HubSpokeManager) Start() error {
	log.Println("Starting hub-spoke manager...")

	// Start the spoke client (discovery and health monitoring)
	if err := h.spokeClient.Start(); err != nil {
		return fmt.Errorf("failed to start spoke client: %w", err)
	}

	log.Println("Hub-spoke manager started successfully")
	return nil
}

// Stop gracefully shuts down the hub-spoke manager
func (h *HubSpokeManager) Stop() error {
	log.Println("Stopping hub-spoke manager...")
	return h.spokeClient.Stop()
}

// GetSpokeStatus returns the status of all discovered spoke agents
func (h *HubSpokeManager) GetSpokeStatus() map[string]*api.SpokeHealthResponse {
	return h.spokeClient.GetHealthStatus()
}

// SendOperationToSpoke sends an operation to a specific spoke agent
func (h *HubSpokeManager) SendOperationToSpoke(clusterID string, operation map[string]interface{}) error {
	resp, err := h.spokeClient.SendToSpoke(clusterID, "/operations", "POST", operation)
	if err != nil {
		return fmt.Errorf("failed to send operation to spoke %s: %w", clusterID, err)
	}
	defer resp.Body.Close()

	log.Printf("Operation sent successfully to spoke %s (status: %d)", clusterID, resp.StatusCode)
	return nil
}

// MonitorSpokeHealth demonstrates periodic health monitoring
func (h *HubSpokeManager) MonitorSpokeHealth(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.logSpokeHealthStatus()
		case <-ctx.Done():
			return
		}
	}
}

// logSpokeHealthStatus logs the current health status of all spokes
func (h *HubSpokeManager) logSpokeHealthStatus() {
	spokes := h.spokeClient.GetSpokes()
	healthStatuses := h.spokeClient.GetHealthStatus()

	log.Printf("=== Spoke Health Status (Total: %d) ===", len(spokes))

	for clusterID, spoke := range spokes {
		if health, exists := healthStatuses[clusterID]; exists {
			log.Printf("  %s (%s): %s [Last check: %v]",
				clusterID, spoke.FQDN, health.Status, health.Timestamp.Format("15:04:05"))
		} else {
			log.Printf("  %s (%s): No health data", clusterID, spoke.FQDN)
		}
	}
}

// Example usage function
func ExampleUsage() {
	// This demonstrates how you would integrate spoke communication
	// into your existing OVIM hub service

	manager, err := NewHubSpokeManager()
	if err != nil {
		log.Fatalf("Failed to create hub-spoke manager: %v", err)
	}

	// Start the manager
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start hub-spoke manager: %v", err)
	}
	defer manager.Stop()

	// Wait for initial discovery
	time.Sleep(2 * time.Second)

	// Example: Send an operation to a spoke
	operation := map[string]interface{}{
		"type":    "health_check",
		"payload": map[string]string{"action": "ping"},
	}

	if err := manager.SendOperationToSpoke("test-infra-cluster-bf2fb343", operation); err != nil {
		log.Printf("Failed to send operation: %v", err)
	}

	// Monitor health for a short period
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	manager.MonitorSpokeHealth(ctx)
}

func main() {
	// Example configuration for your environment
	fmt.Println("OVIM Hub-Spoke Integration Example")
	fmt.Println("==================================")
	fmt.Println()
	fmt.Println("To run this example, set these environment variables:")
	fmt.Println()
	fmt.Println("export OVIM_SPOKE_PROTOCOL=https")
	fmt.Println("export OVIM_SPOKE_TLS_SKIP_VERIFY=true")
	fmt.Println("export OVIM_SPOKE_DISCOVERY_SOURCE=environment")
	fmt.Println("export OVIM_SPOKE_LIST=\"test-infra-cluster-bf2fb343:test-infra-cluster-bf2fb343:agent-ovim.apps.test-infra-cluster-bf2fb343.redhat.com,test-infra-cluster-d4e82f9b:test-infra-cluster-d4e82f9b:agent-ovim.apps.test-infra-cluster-d4e82f9b.redhat.com\"")
	fmt.Println()
	fmt.Println("Then run: go run examples/hub_spoke_integration.go")
	fmt.Println()

	// Uncomment to run the actual example
	// ExampleUsage()
}
