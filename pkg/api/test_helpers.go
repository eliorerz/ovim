package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
)

// MockK8sClient is a mock implementation of the controller-runtime client.Client interface
type MockK8sClient struct {
	mock.Mock
}

func (m *MockK8sClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockK8sClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockK8sClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockK8sClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (m *MockK8sClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj, opts)
	return args.Error(0)
}

func (m *MockK8sClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

func (m *MockK8sClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockK8sClient) Status() client.StatusWriter {
	args := m.Called()
	return args.Get(0).(client.StatusWriter)
}

func (m *MockK8sClient) SubResource(subResource string) client.SubResourceClient {
	args := m.Called(subResource)
	return args.Get(0).(client.SubResourceClient)
}

func (m *MockK8sClient) Scheme() *runtime.Scheme {
	args := m.Called()
	return args.Get(0).(*runtime.Scheme)
}

func (m *MockK8sClient) RESTMapper() meta.RESTMapper {
	args := m.Called()
	return args.Get(0).(meta.RESTMapper)
}

func (m *MockK8sClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	args := m.Called(obj)
	return args.Get(0).(schema.GroupVersionKind), args.Error(1)
}

func (m *MockK8sClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	args := m.Called(obj)
	return args.Bool(0), args.Error(1)
}

// MockStorage is a mock implementation of the storage.Storage interface
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) ListUsers() ([]*models.User, error) {
	args := m.Called()
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *MockStorage) ListUsersByOrg(orgID string) ([]*models.User, error) {
	args := m.Called(orgID)
	return args.Get(0).([]*models.User), args.Error(1)
}

func (m *MockStorage) GetUserByUsername(username string) (*models.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockStorage) GetUserByID(id string) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockStorage) CreateUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockStorage) UpdateUser(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockStorage) DeleteUser(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListOrganizations() ([]*models.Organization, error) {
	args := m.Called()
	return args.Get(0).([]*models.Organization), args.Error(1)
}

func (m *MockStorage) GetOrganization(id string) (*models.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *MockStorage) CreateOrganization(org *models.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockStorage) UpdateOrganization(org *models.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockStorage) DeleteOrganization(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListVDCs(orgID string) ([]*models.VirtualDataCenter, error) {
	args := m.Called(orgID)
	return args.Get(0).([]*models.VirtualDataCenter), args.Error(1)
}

func (m *MockStorage) GetVDC(id string) (*models.VirtualDataCenter, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.VirtualDataCenter), args.Error(1)
}

func (m *MockStorage) CreateVDC(vdc *models.VirtualDataCenter) error {
	args := m.Called(vdc)
	return args.Error(0)
}

func (m *MockStorage) UpdateVDC(vdc *models.VirtualDataCenter) error {
	args := m.Called(vdc)
	return args.Error(0)
}

func (m *MockStorage) DeleteVDC(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListTemplates() ([]*models.Template, error) {
	args := m.Called()
	return args.Get(0).([]*models.Template), args.Error(1)
}

func (m *MockStorage) ListTemplatesByOrg(orgID string) ([]*models.Template, error) {
	args := m.Called(orgID)
	return args.Get(0).([]*models.Template), args.Error(1)
}

func (m *MockStorage) GetTemplate(id string) (*models.Template, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Template), args.Error(1)
}

func (m *MockStorage) CreateTemplate(template *models.Template) error {
	args := m.Called(template)
	return args.Error(0)
}

func (m *MockStorage) UpdateTemplate(template *models.Template) error {
	args := m.Called(template)
	return args.Error(0)
}

func (m *MockStorage) DeleteTemplate(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListVMs(orgID string) ([]*models.VirtualMachine, error) {
	args := m.Called(orgID)
	return args.Get(0).([]*models.VirtualMachine), args.Error(1)
}

func (m *MockStorage) GetVM(id string) (*models.VirtualMachine, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.VirtualMachine), args.Error(1)
}

func (m *MockStorage) CreateVM(vm *models.VirtualMachine) error {
	args := m.Called(vm)
	return args.Error(0)
}

func (m *MockStorage) UpdateVM(vm *models.VirtualMachine) error {
	args := m.Called(vm)
	return args.Error(0)
}

func (m *MockStorage) DeleteVM(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) ListOrganizationCatalogSources(orgID string) ([]*models.OrganizationCatalogSource, error) {
	args := m.Called(orgID)
	return args.Get(0).([]*models.OrganizationCatalogSource), args.Error(1)
}

func (m *MockStorage) GetOrganizationCatalogSource(id string) (*models.OrganizationCatalogSource, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrganizationCatalogSource), args.Error(1)
}

func (m *MockStorage) CreateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error {
	args := m.Called(source)
	return args.Error(0)
}

func (m *MockStorage) UpdateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error {
	args := m.Called(source)
	return args.Error(0)
}

func (m *MockStorage) DeleteOrganizationCatalogSource(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) Ping() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

// setupGinContext creates a Gin context for testing
func setupGinContext(method, url string, body interface{}, userID, username, role, orgID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != nil && body != "" {
		jsonBody, _ := json.Marshal(body)
		req = httptest.NewRequest(method, url, bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}

	c.Request = req

	// Set user context for auth
	c.Set(auth.ContextKeyUserID, userID)
	c.Set(auth.ContextKeyUsername, username)
	c.Set(auth.ContextKeyRole, role)
	c.Set(auth.ContextKeyOrgID, orgID)

	return c, w
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}

// Helper functions for tests
func intPtr(i int) *int {
	return &i
}
