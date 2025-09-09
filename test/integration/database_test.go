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
	users, err := []models.User{}, error(nil)

	existingUsers := []string{"admin", "orgadmin", "user"}
	for _, username := range existingUsers {
		user, err := s.GetUserByUsername(username)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, username, user.Username)
		users = append(users, *user)
	}

	assert.Len(t, users, 3)

	adminUser := users[0]
	assert.Equal(t, models.RoleSystemAdmin, adminUser.Role)
	assert.Nil(t, adminUser.OrgID)

	orgAdminUser := users[1]
	assert.Equal(t, models.RoleOrgAdmin, orgAdminUser.Role)
	assert.NotNil(t, orgAdminUser.OrgID)
	assert.Equal(t, "org-acme", *orgAdminUser.OrgID)

	regularUser := users[2]
	assert.Equal(t, models.RoleOrgUser, regularUser.Role)
	assert.NotNil(t, regularUser.OrgID)
	assert.Equal(t, "org-acme", *regularUser.OrgID)

	_, err = s.GetUserByUsername("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func testOrganizationOperations(t *testing.T, s storage.Storage) {
	orgs, err := s.ListOrganizations()
	require.NoError(t, err)
	assert.Len(t, orgs, 2)

	orgNames := make(map[string]*models.Organization)
	for _, org := range orgs {
		orgNames[org.Name] = org
	}

	acmeOrg, exists := orgNames["Acme Corporation"]
	require.True(t, exists)
	assert.Equal(t, "org-acme", acmeOrg.ID)
	assert.Equal(t, "acme-corp", acmeOrg.Namespace)

	devOrg, exists := orgNames["Development Team"]
	require.True(t, exists)
	assert.Equal(t, "org-dev", devOrg.ID)
	assert.Equal(t, "dev-team", devOrg.Namespace)

	fetchedOrg, err := s.GetOrganization("org-acme")
	require.NoError(t, err)
	assert.Equal(t, acmeOrg.ID, fetchedOrg.ID)
	assert.Equal(t, acmeOrg.Name, fetchedOrg.Name)

	_, err = s.GetOrganization("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func testVDCOperations(t *testing.T, s storage.Storage) {
	vdcs, err := s.ListVDCs("")
	require.NoError(t, err)
	assert.Len(t, vdcs, 2)

	acmeVDCs, err := s.ListVDCs("org-acme")
	require.NoError(t, err)
	assert.Len(t, acmeVDCs, 1)

	acmeVDC := acmeVDCs[0]
	assert.Equal(t, "vdc-acme-main", acmeVDC.ID)
	assert.Equal(t, "Acme Main VDC", acmeVDC.Name)
	assert.Equal(t, "org-acme", acmeVDC.OrgID)
	assert.Equal(t, "acme-corp", acmeVDC.Namespace)

	assert.Contains(t, acmeVDC.ResourceQuotas, "cpu")
	assert.Contains(t, acmeVDC.ResourceQuotas, "memory")
	assert.Contains(t, acmeVDC.ResourceQuotas, "storage")
	assert.Equal(t, "20", acmeVDC.ResourceQuotas["cpu"])
	assert.Equal(t, "64Gi", acmeVDC.ResourceQuotas["memory"])
	assert.Equal(t, "1Ti", acmeVDC.ResourceQuotas["storage"])

	fetchedVDC, err := s.GetVDC("vdc-acme-main")
	require.NoError(t, err)
	assert.Equal(t, acmeVDC.ID, fetchedVDC.ID)
	assert.Equal(t, acmeVDC.ResourceQuotas, fetchedVDC.ResourceQuotas)

	_, err = s.GetVDC("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func testTemplateOperations(t *testing.T, s storage.Storage) {
	templates, err := s.ListTemplates()
	require.NoError(t, err)
	assert.Len(t, templates, 3)

	templateNames := make(map[string]*models.Template)
	for _, tmpl := range templates {
		templateNames[tmpl.Name] = tmpl
	}

	rhel, exists := templateNames["Red Hat Enterprise Linux 9.2"]
	require.True(t, exists)
	assert.Equal(t, "tmpl-rhel9", rhel.ID)
	assert.Equal(t, "Linux", rhel.OSType)
	assert.Equal(t, "RHEL 9.2", rhel.OSVersion)
	assert.Equal(t, 2, rhel.CPU)
	assert.Equal(t, "4Gi", rhel.Memory)
	assert.Contains(t, rhel.Metadata, "vendor")
	assert.Equal(t, "Red Hat", rhel.Metadata["vendor"])

	ubuntu, exists := templateNames["Ubuntu Server 22.04 LTS"]
	require.True(t, exists)
	assert.Equal(t, "tmpl-ubuntu22", ubuntu.ID)
	assert.Equal(t, "Linux", ubuntu.OSType)
	assert.Equal(t, 2, ubuntu.CPU)
	assert.Equal(t, "2Gi", ubuntu.Memory)

	centos, exists := templateNames["CentOS Stream 9"]
	require.True(t, exists)
	assert.Equal(t, "tmpl-centos9", centos.ID)
	assert.Equal(t, 1, centos.CPU)

	fetchedTemplate, err := s.GetTemplate("tmpl-ubuntu22")
	require.NoError(t, err)
	assert.Equal(t, ubuntu.ID, fetchedTemplate.ID)
	assert.Equal(t, ubuntu.Name, fetchedTemplate.Name)

	_, err = s.GetTemplate("nonexistent")
	assert.Equal(t, storage.ErrNotFound, err)
}

func testVMOperations(t *testing.T, s storage.Storage) {
	initialVMs, err := s.ListVMs("")
	require.NoError(t, err)
	initialCount := len(initialVMs)

	testVM := &models.VirtualMachine{
		ID:         "vm-test-integration",
		Name:       "Integration Test VM",
		OrgID:      "org-acme",
		VDCID:      "vdc-acme-main",
		TemplateID: "tmpl-ubuntu22",
		OwnerID:    "user-orgadmin",
		Status:     models.VMStatusPending,
		CPU:        2,
		Memory:     "2Gi",
		DiskSize:   "20Gi",
		Metadata: models.StringMap{
			"test":        "true",
			"description": "VM created during integration testing",
		},
	}

	err = s.CreateVM(testVM)
	require.NoError(t, err)

	allVMs, err := s.ListVMs("")
	require.NoError(t, err)
	assert.Len(t, allVMs, initialCount+1)

	acmeVMs, err := s.ListVMs("org-acme")
	require.NoError(t, err)
	assert.Greater(t, len(acmeVMs), 0)

	fetchedVM, err := s.GetVM("vm-test-integration")
	require.NoError(t, err)
	assert.Equal(t, testVM.Name, fetchedVM.Name)
	assert.Equal(t, testVM.OrgID, fetchedVM.OrgID)
	assert.Equal(t, testVM.Status, fetchedVM.Status)
	assert.Equal(t, testVM.Metadata, fetchedVM.Metadata)

	fetchedVM.Status = models.VMStatusRunning
	err = s.UpdateVM(fetchedVM)
	require.NoError(t, err)

	updatedVM, err := s.GetVM("vm-test-integration")
	require.NoError(t, err)
	assert.Equal(t, models.VMStatusRunning, updatedVM.Status)

	err = s.DeleteVM("vm-test-integration")
	require.NoError(t, err)

	_, err = s.GetVM("vm-test-integration")
	assert.Equal(t, storage.ErrNotFound, err)

	finalVMs, err := s.ListVMs("")
	require.NoError(t, err)
	assert.Len(t, finalVMs, initialCount)
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
