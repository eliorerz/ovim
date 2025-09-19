package acm

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// ZoneSync manages synchronization between ACM clusters and OVIM zones
type ZoneSync struct {
	storage   storage.Storage
	discovery *ClusterDiscovery
	config    SyncConfig
	running   bool
	stopCh    chan struct{}
}

// NewZoneSync creates a new zone synchronization service
func NewZoneSync(storage storage.Storage, discovery *ClusterDiscovery, config SyncConfig) *ZoneSync {
	return &ZoneSync{
		storage:   storage,
		discovery: discovery,
		config:    config,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the zone synchronization process
func (zs *ZoneSync) Start(ctx context.Context) error {
	if zs.running {
		return fmt.Errorf("zone sync is already running")
	}

	if !zs.config.Enabled {
		klog.Info("ACM zone sync is disabled")
		return nil
	}

	zs.running = true
	klog.Infof("Starting ACM zone sync with interval: %v", zs.config.Interval)

	// Perform initial sync
	if err := zs.performSync(ctx); err != nil {
		klog.Errorf("Initial zone sync failed: %v", err)
		// Don't fail startup on sync error, continue with periodic sync
	}

	// Start periodic sync in background
	go zs.syncLoop(ctx)

	return nil
}

// Stop stops the zone synchronization process
func (zs *ZoneSync) Stop() {
	if !zs.running {
		return
	}

	klog.Info("Stopping ACM zone sync")
	zs.running = false
	close(zs.stopCh)
}

// syncLoop runs the periodic synchronization
func (zs *ZoneSync) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(zs.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := zs.performSync(ctx); err != nil {
				klog.Errorf("Zone sync failed: %v", err)
			}
		case <-zs.stopCh:
			klog.Info("Zone sync loop stopped")
			return
		case <-ctx.Done():
			klog.Info("Zone sync loop cancelled")
			return
		}
	}
}

// PerformSync manually triggers a synchronization
func (zs *ZoneSync) PerformSync(ctx context.Context) error {
	return zs.performSync(ctx)
}

// performSync executes the actual synchronization logic
func (zs *ZoneSync) performSync(ctx context.Context) error {
	startTime := time.Now()
	result := &SyncResult{
		Timestamp: startTime,
		Success:   false,
	}

	defer func() {
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		zs.logSyncResult(result)
	}()

	klog.V(2).Info("Starting ACM zone synchronization")

	// Step 1: Discover clusters from ACM
	clusters, err := zs.discovery.DiscoverClusters(ctx)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("cluster discovery failed: %v", err)
		return err
	}

	result.ClustersFound = len(clusters)
	klog.V(3).Infof("Discovered %d clusters from ACM", len(clusters))

	// Step 2: Convert clusters to zones
	discoveredZones, err := zs.discovery.ConvertToZones(clusters)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("zone conversion failed: %v", err)
		return err
	}

	// Step 3: Get existing zones from storage
	existingZones, err := zs.storage.ListZones()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to list existing zones: %v", err)
		return err
	}

	// Step 4: Synchronize zones
	if err := zs.synchronizeZones(discoveredZones, existingZones, result); err != nil {
		result.ErrorMessage = fmt.Sprintf("zone synchronization failed: %v", err)
		return err
	}

	result.Success = true
	klog.Infof("Zone sync completed successfully: created=%d, updated=%d, deleted=%d",
		result.ZonesCreated, result.ZonesUpdated, result.ZonesDeleted)

	return nil
}

// synchronizeZones performs the actual zone synchronization
func (zs *ZoneSync) synchronizeZones(discoveredZones []*models.Zone, existingZones []*models.Zone, result *SyncResult) error {
	// Create maps for efficient lookup
	discoveredMap := make(map[string]*models.Zone)
	existingMap := make(map[string]*models.Zone)

	for _, zone := range discoveredZones {
		discoveredMap[zone.ClusterName] = zone
	}

	for _, zone := range existingZones {
		// Only consider ACM-managed zones for sync
		if zs.isACMManagedZone(zone) {
			existingMap[zone.ClusterName] = zone
		}
	}

	// Process discovered zones (create or update)
	for clusterName, discoveredZone := range discoveredMap {
		if existingZone, exists := existingMap[clusterName]; exists {
			// Update existing zone
			if zs.shouldUpdateZone(existingZone, discoveredZone) {
				if err := zs.updateZone(existingZone, discoveredZone); err != nil {
					klog.Errorf("Failed to update zone %s: %v", existingZone.ID, err)
					continue
				}
				result.ZonesUpdated++
				klog.V(3).Infof("Updated zone: %s (cluster: %s)", existingZone.Name, clusterName)
			}
		} else {
			// Create new zone
			if zs.config.AutoCreateZones {
				if err := zs.storage.CreateZone(discoveredZone); err != nil {
					klog.Errorf("Failed to create zone for cluster %s: %v", clusterName, err)
					continue
				}
				result.ZonesCreated++
				klog.V(3).Infof("Created zone: %s (cluster: %s)", discoveredZone.Name, clusterName)
			} else {
				klog.V(3).Infof("Skipping zone creation for cluster %s (auto-create disabled)", clusterName)
			}
		}
	}

	// Process zones that are no longer discovered (potential deletion)
	for clusterName, existingZone := range existingMap {
		if _, stillExists := discoveredMap[clusterName]; !stillExists {
			// Cluster no longer exists in ACM
			if zs.shouldDeleteZone(existingZone) {
				if err := zs.storage.DeleteZone(existingZone.ID); err != nil {
					klog.Errorf("Failed to delete zone %s: %v", existingZone.ID, err)
					continue
				}
				result.ZonesDeleted++
				klog.V(3).Infof("Deleted zone: %s (cluster: %s)", existingZone.Name, clusterName)
			} else {
				// Mark as unavailable instead of deleting
				existingZone.Status = models.ZoneStatusUnavailable
				existingZone.UpdatedAt = time.Now()
				if err := zs.storage.UpdateZone(existingZone); err != nil {
					klog.Errorf("Failed to mark zone %s as unavailable: %v", existingZone.ID, err)
					continue
				}
				result.ZonesUpdated++
				klog.V(3).Infof("Marked zone as unavailable: %s (cluster: %s)", existingZone.Name, clusterName)
			}
		}
	}

	return nil
}

// isACMManagedZone checks if a zone is managed by ACM sync
func (zs *ZoneSync) isACMManagedZone(zone *models.Zone) bool {
	if zone.Labels == nil {
		return false
	}

	// Check for ACM sync labels
	if managedBy, exists := zone.Labels[LabelManagedBy]; exists {
		return managedBy == "ovim-acm-sync"
	}

	if zoneType, exists := zone.Labels[LabelZoneType]; exists {
		return zoneType == "acm-managed"
	}

	return false
}

// shouldUpdateZone determines if an existing zone should be updated
func (zs *ZoneSync) shouldUpdateZone(existing, discovered *models.Zone) bool {
	// Always update if status changed
	if existing.Status != discovered.Status {
		return true
	}

	// Update if API URL changed
	if existing.APIUrl != discovered.APIUrl {
		return true
	}

	// Update if capacity changed significantly (>10% difference)
	if zs.hasSignificantCapacityChange(existing, discovered) {
		return true
	}

	// Update if it's been more than 24 hours since last sync
	if time.Since(existing.LastSync) > 24*time.Hour {
		return true
	}

	return false
}

// hasSignificantCapacityChange checks if capacity has changed significantly
func (zs *ZoneSync) hasSignificantCapacityChange(existing, discovered *models.Zone) bool {
	threshold := 0.1 // 10% threshold

	// Check CPU capacity change
	if existing.CPUCapacity > 0 {
		cpuChange := float64(abs(existing.CPUCapacity-discovered.CPUCapacity)) / float64(existing.CPUCapacity)
		if cpuChange > threshold {
			return true
		}
	}

	// Check memory capacity change
	if existing.MemoryCapacity > 0 {
		memoryChange := float64(abs(existing.MemoryCapacity-discovered.MemoryCapacity)) / float64(existing.MemoryCapacity)
		if memoryChange > threshold {
			return true
		}
	}

	// Check storage capacity change
	if existing.StorageCapacity > 0 {
		storageChange := float64(abs(existing.StorageCapacity-discovered.StorageCapacity)) / float64(existing.StorageCapacity)
		if storageChange > threshold {
			return true
		}
	}

	return false
}

// shouldDeleteZone determines if a zone should be deleted
func (zs *ZoneSync) shouldDeleteZone(zone *models.Zone) bool {
	// Check if zone has any VDCs deployed
	vdcs, err := zs.storage.ListVDCs("")
	if err != nil {
		klog.Errorf("Failed to list VDCs when checking zone deletion: %v", err)
		return false // Don't delete if we can't check
	}

	// Count VDCs in this zone
	vdcCount := 0
	for _, vdc := range vdcs {
		if vdc.ZoneID != nil && *vdc.ZoneID == zone.ID {
			vdcCount++
		}
	}

	// Don't delete zones with active VDCs
	if vdcCount > 0 {
		klog.V(3).Infof("Preserving zone %s: has %d VDCs deployed", zone.Name, vdcCount)
		return false
	}

	// Check if zone has been unavailable for a grace period
	graceHours := 72 // 3 days grace period
	if zone.Status == models.ZoneStatusUnavailable &&
		time.Since(zone.UpdatedAt) > time.Duration(graceHours)*time.Hour {
		return true
	}

	return false
}

// updateZone updates an existing zone with discovered information
func (zs *ZoneSync) updateZone(existing, discovered *models.Zone) error {
	// Preserve the existing ID and creation time
	discovered.ID = existing.ID
	discovered.CreatedAt = existing.CreatedAt

	// Update timestamps
	discovered.UpdatedAt = time.Now()
	discovered.LastSync = time.Now()

	// Merge labels and annotations (preserve existing, add new)
	discovered.Labels = zs.mergeStringMaps(existing.Labels, discovered.Labels)
	discovered.Annotations = zs.mergeStringMaps(existing.Annotations, discovered.Annotations)

	// Update quota based on new capacity if configured
	if zs.config.DefaultQuotaPercentage > 0 {
		discovered.CPUQuota = zs.discovery.calculateQuota(discovered.CPUCapacity)
		discovered.MemoryQuota = zs.discovery.calculateQuota(discovered.MemoryCapacity)
		discovered.StorageQuota = zs.discovery.calculateQuota(discovered.StorageCapacity)
	} else {
		// Preserve existing quotas
		discovered.CPUQuota = existing.CPUQuota
		discovered.MemoryQuota = existing.MemoryQuota
		discovered.StorageQuota = existing.StorageQuota
	}

	return zs.storage.UpdateZone(discovered)
}

// mergeStringMaps merges two StringMap instances, with new values taking precedence
func (zs *ZoneSync) mergeStringMaps(existing, new models.StringMap) models.StringMap {
	result := make(models.StringMap)

	// Copy existing values
	if existing != nil {
		for k, v := range existing {
			result[k] = v
		}
	}

	// Add/override with new values
	if new != nil {
		for k, v := range new {
			result[k] = v
		}
	}

	return result
}

// logSyncResult logs the result of a sync operation
func (zs *ZoneSync) logSyncResult(result *SyncResult) {
	if result.Success {
		klog.Infof("Zone sync completed: clusters=%d, created=%d, updated=%d, deleted=%d, duration=%dms",
			result.ClustersFound, result.ZonesCreated, result.ZonesUpdated, result.ZonesDeleted, result.ProcessingTimeMs)
	} else {
		klog.Errorf("Zone sync failed after %dms: %s", result.ProcessingTimeMs, result.ErrorMessage)
	}
}

// GetSyncStatus returns the current sync configuration and status
func (zs *ZoneSync) GetSyncStatus() map[string]interface{} {
	return map[string]interface{}{
		"enabled":     zs.config.Enabled,
		"running":     zs.running,
		"interval":    zs.config.Interval.String(),
		"auto_create": zs.config.AutoCreateZones,
		"namespace":   zs.config.Namespace,
	}
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
