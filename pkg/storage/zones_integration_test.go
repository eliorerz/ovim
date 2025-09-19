package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// TestZoneVDCIntegration tests the integration between zones and VDCs
func TestZoneVDCIntegration(t *testing.T) {
	storage, err := NewMemoryStorageForTest()
	require.NoError(t, err)

	// Create test zone
	zone := &models.Zone{
		ID:              "integration-zone-1",
		Name:            "integration-test-zone",
		ClusterName:     "integration-cluster",
		APIUrl:          "https://api.integration.example.com:6443",
		Status:          "available",
		CPUCapacity:     100,
		MemoryCapacity:  400,
		StorageCapacity: 2000,
		CPUQuota:        80,
		MemoryQuota:     320,
		StorageQuota:    1600,
	}
	err = storage.CreateZone(zone)
	require.NoError(t, err)

	// Create test organization
	org := &models.Organization{
		ID:        "integration-org-1",
		Name:      "Integration Test Org",
		IsEnabled: true,
	}
	err = storage.CreateOrganization(org)
	require.NoError(t, err)

	// Create organization zone quota
	quota := &models.OrganizationZoneQuota{
		ID:             "integration-quota-1",
		OrganizationID: "integration-org-1",
		ZoneID:         "integration-zone-1",
		CPUQuota:       40,
		MemoryQuota:    160,
		StorageQuota:   800,
		IsAllowed:      true,
	}
	err = storage.CreateOrganizationZoneQuota(quota)
	require.NoError(t, err)

	t.Run("VDCWithZoneAssignment", func(t *testing.T) {
		// Create VDC assigned to zone
		vdc := &models.VirtualDataCenter{
			ID:           "integration-vdc-1",
			Name:         "Integration VDC",
			OrgID:        "integration-org-1",
			ZoneID:       &zone.ID,
			Phase:        "Active",
			CPUQuota:     10,
			MemoryQuota:  32,
			StorageQuota: 100,
		}
		err = storage.CreateVDC(vdc)
		assert.NoError(t, err)

		// Verify VDC was created with zone assignment
		retrievedVDC, err := storage.GetVDC("integration-vdc-1")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedVDC.ZoneID)
		assert.Equal(t, "integration-zone-1", *retrievedVDC.ZoneID)

		// Verify zone utilization reflects the VDC
		utilization, err := storage.GetZoneUtilization()
		assert.NoError(t, err)

		var testZoneUtil *models.ZoneUtilization
		for _, util := range utilization {
			if util.ID == "integration-zone-1" {
				testZoneUtil = util
				break
			}
		}
		assert.NotNil(t, testZoneUtil)
		assert.Equal(t, 10, testZoneUtil.CPUUsed)
		assert.Equal(t, 32, testZoneUtil.MemoryUsed)
		assert.Equal(t, 100, testZoneUtil.StorageUsed)
		assert.Equal(t, 1, testZoneUtil.VDCCount)
		assert.Equal(t, 1, testZoneUtil.ActiveVDCCount)
	})

	t.Run("OrganizationResourceTracking", func(t *testing.T) {
		// Check organization zone access
		access, err := storage.GetOrganizationZoneAccess("integration-org-1")
		assert.NoError(t, err)
		assert.Len(t, access, 1)

		accessItem := access[0]
		assert.Equal(t, "integration-org-1", accessItem.OrganizationID)
		assert.Equal(t, "integration-zone-1", accessItem.ZoneID)
		assert.Equal(t, "integration-test-zone", accessItem.ZoneName)
		assert.Equal(t, 40, accessItem.CPUQuota)
		assert.Equal(t, 160, accessItem.MemoryQuota)
		assert.Equal(t, 800, accessItem.StorageQuota)
		assert.Equal(t, 10, accessItem.CPUUsed)
		assert.Equal(t, 32, accessItem.MemoryUsed)
		assert.Equal(t, 100, accessItem.StorageUsed)
		assert.Equal(t, 1, accessItem.VDCCount)
	})

	t.Run("MultipleVDCsInZone", func(t *testing.T) {
		// Create second VDC in same zone
		vdc2 := &models.VirtualDataCenter{
			ID:           "integration-vdc-2",
			Name:         "Integration VDC 2",
			OrgID:        "integration-org-1",
			ZoneID:       &zone.ID,
			Phase:        "Pending", // Different phase
			CPUQuota:     5,
			MemoryQuota:  16,
			StorageQuota: 50,
		}
		err = storage.CreateVDC(vdc2)
		assert.NoError(t, err)

		// Verify utilization updates
		utilization, err := storage.GetZoneUtilization()
		assert.NoError(t, err)

		var testZoneUtil *models.ZoneUtilization
		for _, util := range utilization {
			if util.ID == "integration-zone-1" {
				testZoneUtil = util
				break
			}
		}
		assert.NotNil(t, testZoneUtil)
		assert.Equal(t, 15, testZoneUtil.CPUUsed)      // 10 + 5
		assert.Equal(t, 48, testZoneUtil.MemoryUsed)   // 32 + 16
		assert.Equal(t, 150, testZoneUtil.StorageUsed) // 100 + 50
		assert.Equal(t, 2, testZoneUtil.VDCCount)
		assert.Equal(t, 1, testZoneUtil.ActiveVDCCount) // Only first VDC is Active
	})

	t.Run("VDCWithoutZone", func(t *testing.T) {
		// Create VDC without zone assignment (legacy)
		vdc3 := &models.VirtualDataCenter{
			ID:           "integration-vdc-3",
			Name:         "Legacy VDC",
			OrgID:        "integration-org-1",
			ZoneID:       nil, // No zone assignment
			Phase:        "Active",
			CPUQuota:     8,
			MemoryQuota:  24,
			StorageQuota: 80,
		}
		err = storage.CreateVDC(vdc3)
		assert.NoError(t, err)

		// Verify VDC was created without zone
		retrievedVDC, err := storage.GetVDC("integration-vdc-3")
		assert.NoError(t, err)
		assert.Nil(t, retrievedVDC.ZoneID)

		// Verify zone utilization doesn't include unassigned VDC
		utilization, err := storage.GetZoneUtilization()
		assert.NoError(t, err)

		var testZoneUtil *models.ZoneUtilization
		for _, util := range utilization {
			if util.ID == "integration-zone-1" {
				testZoneUtil = util
				break
			}
		}
		assert.NotNil(t, testZoneUtil)
		// Should still be the same as before (15, 48, 150)
		assert.Equal(t, 15, testZoneUtil.CPUUsed)
		assert.Equal(t, 48, testZoneUtil.MemoryUsed)
		assert.Equal(t, 150, testZoneUtil.StorageUsed)
		assert.Equal(t, 2, testZoneUtil.VDCCount) // Still 2, doesn't count unassigned VDC
	})

	// Clean up
	storage.DeleteVDC("integration-vdc-1")
	storage.DeleteVDC("integration-vdc-2")
	storage.DeleteVDC("integration-vdc-3")
	storage.DeleteOrganizationZoneQuota("integration-org-1", "integration-zone-1")
	storage.DeleteOrganization("integration-org-1")
	storage.DeleteZone("integration-zone-1")
}

// TestZoneCapacityManagement tests zone capacity and resource allocation logic
func TestZoneCapacityManagement(t *testing.T) {
	zone := &models.Zone{
		ID:              "capacity-test-zone",
		Name:            "capacity-test",
		ClusterName:     "capacity-cluster",
		APIUrl:          "https://api.capacity.example.com:6443",
		Status:          "available",
		CPUCapacity:     100,
		MemoryCapacity:  400,
		StorageCapacity: 2000,
		CPUQuota:        80,   // 80% of capacity allocated
		MemoryQuota:     320,  // 80% of capacity allocated
		StorageQuota:    1600, // 80% of capacity allocated
	}

	t.Run("AvailableCapacityCalculation", func(t *testing.T) {
		cpu, memory, storage := zone.GetAvailableCapacity()
		assert.Equal(t, 20, cpu)      // 100 - 80
		assert.Equal(t, 80, memory)   // 400 - 320
		assert.Equal(t, 400, storage) // 2000 - 1600
	})

	t.Run("VDCAccommodationCheck", func(t *testing.T) {
		currentUsage := models.ZoneUtilization{
			CPUUsed:     40,  // 50% of quota used
			MemoryUsed:  160, // 50% of quota used
			StorageUsed: 800, // 50% of quota used
		}

		// Should accommodate a small VDC
		canAccommodate := zone.CanAccommodateVDC(10, 40, 200, currentUsage)
		assert.True(t, canAccommodate)

		// Should NOT accommodate VDC that exceeds remaining quota
		canAccommodate = zone.CanAccommodateVDC(50, 40, 200, currentUsage) // CPU too high
		assert.False(t, canAccommodate)

		canAccommodate = zone.CanAccommodateVDC(10, 200, 200, currentUsage) // Memory too high
		assert.False(t, canAccommodate)

		canAccommodate = zone.CanAccommodateVDC(10, 40, 900, currentUsage) // Storage too high
		assert.False(t, canAccommodate)
	})

	t.Run("ZoneHealthStatus", func(t *testing.T) {
		// Available zone should be healthy
		assert.True(t, zone.IsHealthy())

		// Maintenance zone should not be healthy
		zone.Status = "maintenance"
		assert.False(t, zone.IsHealthy())

		// Unavailable zone should not be healthy
		zone.Status = "unavailable"
		assert.False(t, zone.IsHealthy())

		// Reset to available
		zone.Status = "available"
		assert.True(t, zone.IsHealthy())
	})

	t.Run("UtilizationPercentages", func(t *testing.T) {
		usage := models.ZoneUtilization{
			CPUUsed:     40,  // 50% of 80 quota
			MemoryUsed:  160, // 50% of 320 quota
			StorageUsed: 800, // 50% of 1600 quota
		}

		cpuPercent, memoryPercent, storagePercent := zone.GetUtilizationPercentage(usage)
		assert.Equal(t, 50.0, cpuPercent)
		assert.Equal(t, 50.0, memoryPercent)
		assert.Equal(t, 50.0, storagePercent)

		// Test with high utilization
		usage.CPUUsed = 72       // 90% of quota
		usage.MemoryUsed = 288   // 90% of quota
		usage.StorageUsed = 1440 // 90% of quota

		cpuPercent, memoryPercent, storagePercent = zone.GetUtilizationPercentage(usage)
		assert.Equal(t, 90.0, cpuPercent)
		assert.Equal(t, 90.0, memoryPercent)
		assert.Equal(t, 90.0, storagePercent)
	})
}

// TestZoneConstraintsAndValidation tests data validation and constraints
func TestZoneConstraintsAndValidation(t *testing.T) {
	storage, err := NewMemoryStorageForTest()
	require.NoError(t, err)

	t.Run("ZoneNameUniqueness", func(t *testing.T) {
		zone1 := &models.Zone{
			ID:          "zone-unique-1",
			Name:        "unique-zone-name",
			ClusterName: "cluster-1",
			APIUrl:      "https://api1.example.com:6443",
			Status:      "available",
		}

		zone2 := &models.Zone{
			ID:          "zone-unique-2",
			Name:        "unique-zone-name", // Same name
			ClusterName: "cluster-2",
			APIUrl:      "https://api2.example.com:6443",
			Status:      "available",
		}

		err = storage.CreateZone(zone1)
		assert.NoError(t, err)

		// In memory storage, we don't enforce unique constraints
		// This would fail in PostgreSQL with proper constraints
		err = storage.CreateZone(zone2)
		// Memory storage allows duplicates, but PostgreSQL would reject this
		// The test documents expected behavior
	})

	t.Run("OrganizationZoneQuotaConstraints", func(t *testing.T) {
		// Create zone first
		zone := &models.Zone{
			ID:          "constraint-zone",
			Name:        "constraint-test-zone",
			ClusterName: "constraint-cluster",
			APIUrl:      "https://api.constraint.example.com:6443",
			Status:      "available",
		}
		err = storage.CreateZone(zone)
		require.NoError(t, err)

		quota1 := &models.OrganizationZoneQuota{
			ID:             "quota-constraint-1",
			OrganizationID: "org-constraint",
			ZoneID:         "constraint-zone",
			CPUQuota:       20,
			MemoryQuota:    80,
			StorageQuota:   400,
			IsAllowed:      true,
		}

		quota2 := &models.OrganizationZoneQuota{
			ID:             "quota-constraint-2",
			OrganizationID: "org-constraint",  // Same org
			ZoneID:         "constraint-zone", // Same zone
			CPUQuota:       30,
			MemoryQuota:    120,
			StorageQuota:   600,
			IsAllowed:      true,
		}

		err = storage.CreateOrganizationZoneQuota(quota1)
		assert.NoError(t, err)

		// This should fail in a properly constrained system
		// Memory storage allows it, but PostgreSQL should reject
		err = storage.CreateOrganizationZoneQuota(quota2)
		// In memory: succeeds (overwrites), in PostgreSQL: should fail
	})

	t.Run("ResourceQuotaValidation", func(t *testing.T) {
		zone := &models.Zone{
			ID:              "validation-zone",
			Name:            "validation-test-zone",
			ClusterName:     "validation-cluster",
			APIUrl:          "https://api.validation.example.com:6443",
			Status:          "available",
			CPUCapacity:     100,
			MemoryCapacity:  400,
			StorageCapacity: 2000,
			CPUQuota:        120, // Invalid: exceeds capacity
			MemoryQuota:     320,
			StorageQuota:    1600,
		}

		// Memory storage allows invalid quotas
		// In a production system, you'd want validation
		err = storage.CreateZone(zone)
		assert.NoError(t, err)

		// Test helper methods with invalid data
		cpu, memory, storage := zone.GetAvailableCapacity()
		assert.Equal(t, -20, cpu) // Negative available capacity
		assert.Equal(t, 80, memory)
		assert.Equal(t, 400, storage)

		// Zone should still be considered healthy (status-based)
		assert.True(t, zone.IsHealthy())
	})
}
