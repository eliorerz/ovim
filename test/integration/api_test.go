package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/api"
	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	ctrlFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type IntegrationTestSuite struct {
	server     *api.Server
	storage    storage.Storage
	httpServer *httptest.Server
	adminToken string
	orgToken   string
	userToken  string
}

func setupTestSuite(t *testing.T) *IntegrationTestSuite {
	storage, err := storage.NewMemoryStorageForTest()
	require.NoError(t, err)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        "8080",
			Environment: "test",
		},
		Auth: config.AuthConfig{
			JWTSecret: "test-secret-key-for-integration-tests",
		},
		Logging: config.LoggingConfig{
			Level: "info",
		},
	}

	// Create mock KubeVirt provisioner for testing
	provisioner := kubevirt.NewMockClient()

	// Create fake clients for testing
	k8sClient := ctrlFake.NewClientBuilder().Build()
	k8sClientset := fake.NewSimpleClientset()
	recorder := record.NewFakeRecorder(100)

	server := api.NewServer(cfg, storage, provisioner, k8sClient, k8sClientset, recorder)

	httpServer := httptest.NewServer(server.Handler())

	suite := &IntegrationTestSuite{
		server:     server,
		storage:    storage,
		httpServer: httpServer,
	}

	// Seed required test users
	suite.seedTestUsers(t)
	suite.authenticateUsers(t)
	return suite
}

func (suite *IntegrationTestSuite) tearDown() {
	suite.httpServer.Close()
	suite.storage.Close()
}

func (suite *IntegrationTestSuite) seedTestUsers(t *testing.T) {
	adminHash, err := auth.HashPassword("adminpassword")
	require.NoError(t, err)

	userHash, err := auth.HashPassword("userpassword")
	require.NoError(t, err)

	now := time.Now()

	// Create a test organization for org users
	testOrg := &models.Organization{
		ID:          "org-test",
		Name:        "Test Organization",
		Description: "Test organization for development and testing",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = suite.storage.CreateOrganization(testOrg)
	require.NoError(t, err)

	// Seed test users
	users := []*models.User{
		{
			ID:           "user-admin",
			Username:     "admin",
			Email:        "admin@ovim.local",
			PasswordHash: adminHash,
			Role:         models.RoleSystemAdmin,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "user-orgadmin",
			Username:     "orgadmin",
			Email:        "orgadmin@ovim.local",
			PasswordHash: adminHash,
			Role:         models.RoleOrgAdmin,
			OrgID:        &testOrg.ID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "user-user",
			Username:     "user",
			Email:        "user@ovim.local",
			PasswordHash: userHash,
			Role:         models.RoleOrgUser,
			OrgID:        &testOrg.ID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	for _, user := range users {
		err := suite.storage.CreateUser(user)
		require.NoError(t, err)
	}
}

func (suite *IntegrationTestSuite) authenticateUsers(t *testing.T) {
	suite.adminToken = suite.loginUser(t, "admin", "adminpassword")
	suite.orgToken = suite.loginUser(t, "orgadmin", "adminpassword")
	suite.userToken = suite.loginUser(t, "user", "userpassword")
}

func (suite *IntegrationTestSuite) loginUser(t *testing.T, username, password string) string {
	loginData := map[string]string{
		"username": username,
		"password": password,
	}
	jsonData, _ := json.Marshal(loginData)

	resp, err := http.Post(
		suite.httpServer.URL+"/api/v1/auth/login",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var loginResp struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	require.NoError(t, err)

	return loginResp.Token
}

func (suite *IntegrationTestSuite) makeRequest(method, path string, body interface{}, token string) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		jsonData, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(jsonData)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequest(method, suite.httpServer.URL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return http.DefaultClient.Do(req)
}

func TestAuthenticationFlow(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.tearDown()

	t.Run("SuccessfulLogin", func(t *testing.T) {
		resp, err := suite.makeRequest("POST", "/api/v1/auth/login", map[string]string{
			"username": "admin",
			"password": "adminpassword",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var loginResp struct {
			Token string      `json:"token"`
			User  models.User `json:"user"`
		}
		err = json.NewDecoder(resp.Body).Decode(&loginResp)
		require.NoError(t, err)

		assert.NotEmpty(t, loginResp.Token)
		assert.Equal(t, "admin", loginResp.User.Username)
		assert.Equal(t, models.RoleSystemAdmin, loginResp.User.Role)
	})

	t.Run("InvalidCredentials", func(t *testing.T) {
		resp, err := suite.makeRequest("POST", "/api/v1/auth/login", map[string]string{
			"username": "admin",
			"password": "wrongpassword",
		}, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("UnauthorizedAccess", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/organizations/", nil, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestOrganizationManagement(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.tearDown()

	t.Run("ListOrganizations", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/organizations/", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var orgResp struct {
			Organizations []*models.Organization `json:"organizations"`
			Total         int                    `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&orgResp)
		require.NoError(t, err)

		assert.Len(t, orgResp.Organizations, 1)
		assert.Equal(t, 1, orgResp.Total)
	})

	t.Run("GetNonExistentOrganization", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/organizations/nonexistent", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("CreateOrganization", func(t *testing.T) {
		orgData := map[string]interface{}{
			"name":        "Test Organization",
			"description": "Organization created during testing",
			"is_enabled":  true,
		}

		resp, err := suite.makeRequest("POST", "/api/v1/organizations/", orgData, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var org models.Organization
		err = json.NewDecoder(resp.Body).Decode(&org)
		require.NoError(t, err)

		assert.Equal(t, "Test Organization", org.Name)
		assert.Equal(t, "Organization created during testing", org.Description)
		assert.Equal(t, true, org.IsEnabled)
		assert.Equal(t, "test-organization", org.ID) // Should be sanitized
		assert.Equal(t, "test-organization", org.Namespace)
		assert.NotEmpty(t, org.CreatedAt)
		assert.NotEmpty(t, org.UpdatedAt)
	})

	t.Run("UpdateOrganizationStatus", func(t *testing.T) {
		// First create an organization
		orgData := map[string]interface{}{
			"name":        "Update Test Org",
			"description": "Organization for update testing",
			"is_enabled":  true,
		}

		createResp, err := suite.makeRequest("POST", "/api/v1/organizations/", orgData, suite.adminToken)
		require.NoError(t, err)
		defer createResp.Body.Close()

		var createdOrg models.Organization
		err = json.NewDecoder(createResp.Body).Decode(&createdOrg)
		require.NoError(t, err)

		// Update the organization status
		updateData := map[string]interface{}{
			"name":        "Update Test Org",
			"description": "Organization for update testing",
			"is_enabled":  false, // Toggle the status
		}

		updateResp, err := suite.makeRequest("PUT", "/api/v1/organizations/"+createdOrg.ID, updateData, suite.adminToken)
		require.NoError(t, err)
		defer updateResp.Body.Close()

		assert.Equal(t, http.StatusOK, updateResp.StatusCode)

		var updatedOrg models.Organization
		err = json.NewDecoder(updateResp.Body).Decode(&updatedOrg)
		require.NoError(t, err)

		assert.Equal(t, createdOrg.ID, updatedOrg.ID)
		assert.Equal(t, "Update Test Org", updatedOrg.Name)
		assert.Equal(t, false, updatedOrg.IsEnabled)                     // Should be disabled now
		assert.True(t, updatedOrg.UpdatedAt.After(updatedOrg.CreatedAt)) // UpdatedAt should be newer
	})

	t.Run("ToggleOrganizationStatusMultipleTimes", func(t *testing.T) {
		// Create an organization
		orgData := map[string]interface{}{
			"name":        "Toggle Test Org",
			"description": "Organization for toggle testing",
			"is_enabled":  true,
		}

		createResp, err := suite.makeRequest("POST", "/api/v1/organizations/", orgData, suite.adminToken)
		require.NoError(t, err)
		defer createResp.Body.Close()

		var org models.Organization
		err = json.NewDecoder(createResp.Body).Decode(&org)
		require.NoError(t, err)
		orgID := org.ID

		// Toggle to disabled
		updateData := map[string]interface{}{
			"name":        "Toggle Test Org",
			"description": "Organization for toggle testing",
			"is_enabled":  false,
		}

		resp1, err := suite.makeRequest("PUT", "/api/v1/organizations/"+orgID, updateData, suite.adminToken)
		require.NoError(t, err)
		defer resp1.Body.Close()
		assert.Equal(t, http.StatusOK, resp1.StatusCode)

		// Toggle back to enabled
		updateData["is_enabled"] = true
		resp2, err := suite.makeRequest("PUT", "/api/v1/organizations/"+orgID, updateData, suite.adminToken)
		require.NoError(t, err)
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusOK, resp2.StatusCode)

		var finalOrg models.Organization
		err = json.NewDecoder(resp2.Body).Decode(&finalOrg)
		require.NoError(t, err)

		assert.Equal(t, true, finalOrg.IsEnabled)
	})

	t.Run("RegularUserCannotAccessOrganizations", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/organizations/", nil, suite.userToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestVDCManagement(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.tearDown()

	t.Run("ListVDCs", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/vdcs/", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var vdcResp struct {
			VDCs  []*models.VirtualDataCenter `json:"vdcs"`
			Total int                         `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&vdcResp)
		require.NoError(t, err)

		assert.Len(t, vdcResp.VDCs, 0)
		assert.Equal(t, 0, vdcResp.Total)
	})

	t.Run("GetNonExistentVDC", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/vdcs/nonexistent", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestVMCatalog(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.tearDown()

	t.Run("ListTemplates", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/catalog/templates", nil, suite.userToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var templateResp struct {
			Templates []*models.Template `json:"templates"`
			Total     int                `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&templateResp)
		require.NoError(t, err)

		assert.Len(t, templateResp.Templates, 0)
		assert.Equal(t, 0, templateResp.Total)
	})

	t.Run("GetNonExistentTemplate", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/catalog/templates/nonexistent", nil, suite.userToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestVMLifecycle(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.tearDown()

	t.Run("ListVMs", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/vms/", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var vmResp struct {
			VMs   []*models.VirtualMachine `json:"vms"`
			Total int                      `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&vmResp)
		require.NoError(t, err)

		// Should start with no VMs
		assert.Len(t, vmResp.VMs, 0)
		assert.Equal(t, 0, vmResp.Total)
	})

	t.Run("GetNonExistentVM", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/vms/nonexistent", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestRoleBasedAccessControl(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.tearDown()

	t.Run("SystemAdminAccess", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/organizations/", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = suite.makeRequest("GET", "/api/v1/vdcs/", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("OrgAdminAccess", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/catalog/templates", nil, suite.orgToken)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = suite.makeRequest("GET", "/api/v1/vms/", nil, suite.orgToken)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("RegularUserRestrictedAccess", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/organizations/", nil, suite.userToken)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		resp, err = suite.makeRequest("GET", "/api/v1/catalog/templates", nil, suite.userToken)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = suite.makeRequest("GET", "/api/v1/vms/", nil, suite.userToken)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHealthEndpoint(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.tearDown()

	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/health", nil, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var healthResp struct {
			Service string `json:"service"`
			Status  string `json:"status"`
			Version string `json:"version"`
		}
		err = json.NewDecoder(resp.Body).Decode(&healthResp)
		require.NoError(t, err)

		assert.Equal(t, "OVIM Backend", healthResp.Service)
		assert.Equal(t, "healthy", healthResp.Status)
		assert.NotEmpty(t, healthResp.Version)
	})
}
