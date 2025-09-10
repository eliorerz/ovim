package integration

import (
	"testing"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStorageIntegration(t *testing.T) {
	testStorageIntegration(t, func() (storage.Storage, error) {
		return storage.NewMemoryStorage()
	})
}

func TestPostgreSQLStorageIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PostgreSQL integration test in short mode")
	}

	dsn := "postgres://ovim:ovim123@localhost:5432/ovim_test?sslmode=disable"
	testStorageIntegration(t, func() (storage.Storage, error) {
		return storage.NewPostgresStorage(dsn)
	})
}

func testStorageIntegration(t *testing.T, storageFactory func() (storage.Storage, error)) {
	storage, err := storageFactory()
	require.NoError(t, err)
	defer storage.Close()

	t.Run("UserOperations", func(t *testing.T) {
		testUserOperations(t, storage)
	})

	t.Run("OrganizationOperations", func(t *testing.T) {
		testOrganizationOperations(t, storage)
	})

	t.Run("VDCOperations", func(t *testing.T) {
		testVDCOperations(t, storage)
	})

	t.Run("TemplateOperations", func(t *testing.T) {
		testTemplateOperations(t, storage)
	})

	t.Run("VMOperations", func(t *testing.T) {
		testVMOperations(t, storage)
	})
}

func testUserOperations(t *testing.T, s storage.Storage) {
	// Test that only admin user exists
	adminUser, err := s.GetUserByUsername("admin")
	require.NoError(t, err)
	assert.NotNil(t, adminUser)
	assert.Equal(t, "admin", adminUser.Username)
	assert.Equal(t, models.RoleSystemAdmin, adminUser.Role)
	assert.Nil(t, adminUser.OrgID)

	// Test that non-existent users return error
	_, err = s.GetUserByUsername("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func testOrganizationOperations(t *testing.T, s storage.Storage) {
	// Test that initially no organizations exist
	orgs, err := s.ListOrganizations()
	require.NoError(t, err)
	assert.Len(t, orgs, 0)

	// Test getting non-existent organization
	_, err = s.GetOrganization("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func testVDCOperations(t *testing.T, s storage.Storage) {
	// Test that initially no VDCs exist
	vdcs, err := s.ListVDCs("")
	require.NoError(t, err)
	assert.Len(t, vdcs, 0)

	// Test getting non-existent VDC
	_, err = s.GetVDC("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func testTemplateOperations(t *testing.T, s storage.Storage) {
	// Test that initially no templates exist
	templates, err := s.ListTemplates()
	require.NoError(t, err)
	assert.Len(t, templates, 0)

	// Test organization-specific template listing for non-existent org
	nonexistentOrgTemplates, err := s.ListTemplatesByOrg("nonexistent-org")
	require.NoError(t, err)
	assert.Len(t, nonexistentOrgTemplates, 0)

	// Test getting non-existent template
	_, err = s.GetTemplate("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func testVMOperations(t *testing.T, s storage.Storage) {
	// Test that initially no VMs exist
	initialVMs, err := s.ListVMs("")
	require.NoError(t, err)
	assert.Len(t, initialVMs, 0)

	// Test getting non-existent VM
	_, err = s.GetVM("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func TestStorageHealthCheck(t *testing.T) {
	memStorage, err := storage.NewMemoryStorage()
	require.NoError(t, err)
	defer memStorage.Close()

	err = memStorage.Ping()
	assert.NoError(t, err)

	if !testing.Short() {
		dsn := "postgres://ovim:ovim123@localhost:5432/ovim_test?sslmode=disable"
		pgStorage, err := storage.NewPostgresStorage(dsn)
		if err == nil {
			defer pgStorage.Close()
			err = pgStorage.Ping()
			assert.NoError(t, err)
		}
	}
}
