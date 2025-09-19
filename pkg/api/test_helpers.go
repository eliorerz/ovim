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

// Event operations
func (m *MockStorage) ListEvents(filter *models.EventFilter) (*models.EventsResponse, error) {
	args := m.Called(filter)
	return args.Get(0).(*models.EventsResponse), args.Error(1)
}

func (m *MockStorage) GetEvent(id string) (*models.Event, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Event), args.Error(1)
}

func (m *MockStorage) CreateEvent(event *models.Event) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockStorage) CreateEvents(events []*models.Event) error {
	args := m.Called(events)
	return args.Error(0)
}

func (m *MockStorage) UpdateEvent(event *models.Event) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockStorage) DeleteEvent(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) CleanupOldEvents() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

// Event category operations
func (m *MockStorage) ListEventCategories() ([]*models.EventCategory, error) {
	args := m.Called()
	return args.Get(0).([]*models.EventCategory), args.Error(1)
}

func (m *MockStorage) GetEventCategory(name string) (*models.EventCategory, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.EventCategory), args.Error(1)
}

// Event retention policy operations
func (m *MockStorage) ListEventRetentionPolicies() ([]*models.EventRetentionPolicy, error) {
	args := m.Called()
	return args.Get(0).([]*models.EventRetentionPolicy), args.Error(1)
}

func (m *MockStorage) GetEventRetentionPolicy(category, eventType string) (*models.EventRetentionPolicy, error) {
	args := m.Called(category, eventType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.EventRetentionPolicy), args.Error(1)
}

func (m *MockStorage) UpdateEventRetentionPolicy(policy *models.EventRetentionPolicy) error {
	args := m.Called(policy)
	return args.Error(0)
}

// Zone operations
func (m *MockStorage) ListZones() ([]*models.Zone, error) {
	args := m.Called()
	return args.Get(0).([]*models.Zone), args.Error(1)
}

func (m *MockStorage) GetZone(id string) (*models.Zone, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Zone), args.Error(1)
}

func (m *MockStorage) CreateZone(zone *models.Zone) error {
	args := m.Called(zone)
	return args.Error(0)
}

func (m *MockStorage) UpdateZone(zone *models.Zone) error {
	args := m.Called(zone)
	return args.Error(0)
}

func (m *MockStorage) DeleteZone(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStorage) GetZoneUtilization() ([]*models.ZoneUtilization, error) {
	args := m.Called()
	return args.Get(0).([]*models.ZoneUtilization), args.Error(1)
}

// Organization Zone Quota operations
func (m *MockStorage) ListOrganizationZoneQuotas(orgID string) ([]*models.OrganizationZoneQuota, error) {
	args := m.Called(orgID)
	return args.Get(0).([]*models.OrganizationZoneQuota), args.Error(1)
}

func (m *MockStorage) GetOrganizationZoneQuota(orgID, zoneID string) (*models.OrganizationZoneQuota, error) {
	args := m.Called(orgID, zoneID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrganizationZoneQuota), args.Error(1)
}

func (m *MockStorage) CreateOrganizationZoneQuota(quota *models.OrganizationZoneQuota) error {
	args := m.Called(quota)
	return args.Error(0)
}

func (m *MockStorage) UpdateOrganizationZoneQuota(quota *models.OrganizationZoneQuota) error {
	args := m.Called(quota)
	return args.Error(0)
}

func (m *MockStorage) DeleteOrganizationZoneQuota(orgID, zoneID string) error {
	args := m.Called(orgID, zoneID)
	return args.Error(0)
}

func (m *MockStorage) GetOrganizationZoneAccess(orgID string) ([]*models.OrganizationZoneAccess, error) {
	args := m.Called(orgID)
	return args.Get(0).([]*models.OrganizationZoneAccess), args.Error(1)
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
