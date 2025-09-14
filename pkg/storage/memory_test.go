package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

func TestNewMemoryStorage(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	require.NotNil(t, storage)

	// Test that storage implements the interface
	var _ Storage = storage

	// Test ping
	err = storage.Ping()
	assert.NoError(t, err)

	// Test close
	err = storage.Close()
	assert.NoError(t, err)
}

func TestMemoryStorage_UserOperations(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	testUser := &models.User{
		ID:        "test-user-id",
		Username:  "testuser",
		Email:     "test@example.com",
		Role:      "org_user",
		OrgID:     stringPtr("test-org"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	t.Run("CreateUser", func(t *testing.T) {
		err := storage.CreateUser(testUser)
		assert.NoError(t, err)

		// Test duplicate creation
		err = storage.CreateUser(testUser)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
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
		assert.Contains(t, err.Error(), "not found")
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
		assert.GreaterOrEqual(t, len(users), 1) // At least our test user + seed data

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
		updatedUser.Email = "updated@example.com"
		updatedUser.UpdatedAt = time.Now()

		err := storage.UpdateUser(&updatedUser)
		assert.NoError(t, err)

		// Verify update
		user, err := storage.GetUserByID(testUser.ID)
		assert.NoError(t, err)
		assert.Equal(t, "updated@example.com", user.Email)

		// Test update non-existent user
		nonExistentUser := &models.User{ID: "non-existent"}
		err = storage.UpdateUser(nonExistentUser)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
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

func TestMemoryStorage_OrganizationOperations(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	testOrg := &models.Organization{
		ID:          "test-org-id",
		Name:        "test-org",
		Description: "Test Organization",
		Namespace:   "test-org-ns",
		IsEnabled:   true,
		DisplayName: stringPtr("Test Organization"),
		CRName:      "test-org",
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

func TestMemoryStorage_VDCOperations(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	testVDC := &models.VirtualDataCenter{
		ID:                "test-vdc-id",
		Name:              "test-vdc",
		Description:       "Test VDC",
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

func TestMemoryStorage_TemplateOperations(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	testTemplate := &models.Template{
		ID:          "test-template-id",
		Name:        "test-template",
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

func TestMemoryStorage_VMOperations(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	testVM := &models.VirtualMachine{
		ID:         "test-vm-id",
		Name:       "test-vm",
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

func TestMemoryStorage_OrganizationCatalogSourceOperations(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	testCatalogSource := &models.OrganizationCatalogSource{
		ID:              "test-catalog-source-id",
		OrgID:           "test-org",
		SourceType:      "global",
		SourceName:      "test-source",
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
		updatedSource.SourceName = "updated-source"

		err := storage.UpdateOrganizationCatalogSource(&updatedSource)
		assert.NoError(t, err)

		// Verify update
		source, err := storage.GetOrganizationCatalogSource(testCatalogSource.ID)
		assert.NoError(t, err)
		assert.False(t, source.Enabled)
		assert.Equal(t, "updated-source", source.SourceName)

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

func TestMemoryStorage_ConcurrentAccess(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	// Test concurrent read/write operations
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				// Create user
				user := &models.User{
					ID:        fmt.Sprintf("user-%d-%d", goroutineID, j),
					Username:  fmt.Sprintf("user%d%d", goroutineID, j),
					Email:     fmt.Sprintf("user%d%d@example.com", goroutineID, j),
					Role:      "org_user",
					OrgID:     stringPtr("test-org"),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				err := storage.CreateUser(user)
				assert.NoError(t, err)

				// Read user
				_, err = storage.GetUserByID(user.ID)
				assert.NoError(t, err)

				// Update user
				user.Email = fmt.Sprintf("updated%d%d@example.com", goroutineID, j)
				err = storage.UpdateUser(user)
				assert.NoError(t, err)

				// Delete user
				err = storage.DeleteUser(user.ID)
				assert.NoError(t, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestMemoryStorage_ErrorCases(t *testing.T) {
	storage, err := NewMemoryStorage()
	require.NoError(t, err)
	defer storage.Close()

	t.Run("CreateUserWithNilInput", func(t *testing.T) {
		err := storage.CreateUser(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid input")
	})

	t.Run("CreateUserWithEmptyID", func(t *testing.T) {
		user := &models.User{
			Username: "testuser",
			Email:    "test@example.com",
		}
		err := storage.CreateUser(user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid input")
	})

	t.Run("GetUserWithEmptyID", func(t *testing.T) {
		_, err := storage.GetUserByID("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource not found")
	})

	t.Run("UpdateNonExistentUser", func(t *testing.T) {
		user := &models.User{
			ID:       "non-existent",
			Username: "testuser",
			Email:    "test@example.com",
		}
		err := storage.UpdateUser(user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("DeleteNonExistentUser", func(t *testing.T) {
		err := storage.DeleteUser("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
