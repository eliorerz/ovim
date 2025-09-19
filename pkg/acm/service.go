package acm

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// Service provides a high-level interface for ACM cluster management
type Service struct {
	client    *Client
	discovery *ClusterDiscovery
	zoneSync  *ZoneSync
	config    SyncConfig
	started   bool
}

// ServiceOptions contains options for creating an ACM service
type ServiceOptions struct {
	Storage       storage.Storage
	Config        SyncConfig
	ClientOptions ClientOptions
}

// NewService creates a new ACM service
func NewService(opts ServiceOptions) (*Service, error) {
	// Validate config
	if err := validateSyncConfig(opts.Config); err != nil {
		return nil, fmt.Errorf("invalid sync config: %w", err)
	}

	// Create ACM client
	client, err := NewClient(opts.ClientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACM client: %w", err)
	}

	// Create cluster discovery
	discovery := NewClusterDiscovery(client, opts.Config)

	// Create zone sync
	zoneSync := NewZoneSync(opts.Storage, discovery, opts.Config)

	service := &Service{
		client:    client,
		discovery: discovery,
		zoneSync:  zoneSync,
		config:    opts.Config,
	}

	klog.Info("ACM service created successfully")
	return service, nil
}

// Start starts the ACM service and begins zone synchronization
func (s *Service) Start(ctx context.Context) error {
	if s.started {
		return fmt.Errorf("ACM service is already started")
	}

	if !s.config.Enabled {
		klog.Info("ACM service is disabled")
		return nil
	}

	klog.Info("Starting ACM service")

	// Start zone synchronization
	if err := s.zoneSync.Start(ctx); err != nil {
		return fmt.Errorf("failed to start zone sync: %w", err)
	}

	s.started = true
	klog.Info("ACM service started successfully")
	return nil
}

// Stop stops the ACM service
func (s *Service) Stop() error {
	if !s.started {
		return nil
	}

	klog.Info("Stopping ACM service")

	// Stop zone sync
	s.zoneSync.Stop()

	// Close client connections
	if err := s.client.Close(); err != nil {
		klog.Errorf("Error closing ACM client: %v", err)
	}

	s.started = false
	klog.Info("ACM service stopped")
	return nil
}

// DiscoverClusters manually triggers cluster discovery
func (s *Service) DiscoverClusters(ctx context.Context) ([]*ClusterInfo, error) {
	return s.discovery.DiscoverClusters(ctx)
}

// SyncZones manually triggers zone synchronization
func (s *Service) SyncZones(ctx context.Context) error {
	return s.zoneSync.PerformSync(ctx)
}

// GetSyncStatus returns the current sync status
func (s *Service) GetSyncStatus() map[string]interface{} {
	status := s.zoneSync.GetSyncStatus()
	status["client_namespace"] = s.client.GetNamespace()
	status["started"] = s.started
	return status
}

// GetClusterInfo retrieves information about a specific cluster
func (s *Service) GetClusterInfo(ctx context.Context, clusterName string) (*ClusterInfo, error) {
	cluster, err := s.client.GetManagedCluster(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s: %w", clusterName, err)
	}

	return s.client.GetClusterInfo(cluster), nil
}

// TestConnection tests the connection to the ACM hub
func (s *Service) TestConnection(ctx context.Context) error {
	// Try to list clusters to test connectivity
	_, err := s.client.ListManagedClusters(ctx)
	if err != nil {
		return fmt.Errorf("ACM connection test failed: %w", err)
	}

	klog.Info("ACM connection test successful")
	return nil
}

// GetConfig returns the current sync configuration
func (s *Service) GetConfig() SyncConfig {
	return s.config
}

// UpdateConfig updates the sync configuration (requires restart to take effect)
func (s *Service) UpdateConfig(newConfig SyncConfig) error {
	if err := validateSyncConfig(newConfig); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	s.config = newConfig
	klog.Info("ACM service configuration updated (restart required for changes to take effect)")
	return nil
}

// GetMetrics returns basic metrics about the ACM service
func (s *Service) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"started":           s.started,
		"config_enabled":    s.config.Enabled,
		"auto_create_zones": s.config.AutoCreateZones,
		"sync_interval":     s.config.Interval.String(),
		"excluded_clusters": len(s.config.ExcludedClusters),
		"required_labels":   len(s.config.RequiredLabels),
		"quota_percentage":  s.config.DefaultQuotaPercentage,
	}
}

// validateSyncConfig validates the sync configuration
func validateSyncConfig(config SyncConfig) error {
	if config.Enabled {
		if config.Interval <= 0 {
			return fmt.Errorf("sync interval must be positive")
		}

		if config.Interval < time.Minute {
			return fmt.Errorf("sync interval must be at least 1 minute")
		}

		if config.DefaultQuotaPercentage < 0 || config.DefaultQuotaPercentage > 100 {
			return fmt.Errorf("quota percentage must be between 0 and 100")
		}
	}

	return nil
}

// DefaultSyncConfig returns a default sync configuration
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		Enabled:                false, // Disabled by default
		Interval:               10 * time.Minute,
		Namespace:              "open-cluster-management",
		AutoCreateZones:        true,
		ZonePrefix:             "",
		DefaultQuotaPercentage: 80,
		ExcludedClusters:       []string{},
		RequiredLabels:         map[string]string{},
	}
}

// EnabledSyncConfig returns an enabled sync configuration with defaults
func EnabledSyncConfig(kubeconfig string) SyncConfig {
	config := DefaultSyncConfig()
	config.Enabled = true
	config.HubKubeconfig = kubeconfig
	return config
}

// NewServiceWithDefaults creates a new ACM service with default configuration
func NewServiceWithDefaults(storage storage.Storage, kubeconfig string) (*Service, error) {
	config := EnabledSyncConfig(kubeconfig)
	clientOpts := ClientOptions{
		Kubeconfig: kubeconfig,
		Namespace:  config.Namespace,
		Timeout:    30 * time.Second,
	}

	return NewService(ServiceOptions{
		Storage:       storage,
		Config:        config,
		ClientOptions: clientOpts,
	})
}

// NewDisabledService creates a new ACM service with sync disabled (for testing)
func NewDisabledService(storage storage.Storage) (*Service, error) {
	config := DefaultSyncConfig()
	config.Enabled = false

	// Create a minimal client that won't actually connect
	clientOpts := ClientOptions{
		Namespace: config.Namespace,
		Timeout:   5 * time.Second,
	}

	// This will fail to connect, but that's OK since sync is disabled
	client, _ := NewClient(clientOpts)

	discovery := NewClusterDiscovery(client, config)
	zoneSync := NewZoneSync(storage, discovery, config)

	return &Service{
		client:    client,
		discovery: discovery,
		zoneSync:  zoneSync,
		config:    config,
	}, nil
}
