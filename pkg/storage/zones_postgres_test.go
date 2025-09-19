package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// TestPostgresZoneOperations tests zone operations with PostgreSQL storage
// This test requires a test database to be available
func TestPostgresZoneOperations(t *testing.T) {
	// Skip if no test database is configured
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping PostgreSQL tests")
	}

	storage, err := NewPostgresStorage(dsn)
	require.NoError(t, err)
	defer storage.Close()

	// Clean up any existing test data
	cleanupTestZones(t, storage)

	t.Run("PostgresZoneCRUD", func(t *testing.T) {
		testZoneCRUD(t, storage)
	})

	t.Run("PostgresOrganizationZoneQuotaCRUD", func(t *testing.T) {
		testOrganizationZoneQuotaCRUD(t, storage)
	})

	t.Run("PostgresZoneUtilizationView", func(t *testing.T) {
		testZoneUtilizationView(t, storage)
	})

	t.Run("PostgresOrganizationZoneAccessView", func(t *testing.T) {
		testOrganizationZoneAccessView(t, storage)
	})

	// Clean up after tests
	cleanupTestZones(t, storage)
}

func testZoneCRUD(t *testing.T, storage Storage) {
	zone := &models.Zone{
		ID:              "test-zone-1",
		Name:            "postgres-test-zone",
		ClusterName:     "postgres-test-cluster",
		APIUrl:          "https://api.postgres-test.example.com:6443",
		Status:          "available",
		Region:          "us-east-1",
		CloudProvider:   "aws",
		NodeCount:       3,
		CPUCapacity:     50,
		MemoryCapacity:  256,
		StorageCapacity: 1000,
		CPUQuota:        40,
		MemoryQuota:     200,
		StorageQuota:    800,
		Labels: models.StringMap{
			"environment": "test",
			"managed-by":  "ovim",
		},
		Annotations: models.StringMap{
			"description": "Test zone for PostgreSQL storage",
		},
	}

	// Create
	err := storage.CreateZone(zone)
	assert.NoError(t, err)

	// Read
	retrieved, err := storage.GetZone("test-zone-1")
	assert.NoError(t, err)
	assert.Equal(t, zone.ID, retrieved.ID)
	assert.Equal(t, zone.Name, retrieved.Name)
	assert.Equal(t, zone.ClusterName, retrieved.ClusterName)
	assert.Equal(t, zone.APIUrl, retrieved.APIUrl)
	assert.Equal(t, zone.Status, retrieved.Status)
	assert.Equal(t, zone.Region, retrieved.Region)
	assert.Equal(t, zone.CloudProvider, retrieved.CloudProvider)
	assert.Equal(t, zone.NodeCount, retrieved.NodeCount)
	assert.Equal(t, zone.CPUCapacity, retrieved.CPUCapacity)
	assert.Equal(t, zone.MemoryCapacity, retrieved.MemoryCapacity)
	assert.Equal(t, zone.StorageCapacity, retrieved.StorageCapacity)
	assert.Equal(t, zone.CPUQuota, retrieved.CPUQuota)
	assert.Equal(t, zone.MemoryQuota, retrieved.MemoryQuota)
	assert.Equal(t, zone.StorageQuota, retrieved.StorageQuota)
	assert.NotZero(t, retrieved.CreatedAt)
	assert.NotZero(t, retrieved.UpdatedAt)
	assert.NotZero(t, retrieved.LastSync)

	// Verify labels and annotations
	assert.Equal(t, "test", string(retrieved.Labels["environment"]))
	assert.Equal(t, "ovim", string(retrieved.Labels["managed-by"]))
	assert.Equal(t, "Test zone for PostgreSQL storage", string(retrieved.Annotations["description"]))

	// List
	zones, err := storage.ListZones()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(zones), 1)

	found := false
	for _, z := range zones {
		if z.ID == "test-zone-1" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// Update
	zone.Status = "maintenance"
	zone.CPUQuota = 45
	zone.Labels["status"] = "updated"
	err = storage.UpdateZone(zone)
	assert.NoError(t, err)

	updated, err := storage.GetZone("test-zone-1")
	assert.NoError(t, err)
	assert.Equal(t, "maintenance", updated.Status)
	assert.Equal(t, 45, updated.CPUQuota)
	assert.Equal(t, "updated", string(updated.Labels["status"]))

	// Delete
	err = storage.DeleteZone("test-zone-1")
	assert.NoError(t, err)

	// Verify deletion
	_, err = storage.GetZone("test-zone-1")
	assert.Equal(t, ErrNotFound, err)
}

func testOrganizationZoneQuotaCRUD(t *testing.T, storage Storage) {
	// Create test zone first
	zone := &models.Zone{
		ID:           "test-zone-2",
		Name:         "quota-test-zone",
		ClusterName:  "quota-test-cluster",
		APIUrl:       "https://api.quota-test.example.com:6443",
		Status:       "available",
		CPUQuota:     50,
		MemoryQuota:  200,
		StorageQuota: 1000,
	}
	err := storage.CreateZone(zone)
	require.NoError(t, err)

	// Create test organization
	org := &models.Organization{
		ID:        "test-org-1",
		Name:      "Test Organization",
		IsEnabled: true,
	}
	err = storage.CreateOrganization(org)
	require.NoError(t, err)

	quota := &models.OrganizationZoneQuota{
		ID:             "test-quota-1",
		OrganizationID: "test-org-1",
		ZoneID:         "test-zone-2",
		CPUQuota:       20,
		MemoryQuota:    80,
		StorageQuota:   400,
		IsAllowed:      true,
	}

	// Create
	err = storage.CreateOrganizationZoneQuota(quota)
	assert.NoError(t, err)

	// Read
	retrieved, err := storage.GetOrganizationZoneQuota("test-org-1", "test-zone-2")
	assert.NoError(t, err)
	assert.Equal(t, quota.OrganizationID, retrieved.OrganizationID)
	assert.Equal(t, quota.ZoneID, retrieved.ZoneID)
	assert.Equal(t, quota.CPUQuota, retrieved.CPUQuota)
	assert.Equal(t, quota.MemoryQuota, retrieved.MemoryQuota)
	assert.Equal(t, quota.StorageQuota, retrieved.StorageQuota)
	assert.Equal(t, quota.IsAllowed, retrieved.IsAllowed)
	assert.NotZero(t, retrieved.CreatedAt)
	assert.NotZero(t, retrieved.UpdatedAt)

	// Verify zone relationship is loaded
	assert.NotNil(t, retrieved.Zone)
	assert.Equal(t, "test-zone-2", retrieved.Zone.ID)
	assert.Equal(t, "quota-test-zone", retrieved.Zone.Name)

	// List all quotas
	quotas, err := storage.ListOrganizationZoneQuotas("")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(quotas), 1)

	// List quotas for specific organization
	orgQuotas, err := storage.ListOrganizationZoneQuotas("test-org-1")
	assert.NoError(t, err)
	assert.Len(t, orgQuotas, 1)
	assert.Equal(t, "test-org-1", orgQuotas[0].OrganizationID)
	assert.NotNil(t, orgQuotas[0].Zone)

	// Update
	quota.CPUQuota = 25
	quota.IsAllowed = false
	err = storage.UpdateOrganizationZoneQuota(quota)
	assert.NoError(t, err)

	updated, err := storage.GetOrganizationZoneQuota("test-org-1", "test-zone-2")
	assert.NoError(t, err)
	assert.Equal(t, 25, updated.CPUQuota)
	assert.False(t, updated.IsAllowed)

	// Delete
	err = storage.DeleteOrganizationZoneQuota("test-org-1", "test-zone-2")
	assert.NoError(t, err)

	// Verify deletion
	_, err = storage.GetOrganizationZoneQuota("test-org-1", "test-zone-2")
	assert.Equal(t, ErrNotFound, err)

	// Clean up
	storage.DeleteZone("test-zone-2")
	storage.DeleteOrganization("test-org-1")
}

func testZoneUtilizationView(t *testing.T, storage Storage) {
	// Create test zone
	zone := &models.Zone{
		ID:              "test-zone-3",
		Name:            "utilization-test-zone",
		ClusterName:     "utilization-test-cluster",
		APIUrl:          "https://api.utilization-test.example.com:6443",
		Status:          "available",
		CPUCapacity:     100,
		MemoryCapacity:  400,
		StorageCapacity: 2000,
		CPUQuota:        80,
		MemoryQuota:     320,
		StorageQuota:    1600,
	}
	err := storage.CreateZone(zone)
	require.NoError(t, err)

	// Create test organization
	org := &models.Organization{
		ID:        "test-org-2",
		Name:      "Utilization Test Org",
		IsEnabled: true,
	}
	err = storage.CreateOrganization(org)
	require.NoError(t, err)

	// Create test VDCs to generate utilization data
	vdc1 := &models.VirtualDataCenter{
		ID:           "test-vdc-1",
		Name:         "Test VDC 1",
		OrgID:        "test-org-2",
		ZoneID:       &zone.ID,
		Phase:        "Active",
		CPUQuota:     20,
		MemoryQuota:  64,
		StorageQuota: 200,
	}
	err = storage.CreateVDC(vdc1)
	require.NoError(t, err)

	vdc2 := &models.VirtualDataCenter{
		ID:           "test-vdc-2",
		Name:         "Test VDC 2",
		OrgID:        "test-org-2",
		ZoneID:       &zone.ID,
		Phase:        "Active",
		CPUQuota:     15,
		MemoryQuota:  32,
		StorageQuota: 150,
	}
	err = storage.CreateVDC(vdc2)
	require.NoError(t, err)

	// Test zone utilization view
	utilization, err := storage.GetZoneUtilization()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(utilization), 1)

	// Find our test zone
	var testZoneUtil *models.ZoneUtilization
	for _, util := range utilization {
		if util.ID == "test-zone-3" {
			testZoneUtil = util
			break
		}
	}
	assert.NotNil(t, testZoneUtil)
	assert.Equal(t, "test-zone-3", testZoneUtil.ID)
	assert.Equal(t, "utilization-test-zone", testZoneUtil.Name)
	assert.Equal(t, "available", testZoneUtil.Status)
	assert.Equal(t, 100, testZoneUtil.CPUCapacity)
	assert.Equal(t, 400, testZoneUtil.MemoryCapacity)
	assert.Equal(t, 2000, testZoneUtil.StorageCapacity)
	assert.Equal(t, 80, testZoneUtil.CPUQuota)
	assert.Equal(t, 320, testZoneUtil.MemoryQuota)
	assert.Equal(t, 1600, testZoneUtil.StorageQuota)
	assert.Equal(t, 35, testZoneUtil.CPUUsed)      // 20 + 15
	assert.Equal(t, 96, testZoneUtil.MemoryUsed)   // 64 + 32
	assert.Equal(t, 350, testZoneUtil.StorageUsed) // 200 + 150
	assert.Equal(t, 2, testZoneUtil.VDCCount)
	assert.Equal(t, 2, testZoneUtil.ActiveVDCCount)

	// Clean up
	storage.DeleteVDC("test-vdc-1")
	storage.DeleteVDC("test-vdc-2")
	storage.DeleteOrganization("test-org-2")
	storage.DeleteZone("test-zone-3")
}

func testOrganizationZoneAccessView(t *testing.T, storage Storage) {
	// Create test zone
	zone := &models.Zone{
		ID:           "test-zone-4",
		Name:         "access-test-zone",
		ClusterName:  "access-test-cluster",
		APIUrl:       "https://api.access-test.example.com:6443",
		Status:       "available",
		CPUQuota:     60,
		MemoryQuota:  240,
		StorageQuota: 1200,
	}
	err := storage.CreateZone(zone)
	require.NoError(t, err)

	// Create test organization
	org := &models.Organization{
		ID:        "test-org-3",
		Name:      "Access Test Org",
		IsEnabled: true,
	}
	err = storage.CreateOrganization(org)
	require.NoError(t, err)

	// Create organization zone quota
	quota := &models.OrganizationZoneQuota{
		ID:             "test-quota-2",
		OrganizationID: "test-org-3",
		ZoneID:         "test-zone-4",
		CPUQuota:       30,
		MemoryQuota:    120,
		StorageQuota:   600,
		IsAllowed:      true,
	}
	err = storage.CreateOrganizationZoneQuota(quota)
	require.NoError(t, err)

	// Create test VDC to generate usage data
	vdc := &models.VirtualDataCenter{
		ID:           "test-vdc-3",
		Name:         "Access Test VDC",
		OrgID:        "test-org-3",
		ZoneID:       &zone.ID,
		Phase:        "Active",
		CPUQuota:     10,
		MemoryQuota:  40,
		StorageQuota: 200,
	}
	err = storage.CreateVDC(vdc)
	require.NoError(t, err)

	// Test organization zone access view
	access, err := storage.GetOrganizationZoneAccess("test-org-3")
	assert.NoError(t, err)
	assert.Len(t, access, 1)

	accessItem := access[0]
	assert.Equal(t, "test-org-3", accessItem.OrganizationID)
	assert.Equal(t, "test-zone-4", accessItem.ZoneID)
	assert.Equal(t, "access-test-zone", accessItem.ZoneName)
	assert.Equal(t, "available", accessItem.ZoneStatus)
	assert.Equal(t, 30, accessItem.CPUQuota)
	assert.Equal(t, 120, accessItem.MemoryQuota)
	assert.Equal(t, 600, accessItem.StorageQuota)
	assert.True(t, accessItem.IsAllowed)
	assert.Equal(t, 10, accessItem.CPUUsed)
	assert.Equal(t, 40, accessItem.MemoryUsed)
	assert.Equal(t, 200, accessItem.StorageUsed)
	assert.Equal(t, 1, accessItem.VDCCount)

	// Test getting access for all organizations
	allAccess, err := storage.GetOrganizationZoneAccess("")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(allAccess), 1)

	// Clean up
	storage.DeleteVDC("test-vdc-3")
	storage.DeleteOrganizationZoneQuota("test-org-3", "test-zone-4")
	storage.DeleteOrganization("test-org-3")
	storage.DeleteZone("test-zone-4")
}

// cleanupTestZones removes any test zones that might be left over from previous runs
func cleanupTestZones(t *testing.T, storage Storage) {
	zones, err := storage.ListZones()
	if err != nil {
		return // Skip cleanup if we can't list zones
	}

	for _, zone := range zones {
		if zone.Name == "postgres-test-zone" || zone.Name == "quota-test-zone" ||
			zone.Name == "utilization-test-zone" || zone.Name == "access-test-zone" {
			storage.DeleteZone(zone.ID)
		}
	}

	// Clean up test organizations
	orgs, err := storage.ListOrganizations()
	if err != nil {
		return
	}

	for _, org := range orgs {
		if org.Name == "Test Organization" || org.Name == "Utilization Test Org" ||
			org.Name == "Access Test Org" {
			storage.DeleteOrganization(org.ID)
		}
	}
}

// TestPostgresZoneConstraints tests database constraints and foreign keys
func TestPostgresZoneConstraints(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping PostgreSQL constraint tests")
	}

	storage, err := NewPostgresStorage(dsn)
	require.NoError(t, err)
	defer storage.Close()

	t.Run("ZoneUniqueConstraints", func(t *testing.T) {
		zone1 := &models.Zone{
			ID:          "constraint-test-1",
			Name:        "unique-test-zone",
			ClusterName: "constraint-test-cluster",
			APIUrl:      "https://api.constraint-test.example.com:6443",
			Status:      "available",
		}

		zone2 := &models.Zone{
			ID:          "constraint-test-2",
			Name:        "unique-test-zone", // Same name
			ClusterName: "constraint-test-cluster-2",
			APIUrl:      "https://api.constraint-test-2.example.com:6443",
			Status:      "available",
		}

		// Create first zone
		err := storage.CreateZone(zone1)
		assert.NoError(t, err)

		// Try to create second zone with same name - should fail
		err = storage.CreateZone(zone2)
		assert.Error(t, err)

		// Clean up
		storage.DeleteZone("constraint-test-1")
	})

	t.Run("OrganizationZoneQuotaForeignKeys", func(t *testing.T) {
		// Try to create quota for non-existent zone
		quota := &models.OrganizationZoneQuota{
			ID:             "fk-test-1",
			OrganizationID: "test-org",
			ZoneID:         "non-existent-zone",
			CPUQuota:       10,
			MemoryQuota:    32,
			StorageQuota:   100,
			IsAllowed:      true,
		}

		err := storage.CreateOrganizationZoneQuota(quota)
		assert.Error(t, err) // Should fail due to foreign key constraint
	})

	t.Run("VDCZoneForeignKey", func(t *testing.T) {
		// Create organization first
		org := &models.Organization{
			ID:        "fk-test-org",
			Name:      "FK Test Org",
			IsEnabled: true,
		}
		err := storage.CreateOrganization(org)
		require.NoError(t, err)

		// Try to create VDC with non-existent zone
		nonExistentZoneID := "non-existent-zone"
		vdc := &models.VirtualDataCenter{
			ID:           "fk-test-vdc",
			Name:         "FK Test VDC",
			OrgID:        "fk-test-org",
			ZoneID:       &nonExistentZoneID,
			Phase:        "Pending",
			CPUQuota:     5,
			MemoryQuota:  16,
			StorageQuota: 50,
		}

		err = storage.CreateVDC(vdc)
		assert.Error(t, err) // Should fail due to foreign key constraint

		// Clean up
		storage.DeleteOrganization("fk-test-org")
	})
}

// BenchmarkZoneOperations benchmarks zone operations
func BenchmarkZoneOperations(b *testing.B) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		b.Skip("TEST_DATABASE_URL not set, skipping PostgreSQL benchmarks")
	}

	storage, err := NewPostgresStorage(dsn)
	require.NoError(b, err)
	defer storage.Close()

	zone := &models.Zone{
		ID:           "bench-zone",
		Name:         "benchmark-zone",
		ClusterName:  "benchmark-cluster",
		APIUrl:       "https://api.benchmark.example.com:6443",
		Status:       "available",
		CPUQuota:     50,
		MemoryQuota:  200,
		StorageQuota: 1000,
	}

	b.Run("CreateZone", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			zone.ID = "bench-zone-" + string(rune(i))
			storage.CreateZone(zone)
		}
	})

	b.Run("GetZone", func(b *testing.B) {
		storage.CreateZone(zone)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			storage.GetZone("bench-zone")
		}

		storage.DeleteZone("bench-zone")
	})

	b.Run("ListZones", func(b *testing.B) {
		storage.CreateZone(zone)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			storage.ListZones()
		}

		storage.DeleteZone("bench-zone")
	})

	b.Run("GetZoneUtilization", func(b *testing.B) {
		storage.CreateZone(zone)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			storage.GetZoneUtilization()
		}

		storage.DeleteZone("bench-zone")
	})
}
