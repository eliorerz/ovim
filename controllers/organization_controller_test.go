package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// MockStorage implements storage.Storage interface for testing
type MockStorage struct {
	organizations map[string]*models.Organization
	vdcs          map[string]*models.VirtualDataCenter
	catalogs      map[string]*models.Catalog
	shouldError   bool
	errorMessage  string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		organizations: make(map[string]*models.Organization),
		vdcs:          make(map[string]*models.VirtualDataCenter),
		catalogs:      make(map[string]*models.Catalog),
	}
}

func (m *MockStorage) SetError(should bool, message string) {
	m.shouldError = should
	m.errorMessage = message
}

// Organization methods
func (m *MockStorage) CreateOrganization(org *models.Organization) error {
	if m.shouldError {
		return fmt.Errorf("create organization failed: %s", m.errorMessage)
	}
	m.organizations[org.ID] = org
	return nil
}

func (m *MockStorage) GetOrganization(id string) (*models.Organization, error) {
	if m.shouldError {
		return nil, fmt.Errorf("get organization failed: %s", m.errorMessage)
	}
	if org, exists := m.organizations[id]; exists {
		return org, nil
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) UpdateOrganization(org *models.Organization) error {
	if m.shouldError {
		return fmt.Errorf("update organization failed: %s", m.errorMessage)
	}
	m.organizations[org.ID] = org
	return nil
}

func (m *MockStorage) DeleteOrganization(id string) error {
	if m.shouldError {
		return fmt.Errorf("delete organization failed: %s", m.errorMessage)
	}

	// Delete associated VDCs first
	var vdcsToDelete []string
	for vdcID, vdc := range m.vdcs {
		if vdc.OrgID == id {
			vdcsToDelete = append(vdcsToDelete, vdcID)
		}
	}
	for _, vdcID := range vdcsToDelete {
		delete(m.vdcs, vdcID)
	}

	delete(m.organizations, id)
	return nil
}

func (m *MockStorage) ListOrganizations() ([]*models.Organization, error) {
	if m.shouldError {
		return nil, fmt.Errorf("list organizations failed: %s", m.errorMessage)
	}
	var result []*models.Organization
	for _, org := range m.organizations {
		result = append(result, org)
	}
	return result, nil
}

// VDC methods (required by interface)
func (m *MockStorage) CreateVDC(vdc *models.VirtualDataCenter) error {
	if m.shouldError {
		return fmt.Errorf("create vdc failed: %s", m.errorMessage)
	}
	m.vdcs[vdc.ID] = vdc
	return nil
}

func (m *MockStorage) GetVDC(id string) (*models.VirtualDataCenter, error) {
	if m.shouldError {
		return nil, fmt.Errorf("get vdc failed: %s", m.errorMessage)
	}
	if vdc, exists := m.vdcs[id]; exists {
		return vdc, nil
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) UpdateVDC(vdc *models.VirtualDataCenter) error {
	if m.shouldError {
		return fmt.Errorf("update vdc failed: %s", m.errorMessage)
	}
	m.vdcs[vdc.ID] = vdc
	return nil
}

func (m *MockStorage) DeleteVDC(id string) error {
	if m.shouldError {
		return fmt.Errorf("delete vdc failed: %s", m.errorMessage)
	}
	delete(m.vdcs, id)
	return nil
}

func (m *MockStorage) ListVDCs(orgFilter string) ([]*models.VirtualDataCenter, error) {
	if m.shouldError {
		return nil, fmt.Errorf("list vdcs failed: %s", m.errorMessage)
	}
	var result []*models.VirtualDataCenter
	for _, vdc := range m.vdcs {
		if orgFilter == "" || vdc.OrgID == orgFilter {
			result = append(result, vdc)
		}
	}
	return result, nil
}

// Other required interface methods (minimal implementations for testing)
func (m *MockStorage) CreateUser(user *models.User) error      { return nil }
func (m *MockStorage) GetUser(id string) (*models.User, error) { return nil, storage.ErrNotFound }
func (m *MockStorage) GetUserByUsername(username string) (*models.User, error) {
	return nil, storage.ErrNotFound
}
func (m *MockStorage) GetUserByID(id string) (*models.User, error) { return nil, storage.ErrNotFound }
func (m *MockStorage) ListUsersByOrg(orgID string) ([]*models.User, error) {
	return []*models.User{}, nil
}
func (m *MockStorage) UpdateUser(user *models.User) error             { return nil }
func (m *MockStorage) DeleteUser(id string) error                     { return nil }
func (m *MockStorage) ListUsers() ([]*models.User, error)             { return []*models.User{}, nil }
func (m *MockStorage) CreateTemplate(template *models.Template) error { return nil }
func (m *MockStorage) GetTemplate(id string) (*models.Template, error) {
	return nil, storage.ErrNotFound
}
func (m *MockStorage) UpdateTemplate(template *models.Template) error { return nil }
func (m *MockStorage) DeleteTemplate(id string) error                 { return nil }
func (m *MockStorage) ListTemplates() ([]*models.Template, error)     { return []*models.Template{}, nil }
func (m *MockStorage) ListTemplatesByOrg(orgID string) ([]*models.Template, error) {
	return []*models.Template{}, nil
}
func (m *MockStorage) CreateVM(vm *models.VirtualMachine) error { return nil }
func (m *MockStorage) GetVM(id string) (*models.VirtualMachine, error) {
	return nil, storage.ErrNotFound
}
func (m *MockStorage) UpdateVM(vm *models.VirtualMachine) error { return nil }
func (m *MockStorage) DeleteVM(id string) error                 { return nil }
func (m *MockStorage) ListVMs(orgFilter string) ([]*models.VirtualMachine, error) {
	return []*models.VirtualMachine{}, nil
}
func (m *MockStorage) CreateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error {
	return nil
}
func (m *MockStorage) GetOrganizationCatalogSource(id string) (*models.OrganizationCatalogSource, error) {
	return nil, storage.ErrNotFound
}
func (m *MockStorage) UpdateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error {
	return nil
}
func (m *MockStorage) DeleteOrganizationCatalogSource(id string) error { return nil }
func (m *MockStorage) ListOrganizationCatalogSources(orgID string) ([]*models.OrganizationCatalogSource, error) {
	return []*models.OrganizationCatalogSource{}, nil
}
func (m *MockStorage) Ping() error  { return nil }
func (m *MockStorage) Close() error { return nil }

func setupOrganizationTest() (*OrganizationReconciler, client.Client, *MockStorage) {
	// Create scheme with our CRD types
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = ovimv1.AddToScheme(s)

	// Create fake client with status subresource support
	fakeClient := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&ovimv1.Organization{}, &ovimv1.VirtualDataCenter{}).Build()

	// Create mock storage
	mockStorage := NewMockStorage()

	// Create reconciler
	reconciler := &OrganizationReconciler{
		Client:  fakeClient,
		Scheme:  s,
		Storage: mockStorage,
	}

	return reconciler, fakeClient, mockStorage
}

func TestOrganizationReconciler_Reconcile_CreateOrganization(t *testing.T) {
	reconciler, client, mockStorage := setupOrganizationTest()
	ctx := context.Background()

	// Create organization CRD
	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin1", "admin2"},
			IsEnabled:   true,
		},
	}

	// Create the organization in the fake client
	err := client.Create(ctx, org)
	require.NoError(t, err)

	// Reconcile - first pass adds finalizer
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Reconcile - second pass creates namespace and sets up RBAC
	result, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify finalizer was added
	var updatedOrg ovimv1.Organization
	err = client.Get(ctx, req.NamespacedName, &updatedOrg)
	require.NoError(t, err)
	assert.True(t, controllerutil.ContainsFinalizer(&updatedOrg, OrganizationFinalizer))

	// Verify namespace was created
	expectedNamespace := "org-test-org"
	var namespace corev1.Namespace
	err = client.Get(ctx, types.NamespacedName{Name: expectedNamespace}, &namespace)
	require.NoError(t, err)
	assert.Equal(t, expectedNamespace, namespace.Name)
	assert.Equal(t, "ovim", namespace.Labels["app.kubernetes.io/name"])
	assert.Equal(t, "organization", namespace.Labels["app.kubernetes.io/component"])
	assert.Equal(t, "test-org", namespace.Labels["ovim.io/organization-name"])

	// Verify status was updated
	assert.Equal(t, expectedNamespace, updatedOrg.Status.Namespace)
	assert.Equal(t, ovimv1.OrganizationPhaseActive, updatedOrg.Status.Phase)

	// Verify condition was set
	require.Len(t, updatedOrg.Status.Conditions, 1)
	condition := updatedOrg.Status.Conditions[0]
	assert.Equal(t, ConditionReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "OrganizationReady", condition.Reason)

	// Verify database sync was called
	assert.Len(t, mockStorage.organizations, 1)
	dbOrg := mockStorage.organizations["test-org"]
	require.NotNil(t, dbOrg)
	assert.Equal(t, "test-org", dbOrg.ID)
	assert.Equal(t, "Test Organization", dbOrg.Name)
}

func TestOrganizationReconciler_Reconcile_NotFound(t *testing.T) {
	reconciler, _, _ := setupOrganizationTest()
	ctx := context.Background()

	// Try to reconcile non-existent organization
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "non-existent",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestOrganizationReconciler_Reconcile_NamespaceCreationFails(t *testing.T) {
	reconciler, client, _ := setupOrganizationTest()
	ctx := context.Background()

	// Create organization CRD
	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin1"},
			IsEnabled:   true,
		},
	}

	// Create the organization with finalizer already present
	controllerutil.AddFinalizer(org, OrganizationFinalizer)
	err := client.Create(ctx, org)
	require.NoError(t, err)

	// Create a namespace with the same name to cause conflict
	existingNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "org-test-org",
			Labels: map[string]string{
				"conflicting": "namespace",
			},
		},
	}
	err = client.Create(ctx, existingNS)
	require.NoError(t, err)

	// Reconcile should succeed (namespace already exists)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the existing namespace is preserved
	var namespace corev1.Namespace
	err = client.Get(ctx, types.NamespacedName{Name: "org-test-org"}, &namespace)
	require.NoError(t, err)
	assert.Equal(t, "namespace", namespace.Labels["conflicting"])
}

func TestOrganizationReconciler_Reconcile_DatabaseSyncFails(t *testing.T) {
	reconciler, client, mockStorage := setupOrganizationTest()
	ctx := context.Background()

	// Configure mock to fail database operations
	mockStorage.SetError(true, "database connection failed")

	// Create organization CRD
	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin1"},
			IsEnabled:   true,
		},
	}

	// Create the organization with finalizer already present
	controllerutil.AddFinalizer(org, OrganizationFinalizer)
	err := client.Create(ctx, org)
	require.NoError(t, err)

	// Reconcile should still succeed (database sync is non-critical)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify namespace and RBAC were still created
	var namespace corev1.Namespace
	err = client.Get(ctx, types.NamespacedName{Name: "org-test-org"}, &namespace)
	require.NoError(t, err)

	// Verify status was still updated to active
	var updatedOrg ovimv1.Organization
	err = client.Get(ctx, req.NamespacedName, &updatedOrg)
	require.NoError(t, err)
	assert.Equal(t, ovimv1.OrganizationPhaseActive, updatedOrg.Status.Phase)
}

func TestOrganizationReconciler_HandleDeletion(t *testing.T) {
	reconciler, client, mockStorage := setupOrganizationTest()
	ctx := context.Background()

	// Create organization in mock storage
	dbOrg := &models.Organization{
		ID:          "test-org",
		Name:        "test-org",
		DisplayName: func(s string) *string { return &s }("Test Organization"),
	}
	err := mockStorage.CreateOrganization(dbOrg)
	require.NoError(t, err)

	// Create organization namespace
	orgNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "org-test-org",
			Labels: map[string]string{
				"app.kubernetes.io/name":  "ovim",
				"ovim.io/organization-id": "test-org",
			},
		},
	}
	err = client.Create(ctx, orgNamespace)
	require.NoError(t, err)

	// Create organization CRD with deletion timestamp
	now := metav1.Now()
	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-org",
			DeletionTimestamp: &now,
			Finalizers:        []string{OrganizationFinalizer},
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin1"},
			IsEnabled:   true,
		},
		Status: ovimv1.OrganizationStatus{
			Namespace: "org-test-org",
			Phase:     ovimv1.OrganizationPhaseActive,
		},
	}

	err = client.Create(ctx, org)
	require.NoError(t, err)

	// Reconcile deletion - should succeed since no VDCs exist
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify finalizer was removed
	var updatedOrg ovimv1.Organization
	err = client.Get(ctx, req.NamespacedName, &updatedOrg)
	require.NoError(t, err)
	assert.False(t, controllerutil.ContainsFinalizer(&updatedOrg, OrganizationFinalizer))

	// Verify namespace was deleted
	var namespace corev1.Namespace
	err = client.Get(ctx, types.NamespacedName{Name: "org-test-org"}, &namespace)
	assert.True(t, err != nil) // Should not be found

	// Verify organization was deleted from database
	_, err = mockStorage.GetOrganization("test-org")
	assert.Equal(t, storage.ErrNotFound, err)
}

func TestOrganizationReconciler_UpdateOrgCondition(t *testing.T) {
	reconciler, _, _ := setupOrganizationTest()

	org := &ovimv1.Organization{
		Status: ovimv1.OrganizationStatus{},
	}

	// Test adding new condition
	reconciler.updateOrgCondition(org, ConditionReady, metav1.ConditionTrue, "TestReason", "Test message")

	require.Len(t, org.Status.Conditions, 1)
	condition := org.Status.Conditions[0]
	assert.Equal(t, ConditionReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "TestReason", condition.Reason)
	assert.Equal(t, "Test message", condition.Message)

	// Test updating existing condition
	oldTime := condition.LastTransitionTime
	time.Sleep(time.Millisecond) // Ensure time difference

	reconciler.updateOrgCondition(org, ConditionReady, metav1.ConditionFalse, "UpdatedReason", "Updated message")

	require.Len(t, org.Status.Conditions, 1)
	updatedCondition := org.Status.Conditions[0]
	assert.Equal(t, ConditionReady, updatedCondition.Type)
	assert.Equal(t, metav1.ConditionFalse, updatedCondition.Status)
	assert.Equal(t, "UpdatedReason", updatedCondition.Reason)
	assert.Equal(t, "Updated message", updatedCondition.Message)
	assert.True(t, updatedCondition.LastTransitionTime.After(oldTime.Time))

	// Test adding different condition
	reconciler.updateOrgCondition(org, ConditionReadyForDeletion, metav1.ConditionTrue, "DeletionReady", "Ready for deletion")

	require.Len(t, org.Status.Conditions, 2)
}

func TestOrganizationReconciler_SetupForManager(t *testing.T) {
	reconciler, _, _ := setupOrganizationTest()

	// This test verifies that SetupWithManager can be called without error
	// In a real test environment, you would use a real manager
	// For this unit test, we just verify the method exists and has the right signature
	assert.NotNil(t, reconciler.SetupWithManager)
}

func TestOrganizationReconciler_EnsureOrgRBAC(t *testing.T) {
	reconciler, client, _ := setupOrganizationTest()
	ctx := context.Background()

	// Create the organization namespace first
	orgNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "org-test-org",
		},
	}
	err := client.Create(ctx, orgNamespace)
	require.NoError(t, err)

	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin-group-1", "admin-group-2"},
			IsEnabled:   true,
		},
	}

	err = reconciler.setupOrgRBAC(ctx, org, "org-test-org")
	require.NoError(t, err)

	// Verify RoleBindings were created for each admin group
	var binding1 rbacv1.RoleBinding
	err = client.Get(ctx, types.NamespacedName{Name: "org-admin-admin-group-1", Namespace: "org-test-org"}, &binding1)
	require.NoError(t, err)

	assert.Equal(t, "ovim:org-admin", binding1.RoleRef.Name)
	assert.Equal(t, "ClusterRole", binding1.RoleRef.Kind)
	require.Len(t, binding1.Subjects, 1)
	assert.Equal(t, "Group", binding1.Subjects[0].Kind)
	assert.Equal(t, "admin-group-1", binding1.Subjects[0].Name)

	var binding2 rbacv1.RoleBinding
	err = client.Get(ctx, types.NamespacedName{Name: "org-admin-admin-group-2", Namespace: "org-test-org"}, &binding2)
	require.NoError(t, err)
	assert.Equal(t, "admin-group-2", binding2.Subjects[0].Name)
}

func TestOrganizationReconciler_SyncToDatabase(t *testing.T) {
	reconciler, _, mockStorage := setupOrganizationTest()
	ctx := context.Background()

	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin1", "admin2"},
			IsEnabled:   true,
		},
		Status: ovimv1.OrganizationStatus{
			Namespace: "org-test-org",
			Phase:     ovimv1.OrganizationPhaseActive,
		},
	}

	err := reconciler.syncToDatabase(ctx, org)
	require.NoError(t, err)

	// Verify organization was created in database
	dbOrg, err := mockStorage.GetOrganization("test-org")
	require.NoError(t, err)
	assert.Equal(t, "test-org", dbOrg.ID)
	assert.Equal(t, "Test Organization", dbOrg.Name)
	assert.Equal(t, "org-test-org", dbOrg.Namespace)
	assert.True(t, dbOrg.IsEnabled)
}

func TestOrganizationReconciler_SyncToDatabase_UpdateExisting(t *testing.T) {
	reconciler, _, mockStorage := setupOrganizationTest()
	ctx := context.Background()

	// Pre-create organization in database
	existingOrg := &models.Organization{
		ID:          "test-org",
		Name:        "test-org",
		DisplayName: func(s string) *string { return &s }("Old Name"),
		IsEnabled:   false,
	}
	err := mockStorage.CreateOrganization(existingOrg)
	require.NoError(t, err)

	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Updated Organization",
			Admins:      []string{"admin1"},
			IsEnabled:   true,
		},
		Status: ovimv1.OrganizationStatus{
			Namespace: "org-test-org",
			Phase:     ovimv1.OrganizationPhaseActive,
		},
	}

	err = reconciler.syncToDatabase(ctx, org)
	require.NoError(t, err)

	// Verify organization was updated in database
	dbOrg, err := mockStorage.GetOrganization("test-org")
	require.NoError(t, err)
	assert.Equal(t, "Updated Organization", dbOrg.Name)
	assert.True(t, dbOrg.IsEnabled)
}

func TestOrganizationReconciler_SyncToDatabase_Error(t *testing.T) {
	reconciler, _, mockStorage := setupOrganizationTest()
	ctx := context.Background()

	// Configure mock to fail
	mockStorage.SetError(true, "database connection failed")

	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin1"},
			IsEnabled:   true,
		},
	}

	err := reconciler.syncToDatabase(ctx, org)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database connection failed")
}
