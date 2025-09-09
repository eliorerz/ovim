package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eliorerz/ovim-updated/pkg/api"
	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	storage, err := storage.NewMemoryStorage()
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

	server := api.NewServer(cfg, storage, provisioner)

	httpServer := httptest.NewServer(server.Handler())

	suite := &IntegrationTestSuite{
		server:     server,
		storage:    storage,
		httpServer: httpServer,
	}

	suite.authenticateUsers(t)
	return suite
}

func (suite *IntegrationTestSuite) tearDown() {
	suite.httpServer.Close()
	suite.storage.Close()
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

		assert.Len(t, orgResp.Organizations, 2)
		assert.Equal(t, 2, orgResp.Total)

		orgNames := make([]string, len(orgResp.Organizations))
		for i, org := range orgResp.Organizations {
			orgNames[i] = org.Name
		}
		assert.Contains(t, orgNames, "Acme Corporation")
		assert.Contains(t, orgNames, "Development Team")
	})

	t.Run("GetSpecificOrganization", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/organizations/org-acme", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var org models.Organization
		err = json.NewDecoder(resp.Body).Decode(&org)
		require.NoError(t, err)

		assert.Equal(t, "org-acme", org.ID)
		assert.Equal(t, "Acme Corporation", org.Name)
		assert.Equal(t, "acme-corp", org.Namespace)
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

		assert.Len(t, vdcResp.VDCs, 2)
		assert.Equal(t, 2, vdcResp.Total)
	})

	t.Run("GetVDCWithResourceQuotas", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/vdcs/vdc-acme-main", nil, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var vdc models.VirtualDataCenter
		err = json.NewDecoder(resp.Body).Decode(&vdc)
		require.NoError(t, err)

		assert.Equal(t, "vdc-acme-main", vdc.ID)
		assert.Equal(t, "Acme Main VDC", vdc.Name)
		assert.Equal(t, "org-acme", vdc.OrgID)
		assert.Equal(t, "20", vdc.ResourceQuotas["cpu"])
		assert.Equal(t, "64Gi", vdc.ResourceQuotas["memory"])
		assert.Equal(t, "1Ti", vdc.ResourceQuotas["storage"])
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

		assert.Len(t, templateResp.Templates, 3)
		assert.Equal(t, 3, templateResp.Total)

		templateNames := make([]string, len(templateResp.Templates))
		for i, tmpl := range templateResp.Templates {
			templateNames[i] = tmpl.Name
		}
		assert.Contains(t, templateNames, "Red Hat Enterprise Linux 9.2")
		assert.Contains(t, templateNames, "Ubuntu Server 22.04 LTS")
		assert.Contains(t, templateNames, "CentOS Stream 9")
	})

	t.Run("GetSpecificTemplate", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/catalog/templates/tmpl-ubuntu22", nil, suite.userToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var template models.Template
		err = json.NewDecoder(resp.Body).Decode(&template)
		require.NoError(t, err)

		assert.Equal(t, "tmpl-ubuntu22", template.ID)
		assert.Equal(t, "Ubuntu Server 22.04 LTS", template.Name)
		assert.Equal(t, "Linux", template.OSType)
		assert.Equal(t, 2, template.CPU)
		assert.Equal(t, "2Gi", template.Memory)
	})
}

func TestVMLifecycle(t *testing.T) {
	suite := setupTestSuite(t)
	defer suite.tearDown()

	t.Run("CreateVM", func(t *testing.T) {
		vmData := map[string]interface{}{
			"name":        "test-integration-vm",
			"description": "VM created during integration testing",
			"vdc_id":      "vdc-acme-main",
			"template_id": "tmpl-ubuntu22",
			"metadata": map[string]string{
				"environment": "test",
				"purpose":     "integration",
			},
		}

		resp, err := suite.makeRequest("POST", "/api/v1/vms/", vmData, suite.orgToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var vm models.VirtualMachine
		err = json.NewDecoder(resp.Body).Decode(&vm)
		require.NoError(t, err)

		assert.Equal(t, "test-integration-vm", vm.Name)
		assert.Equal(t, "org-acme", vm.OrgID)
		assert.Equal(t, "vdc-acme-main", vm.VDCID)
		assert.Equal(t, "tmpl-ubuntu22", vm.TemplateID)
		assert.Equal(t, models.VMStatusPending, vm.Status)
		assert.Equal(t, 2, vm.CPU)
		assert.Equal(t, "2Gi", vm.Memory)
	})

	t.Run("SystemAdminCannotCreateVM", func(t *testing.T) {
		vmData := map[string]interface{}{
			"name":        "admin-vm",
			"vdc_id":      "vdc-acme-main",
			"template_id": "tmpl-ubuntu22",
		}

		resp, err := suite.makeRequest("POST", "/api/v1/vms/", vmData, suite.adminToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResp struct {
			Error string `json:"error"`
		}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)

		assert.Contains(t, errorResp.Error, "not associated with any organization")
	})

	t.Run("ListVMs", func(t *testing.T) {
		resp, err := suite.makeRequest("GET", "/api/v1/vms/", nil, suite.orgToken)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var vmResp struct {
			VMs   []*models.VirtualMachine `json:"vms"`
			Total int                      `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&vmResp)
		require.NoError(t, err)

		assert.Greater(t, vmResp.Total, 0)
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
