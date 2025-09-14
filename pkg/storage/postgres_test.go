package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestDSN() string {
	// Check for test database environment variable
	dsn := os.Getenv("OVIM_TEST_DATABASE_URL")
	if dsn == "" {
		// Default test database DSN
		dsn = "postgres://ovim:ovim123@localhost:5432/ovim_test?sslmode=disable"
	}
	return dsn
}

func setupTestPostgresStorage(t *testing.T) Storage {
	dsn := getTestDSN()
	storage, err := NewPostgresStorage(dsn)
	if err != nil {
		t.Skipf("Skipping PostgreSQL tests - could not connect to test database: %v", err)
	}
	require.NotNil(t, storage)
	return storage
}

func TestNewPostgresStorage(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	// Test that storage implements the interface
	var _ Storage = storage

	// Test ping
	err := storage.Ping()
	assert.NoError(t, err)
}

func TestPostgresStorage_UserOperations(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	testUser := &models.User{
		ID:        fmt.Sprintf("test-user-%d", time.Now().UnixNano()),
		Username:  fmt.Sprintf("testuser-%d", time.Now().UnixNano()),
		Email:     fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
		Role:      "org_user",
		OrgID:     stringPtr("test-org"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	t.Run("CreateUser", func(t *testing.T) {
		err := storage.CreateUser(testUser)
		assert.NoError(t, err)

		// Test duplicate creation (should fail on unique constraints)
		err = storage.CreateUser(testUser)
		assert.Error(t, err)
	})

	t.Run("GetUserByID", func(t *testing.T) {
		user, err := storage.GetUserByID(testUser.ID)
		assert.NoError(t, err)
		assert.Equal(t, testUser.ID, user.ID)
		assert.Equal(t, testUser.Username, user.Username)
		assert.Equal(t, testUser.Email, user.Email)

		// Test non-existent user
		_, err = storage.GetUserByID("non-existent")
		assert.Error(t, err)
	})

	t.Run("GetUserByUsername", func(t *testing.T) {
		user, err := storage.GetUserByUsername(testUser.Username)
		assert.NoError(t, err)
		assert.Equal(t, testUser.Username, user.Username)

		// Test non-existent user
		_, err = storage.GetUserByUsername("non-existent")
		assert.Error(t, err)
	})

	t.Run("ListUsers", func(t *testing.T) {
		users, err := storage.ListUsers()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(users), 1)

		// Find our test user
		found := false
		for _, user := range users {
			if user.ID == testUser.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("ListUsersByOrg", func(t *testing.T) {
		users, err := storage.ListUsersByOrg(*testUser.OrgID)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(users), 1)

		// Test with non-existent org
		users, err = storage.ListUsersByOrg("non-existent-org")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(users))
	})

	t.Run("UpdateUser", func(t *testing.T) {
		updatedUser := *testUser
		updatedUser.Email = fmt.Sprintf("updated-%d@example.com", time.Now().UnixNano())
		updatedUser.UpdatedAt = time.Now()

		err := storage.UpdateUser(&updatedUser)
		assert.NoError(t, err)

		// Verify update
		user, err := storage.GetUserByID(testUser.ID)
		assert.NoError(t, err)
		assert.Equal(t, updatedUser.Email, user.Email)

		// Test update non-existent user
		nonExistentUser := &models.User{
			ID:       "non-existent",
			Username: "nonexistent",
			Email:    "nonexistent@example.com",
			Role:     "org_user",
			OrgID:    stringPtr("test-org"),
		}
		err = storage.UpdateUser(nonExistentUser)
		assert.Error(t, err)
	})

	t.Run("DeleteUser", func(t *testing.T) {
		err := storage.DeleteUser(testUser.ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.GetUserByID(testUser.ID)
		assert.Error(t, err)

		// Test delete non-existent user
		err = storage.DeleteUser("non-existent")
		assert.Error(t, err)
	})
}

func TestPostgresStorage_OrganizationOperations(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	testOrg := &models.Organization{
		ID:          fmt.Sprintf("test-org-%d", time.Now().UnixNano()),
		Name:        fmt.Sprintf("test-org-%d", time.Now().UnixNano()),
		Description: "Test Organization",
		IsEnabled:   true,
		DisplayName: stringPtr("Test Organization"),
		CRName:      fmt.Sprintf("test-org-%d", time.Now().UnixNano()),
		CRNamespace: "ovim-system",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	t.Run("CreateOrganization", func(t *testing.T) {
		err := storage.CreateOrganization(testOrg)
		assert.NoError(t, err)

		// Test duplicate creation
		err = storage.CreateOrganization(testOrg)
		assert.Error(t, err)
	})

	t.Run("GetOrganization", func(t *testing.T) {
		org, err := storage.GetOrganization(testOrg.ID)
		assert.NoError(t, err)
		assert.Equal(t, testOrg.ID, org.ID)
		assert.Equal(t, testOrg.Name, org.Name)

		// Test non-existent org
		_, err = storage.GetOrganization("non-existent")
		assert.Error(t, err)
	})

	t.Run("ListOrganizations", func(t *testing.T) {
		orgs, err := storage.ListOrganizations()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(orgs), 1)

		// Find our test org
		found := false
		for _, org := range orgs {
			if org.ID == testOrg.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("UpdateOrganization", func(t *testing.T) {
		updatedOrg := *testOrg
		updatedOrg.Description = "Updated Description"
		updatedOrg.IsEnabled = false

		err := storage.UpdateOrganization(&updatedOrg)
		assert.NoError(t, err)

		// Verify update
		org, err := storage.GetOrganization(testOrg.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Description", org.Description)
		assert.False(t, org.IsEnabled)

		// Test update non-existent org
		nonExistentOrg := &models.Organization{ID: "non-existent"}
		err = storage.UpdateOrganization(nonExistentOrg)
		assert.Error(t, err)
	})

	t.Run("DeleteOrganization", func(t *testing.T) {
		err := storage.DeleteOrganization(testOrg.ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.GetOrganization(testOrg.ID)
		assert.Error(t, err)

		// Test delete non-existent org
		err = storage.DeleteOrganization("non-existent")
		assert.Error(t, err)
	})
}

func TestPostgresStorage_VDCOperations(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	// First create an organization that the VDC will belong to
	testOrg := &models.Organization{
		ID:          "test-org",
		Name:        "test-org",
		Description: "Test Organization for VDC",
		IsEnabled:   true,
		DisplayName: stringPtr("Test Organization"),
		CRName:      "test-org",
		CRNamespace: "ovim-system",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err := storage.CreateOrganization(testOrg)
	require.NoError(t, err)

	testVDC := &models.VirtualDataCenter{
		ID:                fmt.Sprintf("test-vdc-%d", time.Now().UnixNano()),
		Name:              fmt.Sprintf("test-vdc-%d", time.Now().UnixNano()),
		Description:       "Test VDC",
		OrgID:             "test-org",
		CRName:            fmt.Sprintf("test-vdc-%d", time.Now().UnixNano()),
		CRNamespace:       "ovim-system",
		WorkloadNamespace: fmt.Sprintf("test-vdc-workload-%d", time.Now().UnixNano()),
		CPUQuota:          4,
		MemoryQuota:       8,
		StorageQuota:      100,
		NetworkPolicy:     "default",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	t.Run("CreateVDC", func(t *testing.T) {
		err := storage.CreateVDC(testVDC)
		assert.NoError(t, err)

		// Test duplicate creation
		err = storage.CreateVDC(testVDC)
		assert.Error(t, err)
	})

	t.Run("GetVDC", func(t *testing.T) {
		vdc, err := storage.GetVDC(testVDC.ID)
		assert.NoError(t, err)
		assert.Equal(t, testVDC.ID, vdc.ID)
		assert.Equal(t, testVDC.Name, vdc.Name)
		assert.Equal(t, testVDC.CPUQuota, vdc.CPUQuota)

		// Test non-existent VDC
		_, err = storage.GetVDC("non-existent")
		assert.Error(t, err)
	})

	t.Run("ListVDCs", func(t *testing.T) {
		vdcs, err := storage.ListVDCs(testVDC.OrgID)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(vdcs), 1)

		// Test with non-existent org
		vdcs, err = storage.ListVDCs("non-existent-org")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(vdcs))
	})

	t.Run("UpdateVDC", func(t *testing.T) {
		updatedVDC := *testVDC
		updatedVDC.Description = "Updated VDC Description"
		updatedVDC.CPUQuota = 8

		err := storage.UpdateVDC(&updatedVDC)
		assert.NoError(t, err)

		// Verify update
		vdc, err := storage.GetVDC(testVDC.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated VDC Description", vdc.Description)
		assert.Equal(t, 8, vdc.CPUQuota)

		// Test update non-existent VDC
		nonExistentVDC := &models.VirtualDataCenter{ID: "non-existent"}
		err = storage.UpdateVDC(nonExistentVDC)
		assert.Error(t, err)
	})

	t.Run("DeleteVDC", func(t *testing.T) {
		err := storage.DeleteVDC(testVDC.ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.GetVDC(testVDC.ID)
		assert.Error(t, err)

		// Test delete non-existent VDC
		err = storage.DeleteVDC("non-existent")
		assert.Error(t, err)
	})
}

func TestPostgresStorage_TemplateOperations(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	testTemplate := &models.Template{
		ID:          fmt.Sprintf("test-template-%d", time.Now().UnixNano()),
		Name:        fmt.Sprintf("test-template-%d", time.Now().UnixNano()),
		Description: "Test Template",
		OSType:      "Linux",
		OSVersion:   "Ubuntu 20.04",
		CPU:         2,
		Memory:      "4Gi",
		DiskSize:    "20Gi",
		ImageURL:    "http://example.com/image.iso",
		OrgID:       "test-org",
		Metadata:    models.StringMap{"key": "value"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	t.Run("CreateTemplate", func(t *testing.T) {
		err := storage.CreateTemplate(testTemplate)
		assert.NoError(t, err)

		// Test duplicate creation
		err = storage.CreateTemplate(testTemplate)
		assert.Error(t, err)
	})

	t.Run("GetTemplate", func(t *testing.T) {
		template, err := storage.GetTemplate(testTemplate.ID)
		assert.NoError(t, err)
		assert.Equal(t, testTemplate.ID, template.ID)
		assert.Equal(t, testTemplate.Name, template.Name)
		assert.Equal(t, testTemplate.OSType, template.OSType)

		// Test non-existent template
		_, err = storage.GetTemplate("non-existent")
		assert.Error(t, err)
	})

	t.Run("ListTemplates", func(t *testing.T) {
		templates, err := storage.ListTemplates()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(templates), 1)

		// Find our test template
		found := false
		for _, template := range templates {
			if template.ID == testTemplate.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("ListTemplatesByOrg", func(t *testing.T) {
		templates, err := storage.ListTemplatesByOrg(testTemplate.OrgID)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(templates), 1)

		// Test with non-existent org
		templates, err = storage.ListTemplatesByOrg("non-existent-org")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(templates))
	})

	t.Run("UpdateTemplate", func(t *testing.T) {
		updatedTemplate := *testTemplate
		updatedTemplate.Description = "Updated Template Description"
		updatedTemplate.CPU = 4

		err := storage.UpdateTemplate(&updatedTemplate)
		assert.NoError(t, err)

		// Verify update
		template, err := storage.GetTemplate(testTemplate.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Template Description", template.Description)
		assert.Equal(t, 4, template.CPU)

		// Test update non-existent template
		nonExistentTemplate := &models.Template{ID: "non-existent"}
		err = storage.UpdateTemplate(nonExistentTemplate)
		assert.Error(t, err)
	})

	t.Run("DeleteTemplate", func(t *testing.T) {
		err := storage.DeleteTemplate(testTemplate.ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.GetTemplate(testTemplate.ID)
		assert.Error(t, err)

		// Test delete non-existent template
		err = storage.DeleteTemplate("non-existent")
		assert.Error(t, err)
	})
}

func TestPostgresStorage_VMOperations(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	// First create an organization and VDC for the VM
	testOrg := &models.Organization{
		ID:          "test-org",
		Name:        "test-org",
		Description: "Test Organization for VM",
		IsEnabled:   true,
		DisplayName: stringPtr("Test Organization"),
		CRName:      "test-org",
		CRNamespace: "ovim-system",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err := storage.CreateOrganization(testOrg)
	require.NoError(t, err)

	testVDC := &models.VirtualDataCenter{
		ID:                "test-vdc",
		Name:              "test-vdc",
		Description:       "Test VDC for VM",
		OrgID:             "test-org",
		CRName:            "test-vdc",
		CRNamespace:       "ovim-system",
		WorkloadNamespace: "test-vdc-workload",
		CPUQuota:          4,
		MemoryQuota:       8,
		StorageQuota:      100,
		NetworkPolicy:     "default",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	err = storage.CreateVDC(testVDC)
	require.NoError(t, err)

	testVM := &models.VirtualMachine{
		ID:         fmt.Sprintf("test-vm-%d", time.Now().UnixNano()),
		Name:       fmt.Sprintf("test-vm-%d", time.Now().UnixNano()),
		OrgID:      "test-org",
		VDCID:      stringPtr("test-vdc"),
		TemplateID: "test-template",
		OwnerID:    "test-user",
		Status:     "running",
		CPU:        2,
		Memory:     "4Gi",
		DiskSize:   "20Gi",
		IPAddress:  "192.168.1.100",
		Metadata:   models.StringMap{"env": "test"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	t.Run("CreateVM", func(t *testing.T) {
		err := storage.CreateVM(testVM)
		assert.NoError(t, err)

		// Test duplicate creation
		err = storage.CreateVM(testVM)
		assert.Error(t, err)
	})

	t.Run("GetVM", func(t *testing.T) {
		vm, err := storage.GetVM(testVM.ID)
		assert.NoError(t, err)
		assert.Equal(t, testVM.ID, vm.ID)
		assert.Equal(t, testVM.Name, vm.Name)
		assert.Equal(t, testVM.Status, vm.Status)

		// Test non-existent VM
		_, err = storage.GetVM("non-existent")
		assert.Error(t, err)
	})

	t.Run("ListVMs", func(t *testing.T) {
		vms, err := storage.ListVMs(testVM.OrgID)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(vms), 1)

		// Find our test VM
		found := false
		for _, vm := range vms {
			if vm.ID == testVM.ID {
				found = true
				break
			}
		}
		assert.True(t, found)

		// Test with non-existent org
		vms, err = storage.ListVMs("non-existent-org")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(vms))
	})

	t.Run("UpdateVM", func(t *testing.T) {
		updatedVM := *testVM
		updatedVM.Status = "stopped"
		updatedVM.IPAddress = "192.168.1.101"

		err := storage.UpdateVM(&updatedVM)
		assert.NoError(t, err)

		// Verify update
		vm, err := storage.GetVM(testVM.ID)
		assert.NoError(t, err)
		assert.Equal(t, "stopped", vm.Status)
		assert.Equal(t, "192.168.1.101", vm.IPAddress)

		// Test update non-existent VM
		nonExistentVM := &models.VirtualMachine{ID: "non-existent"}
		err = storage.UpdateVM(nonExistentVM)
		assert.Error(t, err)
	})

	t.Run("DeleteVM", func(t *testing.T) {
		err := storage.DeleteVM(testVM.ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.GetVM(testVM.ID)
		assert.Error(t, err)

		// Test delete non-existent VM
		err = storage.DeleteVM("non-existent")
		assert.Error(t, err)
	})
}

func TestPostgresStorage_OrganizationCatalogSourceOperations(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	testCatalogSource := &models.OrganizationCatalogSource{
		ID:              fmt.Sprintf("test-catalog-source-%d", time.Now().UnixNano()),
		OrgID:           "test-org",
		SourceType:      "global",
		SourceName:      fmt.Sprintf("test-source-%d", time.Now().UnixNano()),
		SourceNamespace: "openshift-marketplace",
		Enabled:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	t.Run("CreateOrganizationCatalogSource", func(t *testing.T) {
		err := storage.CreateOrganizationCatalogSource(testCatalogSource)
		assert.NoError(t, err)

		// Test duplicate creation
		err = storage.CreateOrganizationCatalogSource(testCatalogSource)
		assert.Error(t, err)
	})

	t.Run("GetOrganizationCatalogSource", func(t *testing.T) {
		source, err := storage.GetOrganizationCatalogSource(testCatalogSource.ID)
		assert.NoError(t, err)
		assert.Equal(t, testCatalogSource.ID, source.ID)
		assert.Equal(t, testCatalogSource.SourceType, source.SourceType)

		// Test non-existent source
		_, err = storage.GetOrganizationCatalogSource("non-existent")
		assert.Error(t, err)
	})

	t.Run("ListOrganizationCatalogSources", func(t *testing.T) {
		sources, err := storage.ListOrganizationCatalogSources(testCatalogSource.OrgID)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(sources), 1)

		// Test with non-existent org
		sources, err = storage.ListOrganizationCatalogSources("non-existent-org")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(sources))
	})

	t.Run("UpdateOrganizationCatalogSource", func(t *testing.T) {
		updatedSource := *testCatalogSource
		updatedSource.Enabled = false
		updatedSource.SourceName = fmt.Sprintf("updated-source-%d", time.Now().UnixNano())

		err := storage.UpdateOrganizationCatalogSource(&updatedSource)
		assert.NoError(t, err)

		// Verify update
		source, err := storage.GetOrganizationCatalogSource(testCatalogSource.ID)
		assert.NoError(t, err)
		assert.False(t, source.Enabled)
		assert.Equal(t, updatedSource.SourceName, source.SourceName)

		// Test update non-existent source
		nonExistentSource := &models.OrganizationCatalogSource{ID: "non-existent"}
		err = storage.UpdateOrganizationCatalogSource(nonExistentSource)
		assert.Error(t, err)
	})

	t.Run("DeleteOrganizationCatalogSource", func(t *testing.T) {
		err := storage.DeleteOrganizationCatalogSource(testCatalogSource.ID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = storage.GetOrganizationCatalogSource(testCatalogSource.ID)
		assert.Error(t, err)

		// Test delete non-existent source
		err = storage.DeleteOrganizationCatalogSource("non-existent")
		assert.Error(t, err)
	})
}

func TestPostgresStorage_TransactionHandling(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	// Test creating related entities in sequence
	org := &models.Organization{
		ID:          fmt.Sprintf("test-tx-org-%d", time.Now().UnixNano()),
		Name:        fmt.Sprintf("test-tx-org-%d", time.Now().UnixNano()),
		Description: "Transaction Test Org",
		IsEnabled:   true,
		DisplayName: stringPtr("Transaction Test Org"),
		CRName:      fmt.Sprintf("test-tx-org-%d", time.Now().UnixNano()),
		CRNamespace: "ovim-system",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	vdc := &models.VirtualDataCenter{
		ID:                fmt.Sprintf("test-tx-vdc-%d", time.Now().UnixNano()),
		Name:              fmt.Sprintf("test-tx-vdc-%d", time.Now().UnixNano()),
		Description:       "Transaction Test VDC",
		OrgID:             org.ID,
		CRName:            fmt.Sprintf("test-tx-vdc-%d", time.Now().UnixNano()),
		CRNamespace:       "ovim-system",
		WorkloadNamespace: fmt.Sprintf("test-tx-vdc-workload-%d", time.Now().UnixNano()),
		CPUQuota:          4,
		MemoryQuota:       8,
		StorageQuota:      100,
		NetworkPolicy:     "default",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	user := &models.User{
		ID:        fmt.Sprintf("test-tx-user-%d", time.Now().UnixNano()),
		Username:  fmt.Sprintf("test-tx-user-%d", time.Now().UnixNano()),
		Email:     fmt.Sprintf("test-tx-%d@example.com", time.Now().UnixNano()),
		Role:      "org_admin",
		OrgID:     &org.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create entities in order
	err := storage.CreateOrganization(org)
	require.NoError(t, err)

	err = storage.CreateVDC(vdc)
	require.NoError(t, err)

	err = storage.CreateUser(user)
	require.NoError(t, err)

	// Verify relationships
	retrievedVDCs, err := storage.ListVDCs(org.ID)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(retrievedVDCs), 1)

	retrievedUsers, err := storage.ListUsersByOrg(org.ID)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(retrievedUsers), 1)

	// Cleanup
	err = storage.DeleteUser(user.ID)
	assert.NoError(t, err)

	err = storage.DeleteVDC(vdc.ID)
	assert.NoError(t, err)

	err = storage.DeleteOrganization(org.ID)
	assert.NoError(t, err)
}

func TestPostgresStorage_ErrorHandling(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	t.Run("CreateUserWithNilInput", func(t *testing.T) {
		err := storage.CreateUser(nil)
		assert.Error(t, err)
	})

	t.Run("CreateUserWithEmptyID", func(t *testing.T) {
		user := &models.User{
			Username: "testuser",
			Email:    "test@example.com",
		}
		err := storage.CreateUser(user)
		assert.Error(t, err)
	})

	t.Run("GetUserWithEmptyID", func(t *testing.T) {
		_, err := storage.GetUserByID("")
		assert.Error(t, err)
	})

	t.Run("UpdateNonExistentUser", func(t *testing.T) {
		user := &models.User{
			ID:       "non-existent",
			Username: "testuser",
			Email:    "test@example.com",
		}
		err := storage.UpdateUser(user)
		assert.Error(t, err)
	})

	t.Run("DeleteNonExistentUser", func(t *testing.T) {
		err := storage.DeleteUser("non-existent")
		assert.Error(t, err)
	})
}

func TestPostgresStorage_ConnectionPooling(t *testing.T) {
	storage := setupTestPostgresStorage(t)
	defer storage.Close()

	// Test multiple concurrent operations
	const numOperations = 50
	done := make(chan bool, numOperations)

	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Perform a simple read operation
			_, err := storage.ListOrganizations()
			assert.NoError(t, err)

			// Perform ping
			err = storage.Ping()
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numOperations; i++ {
		<-done
	}
}
