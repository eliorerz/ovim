package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// TestZoneOperations tests zone CRUD operations
func TestZoneOperations(t *testing.T) {
	storage, err := NewMemoryStorageForTest()
	require.NoError(t, err)

	// Test data
	zone := &models.Zone{
		ID:              "zone-1",
		Name:            "test-zone",
		ClusterName:     "test-cluster",
		APIUrl:          "https://api.test-cluster.example.com:6443",
		Status:          "available",
		Region:          "us-west-2",
		CloudProvider:   "aws",
		NodeCount:       5,
		CPUCapacity:     100,
		MemoryCapacity:  512,
		StorageCapacity: 2000,
		CPUQuota:        80,
		MemoryQuota:     400,
		StorageQuota:    1600,
	}

	t.Run("CreateZone", func(t *testing.T) {
		err := storage.CreateZone(zone)
		assert.NoError(t, err)

		// Test duplicate creation
		err = storage.CreateZone(zone)
		assert.Equal(t, ErrAlreadyExists, err)

		// Test nil zone
		err = storage.CreateZone(nil)
		assert.Equal(t, ErrInvalidInput, err)
	})

	t.Run("GetZone", func(t *testing.T) {
		retrieved, err := storage.GetZone("zone-1")
		assert.NoError(t, err)
		assert.Equal(t, zone.ID, retrieved.ID)
		assert.Equal(t, zone.Name, retrieved.Name)
		assert.Equal(t, zone.ClusterName, retrieved.ClusterName)
		assert.Equal(t, zone.Status, retrieved.Status)

		// Test non-existent zone
		_, err = storage.GetZone("non-existent")
		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("ListZones", func(t *testing.T) {
		zones, err := storage.ListZones()
		assert.NoError(t, err)
		assert.Len(t, zones, 1)
		assert.Equal(t, "zone-1", zones[0].ID)
	})

	t.Run("UpdateZone", func(t *testing.T) {
		zone.Status = "maintenance"
		zone.CPUQuota = 90
		err := storage.UpdateZone(zone)
		assert.NoError(t, err)

		updated, err := storage.GetZone("zone-1")
		assert.NoError(t, err)
		assert.Equal(t, "maintenance", updated.Status)
		assert.Equal(t, 90, updated.CPUQuota)

		// Test updating non-existent zone
		nonExistent := &models.Zone{ID: "non-existent"}
		err = storage.UpdateZone(nonExistent)
		assert.Equal(t, ErrNotFound, err)

		// Test nil zone
		err = storage.UpdateZone(nil)
		assert.Equal(t, ErrInvalidInput, err)
	})

	t.Run("DeleteZone", func(t *testing.T) {
		err := storage.DeleteZone("zone-1")
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.GetZone("zone-1")
		assert.Equal(t, ErrNotFound, err)

		// Test deleting non-existent zone
		err = storage.DeleteZone("zone-1")
		assert.Equal(t, ErrNotFound, err)
	})
}

// TestOrganizationZoneQuotaOperations tests organization zone quota CRUD operations
func TestOrganizationZoneQuotaOperations(t *testing.T) {
	storage, err := NewMemoryStorageForTest()
	require.NoError(t, err)

	// Create a test zone first
	zone := &models.Zone{
		ID:              "zone-1",
		Name:            "test-zone",
		ClusterName:     "test-cluster",
		APIUrl:          "https://api.test-cluster.example.com:6443",
		Status:          "available",
		CPUCapacity:     100,
		MemoryCapacity:  512,
		StorageCapacity: 2000,
		CPUQuota:        80,
		MemoryQuota:     400,
		StorageQuota:    1600,
	}
	err = storage.CreateZone(zone)
	require.NoError(t, err)

	// Test data
	quota := &models.OrganizationZoneQuota{
		ID:             "quota-1",
		OrganizationID: "org-1",
		ZoneID:         "zone-1",
		CPUQuota:       20,
		MemoryQuota:    100,
		StorageQuota:   400,
		IsAllowed:      true,
	}

	t.Run("CreateOrganizationZoneQuota", func(t *testing.T) {
		err := storage.CreateOrganizationZoneQuota(quota)
		assert.NoError(t, err)

		// Test duplicate creation
		err = storage.CreateOrganizationZoneQuota(quota)
		assert.Equal(t, ErrAlreadyExists, err)

		// Test nil quota
		err = storage.CreateOrganizationZoneQuota(nil)
		assert.Equal(t, ErrInvalidInput, err)
	})

	t.Run("GetOrganizationZoneQuota", func(t *testing.T) {
		retrieved, err := storage.GetOrganizationZoneQuota("org-1", "zone-1")
		assert.NoError(t, err)
		assert.Equal(t, quota.OrganizationID, retrieved.OrganizationID)
		assert.Equal(t, quota.ZoneID, retrieved.ZoneID)
		assert.Equal(t, quota.CPUQuota, retrieved.CPUQuota)
		assert.Equal(t, quota.IsAllowed, retrieved.IsAllowed)
		assert.NotNil(t, retrieved.Zone)
		assert.Equal(t, "test-zone", retrieved.Zone.Name)

		// Test non-existent quota
		_, err = storage.GetOrganizationZoneQuota("org-2", "zone-1")
		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("ListOrganizationZoneQuotas", func(t *testing.T) {
		// List all quotas
		quotas, err := storage.ListOrganizationZoneQuotas("")
		assert.NoError(t, err)
		assert.Len(t, quotas, 1)
		assert.Equal(t, "org-1", quotas[0].OrganizationID)
		assert.NotNil(t, quotas[0].Zone)

		// List quotas for specific organization
		quotas, err = storage.ListOrganizationZoneQuotas("org-1")
		assert.NoError(t, err)
		assert.Len(t, quotas, 1)

		// List quotas for non-existent organization
		quotas, err = storage.ListOrganizationZoneQuotas("org-2")
		assert.NoError(t, err)
		assert.Len(t, quotas, 0)
	})

	t.Run("UpdateOrganizationZoneQuota", func(t *testing.T) {
		quota.CPUQuota = 30
		quota.IsAllowed = false
		err := storage.UpdateOrganizationZoneQuota(quota)
		assert.NoError(t, err)

		updated, err := storage.GetOrganizationZoneQuota("org-1", "zone-1")
		assert.NoError(t, err)
		assert.Equal(t, 30, updated.CPUQuota)
		assert.Equal(t, false, updated.IsAllowed)

		// Test updating non-existent quota
		nonExistent := &models.OrganizationZoneQuota{
			OrganizationID: "org-2",
			ZoneID:         "zone-1",
		}
		err = storage.UpdateOrganizationZoneQuota(nonExistent)
		assert.Equal(t, ErrNotFound, err)

		// Test nil quota
		err = storage.UpdateOrganizationZoneQuota(nil)
		assert.Equal(t, ErrInvalidInput, err)
	})

	t.Run("DeleteOrganizationZoneQuota", func(t *testing.T) {
		err := storage.DeleteOrganizationZoneQuota("org-1", "zone-1")
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.GetOrganizationZoneQuota("org-1", "zone-1")
		assert.Equal(t, ErrNotFound, err)

		// Test deleting non-existent quota
		err = storage.DeleteOrganizationZoneQuota("org-1", "zone-1")
		assert.Equal(t, ErrNotFound, err)
	})
}

// TestZoneUtilization tests zone utilization calculations
func TestZoneUtilization(t *testing.T) {
	storage, err := NewMemoryStorageForTest()
	require.NoError(t, err)

	// Create test zone
	zone := &models.Zone{
		ID:              "zone-1",
		Name:            "test-zone",
		ClusterName:     "test-cluster",
		APIUrl:          "https://api.test-cluster.example.com:6443",
		Status:          "available",
		CPUCapacity:     100,
		MemoryCapacity:  512,
		StorageCapacity: 2000,
		CPUQuota:        80,
		MemoryQuota:     400,
		StorageQuota:    1600,
	}
	err = storage.CreateZone(zone)
	require.NoError(t, err)

	// Create test organization
	org := &models.Organization{
		ID:        "org-1",
		Name:      "Test Organization",
		IsEnabled: true,
	}
	err = storage.CreateOrganization(org)
	require.NoError(t, err)

	// Create test VDCs in the zone
	vdc1 := &models.VirtualDataCenter{
		ID:           "vdc-1",
		Name:         "VDC 1",
		OrgID:        "org-1",
		ZoneID:       &zone.ID,
		Phase:        "Active",
		CPUQuota:     10,
		MemoryQuota:  32,
		StorageQuota: 100,
	}
	err = storage.CreateVDC(vdc1)
	require.NoError(t, err)

	vdc2 := &models.VirtualDataCenter{
		ID:           "vdc-2",
		Name:         "VDC 2",
		OrgID:        "org-1",
		ZoneID:       &zone.ID,
		Phase:        "Pending",
		CPUQuota:     5,
		MemoryQuota:  16,
		StorageQuota: 50,
	}
	err = storage.CreateVDC(vdc2)
	require.NoError(t, err)

	t.Run("GetZoneUtilization", func(t *testing.T) {
		utilization, err := storage.GetZoneUtilization()
		assert.NoError(t, err)
		assert.Len(t, utilization, 1)

		util := utilization[0]
		assert.Equal(t, "zone-1", util.ID)
		assert.Equal(t, "test-zone", util.Name)
		assert.Equal(t, "available", util.Status)
		assert.Equal(t, 100, util.CPUCapacity)
		assert.Equal(t, 512, util.MemoryCapacity)
		assert.Equal(t, 2000, util.StorageCapacity)
		assert.Equal(t, 80, util.CPUQuota)
		assert.Equal(t, 400, util.MemoryQuota)
		assert.Equal(t, 1600, util.StorageQuota)
		assert.Equal(t, 15, util.CPUUsed)      // 10 + 5 from VDCs
		assert.Equal(t, 48, util.MemoryUsed)   // 32 + 16 from VDCs
		assert.Equal(t, 150, util.StorageUsed) // 100 + 50 from VDCs
		assert.Equal(t, 2, util.VDCCount)
		assert.Equal(t, 1, util.ActiveVDCCount) // Only vdc1 is Active
	})
}

// TestOrganizationZoneAccess tests organization zone access queries
func TestOrganizationZoneAccess(t *testing.T) {
	storage, err := NewMemoryStorageForTest()
	require.NoError(t, err)

	// Create test zone
	zone := &models.Zone{
		ID:          "zone-1",
		Name:        "test-zone",
		ClusterName: "test-cluster",
		APIUrl:      "https://api.test-cluster.example.com:6443",
		Status:      "available",
	}
	err = storage.CreateZone(zone)
	require.NoError(t, err)

	// Create test organization
	org := &models.Organization{
		ID:        "org-1",
		Name:      "Test Organization",
		IsEnabled: true,
	}
	err = storage.CreateOrganization(org)
	require.NoError(t, err)

	// Create organization zone quota
	quota := &models.OrganizationZoneQuota{
		ID:             "quota-1",
		OrganizationID: "org-1",
		ZoneID:         "zone-1",
		CPUQuota:       20,
		MemoryQuota:    100,
		StorageQuota:   400,
		IsAllowed:      true,
	}
	err = storage.CreateOrganizationZoneQuota(quota)
	require.NoError(t, err)

	// Create test VDC in the zone
	vdc := &models.VirtualDataCenter{
		ID:           "vdc-1",
		Name:         "VDC 1",
		OrgID:        "org-1",
		ZoneID:       &zone.ID,
		Phase:        "Active",
		CPUQuota:     10,
		MemoryQuota:  32,
		StorageQuota: 100,
	}
	err = storage.CreateVDC(vdc)
	require.NoError(t, err)

	t.Run("GetOrganizationZoneAccess", func(t *testing.T) {
		// Get access for specific organization
		access, err := storage.GetOrganizationZoneAccess("org-1")
		assert.NoError(t, err)
		assert.Len(t, access, 1)

		accessItem := access[0]
		assert.Equal(t, "org-1", accessItem.OrganizationID)
		assert.Equal(t, "zone-1", accessItem.ZoneID)
		assert.Equal(t, "test-zone", accessItem.ZoneName)
		assert.Equal(t, "available", accessItem.ZoneStatus)
		assert.Equal(t, 20, accessItem.CPUQuota)
		assert.Equal(t, 100, accessItem.MemoryQuota)
		assert.Equal(t, 400, accessItem.StorageQuota)
		assert.Equal(t, true, accessItem.IsAllowed)
		assert.Equal(t, 10, accessItem.CPUUsed)
		assert.Equal(t, 32, accessItem.MemoryUsed)
		assert.Equal(t, 100, accessItem.StorageUsed)
		assert.Equal(t, 1, accessItem.VDCCount)

		// Get access for all organizations
		allAccess, err := storage.GetOrganizationZoneAccess("")
		assert.NoError(t, err)
		assert.Len(t, allAccess, 1)

		// Get access for non-existent organization
		noAccess, err := storage.GetOrganizationZoneAccess("org-2")
		assert.NoError(t, err)
		assert.Len(t, noAccess, 0)
	})
}

// TestZoneModelHelpers tests helper methods on zone models
func TestZoneModelHelpers(t *testing.T) {
	zone := &models.Zone{
		ID:              "zone-1",
		Name:            "test-zone",
		Status:          "available",
		CPUCapacity:     100,
		MemoryCapacity:  512,
		StorageCapacity: 2000,
		CPUQuota:        80,
		MemoryQuota:     400,
		StorageQuota:    1600,
	}

	t.Run("GetAvailableCapacity", func(t *testing.T) {
		cpu, memory, storage := zone.GetAvailableCapacity()
		assert.Equal(t, 20, cpu)      // 100 - 80
		assert.Equal(t, 112, memory)  // 512 - 400
		assert.Equal(t, 400, storage) // 2000 - 1600
	})

	t.Run("IsHealthy", func(t *testing.T) {
		assert.True(t, zone.IsHealthy())

		zone.Status = "maintenance"
		assert.False(t, zone.IsHealthy())

		zone.Status = "unavailable"
		assert.False(t, zone.IsHealthy())
	})

	t.Run("GetUtilizationPercentage", func(t *testing.T) {
		zone.Status = "available" // Reset to healthy state

		used := models.ZoneUtilization{
			CPUUsed:     40,
			MemoryUsed:  200,
			StorageUsed: 800,
		}

		cpuPercent, memoryPercent, storagePercent := zone.GetUtilizationPercentage(used)
		assert.Equal(t, 50.0, cpuPercent)     // 40/80 * 100
		assert.Equal(t, 50.0, memoryPercent)  // 200/400 * 100
		assert.Equal(t, 50.0, storagePercent) // 800/1600 * 100
	})

	t.Run("CanAccommodateVDC", func(t *testing.T) {
		currentUsage := models.ZoneUtilization{
			CPUUsed:     40,
			MemoryUsed:  200,
			StorageUsed: 800,
		}

		// Test successful accommodation
		canAccommodate := zone.CanAccommodateVDC(20, 100, 400, currentUsage)
		assert.True(t, canAccommodate)

		// Test CPU limitation
		canAccommodate = zone.CanAccommodateVDC(50, 100, 400, currentUsage)
		assert.False(t, canAccommodate)

		// Test memory limitation
		canAccommodate = zone.CanAccommodateVDC(20, 250, 400, currentUsage)
		assert.False(t, canAccommodate)

		// Test storage limitation
		canAccommodate = zone.CanAccommodateVDC(20, 100, 900, currentUsage)
		assert.False(t, canAccommodate)

		// Test unhealthy zone
		zone.Status = "maintenance"
		canAccommodate = zone.CanAccommodateVDC(10, 50, 200, currentUsage)
		assert.False(t, canAccommodate)
	})
}

// TestZoneTimestampHandling tests timestamp handling in zone operations
func TestZoneTimestampHandling(t *testing.T) {
	storage, err := NewMemoryStorageForTest()
	require.NoError(t, err)

	zone := &models.Zone{
		ID:          "zone-1",
		Name:        "test-zone",
		ClusterName: "test-cluster",
		APIUrl:      "https://api.test-cluster.example.com:6443",
		Status:      "available",
	}

	t.Run("CreateZoneSetsTimestamps", func(t *testing.T) {
		before := time.Now()
		err := storage.CreateZone(zone)
		after := time.Now()
		assert.NoError(t, err)

		retrieved, err := storage.GetZone("zone-1")
		assert.NoError(t, err)
		assert.True(t, retrieved.CreatedAt.After(before) || retrieved.CreatedAt.Equal(before))
		assert.True(t, retrieved.CreatedAt.Before(after) || retrieved.CreatedAt.Equal(after))
		assert.True(t, retrieved.UpdatedAt.After(before) || retrieved.UpdatedAt.Equal(before))
		assert.True(t, retrieved.UpdatedAt.Before(after) || retrieved.UpdatedAt.Equal(after))
		assert.True(t, retrieved.LastSync.After(before) || retrieved.LastSync.Equal(before))
		assert.True(t, retrieved.LastSync.Before(after) || retrieved.LastSync.Equal(after))
	})

	t.Run("UpdateZoneUpdatesTimestamp", func(t *testing.T) {
		// Wait a bit to ensure different timestamp
		time.Sleep(10 * time.Millisecond)

		original, err := storage.GetZone("zone-1")
		require.NoError(t, err)

		before := time.Now()
		zone.Status = "maintenance"
		err = storage.UpdateZone(zone)
		after := time.Now()
		assert.NoError(t, err)

		updated, err := storage.GetZone("zone-1")
		assert.NoError(t, err)
		assert.Equal(t, original.CreatedAt, updated.CreatedAt) // Should not change
		assert.True(t, updated.UpdatedAt.After(original.UpdatedAt))
		assert.True(t, updated.UpdatedAt.After(before) || updated.UpdatedAt.Equal(before))
		assert.True(t, updated.UpdatedAt.Before(after) || updated.UpdatedAt.Equal(after))
	})
}
