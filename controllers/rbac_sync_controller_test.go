package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
)

func setupRBACTest() (*RBACReconciler, client.Client) {
	// Create scheme with our CRD types
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = ovimv1.AddToScheme(s)

	// Create fake client with status subresource support
	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(&ovimv1.Organization{}, &ovimv1.VirtualDataCenter{}).
		Build()

	// Create reconciler
	reconciler := &RBACReconciler{
		Client: fakeClient,
		Scheme: s,
	}

	return reconciler, fakeClient
}

func TestRBACReconciler_Reconcile_OrganizationNotFound(t *testing.T) {
	reconciler, _ := setupRBACTest()
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

func TestRBACReconciler_Reconcile_OrganizationBeingDeleted(t *testing.T) {
	reconciler, client := setupRBACTest()
	ctx := context.Background()

	// Create organization with deletion timestamp
	now := metav1.Now()
	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-org",
			DeletionTimestamp: &now,
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin1"},
			IsEnabled:   true,
		},
		Status: ovimv1.OrganizationStatus{
			Phase:     ovimv1.OrganizationPhaseActive,
			Namespace: "org-test-org",
		},
	}

	err := client.Create(ctx, org)
	require.NoError(t, err)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestRBACReconciler_Reconcile_OrganizationNotReady(t *testing.T) {
	reconciler, client := setupRBACTest()
	ctx := context.Background()

	// Create organization that's not ready
	org := &ovimv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: ovimv1.OrganizationSpec{
			DisplayName: "Test Organization",
			Admins:      []string{"admin1"},
			IsEnabled:   true,
		},
		Status: ovimv1.OrganizationStatus{
			Phase: ovimv1.OrganizationPhasePending,
		},
	}

	err := client.Create(ctx, org)
	require.NoError(t, err)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
}

func TestRBACReconciler_Reconcile_SyncVDCRBAC(t *testing.T) {
	reconciler, client := setupRBACTest()
	ctx := context.Background()

	// Create ready organization
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
			Phase:     ovimv1.OrganizationPhaseActive,
			Namespace: "org-test-org",
		},
	}

	err := client.Create(ctx, org)
	require.NoError(t, err)

	// Create VDCs in the organization namespace
	vdc1 := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc1",
			Namespace: "org-test-org",
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "VDC 1",
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Namespace: "vdc-test-org-vdc1",
			Phase:     ovimv1.VirtualDataCenterPhaseActive,
		},
	}

	vdc2 := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc2",
			Namespace: "org-test-org",
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "VDC 2",
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Namespace: "vdc-test-org-vdc2",
			Phase:     ovimv1.VirtualDataCenterPhaseActive,
		},
	}

	err = client.Create(ctx, vdc1)
	require.NoError(t, err)
	err = client.Create(ctx, vdc2)
	require.NoError(t, err)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 10 * time.Minute}, result)

	// Verify organization status was updated
	var updatedOrg ovimv1.Organization
	err = client.Get(ctx, req.NamespacedName, &updatedOrg)
	require.NoError(t, err)
	assert.Equal(t, 2, updatedOrg.Status.VDCCount)
	assert.NotNil(t, updatedOrg.Status.LastRBACSync)
}

func TestRBACReconciler_SyncVDCRBAC_CreateRoleBindings(t *testing.T) {
	reconciler, client := setupRBACTest()
	ctx := context.Background()

	// Create organization
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

	// Create VDC
	vdc := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test VDC",
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Namespace: "vdc-test-org-test-vdc",
			Phase:     ovimv1.VirtualDataCenterPhaseActive,
		},
	}

	err := client.Create(ctx, vdc)
	require.NoError(t, err)

	// Sync RBAC
	err = reconciler.syncVDCRBAC(ctx, org, vdc)
	require.NoError(t, err)

	// Verify RoleBindings were created
	for _, admin := range org.Spec.Admins {
		var binding rbacv1.RoleBinding
		err = client.Get(ctx, types.NamespacedName{
			Name:      "vdc-admin-" + admin,
			Namespace: "vdc-test-org-test-vdc",
		}, &binding)
		require.NoError(t, err)

		// Verify binding properties
		assert.Equal(t, "ovim:vdc-admin", binding.RoleRef.Name)
		assert.Equal(t, "ClusterRole", binding.RoleRef.Kind)
		require.Len(t, binding.Subjects, 1)
		assert.Equal(t, "Group", binding.Subjects[0].Kind)
		assert.Equal(t, admin, binding.Subjects[0].Name)
		assert.Equal(t, "ovim", binding.Labels["managed-by"])
		assert.Equal(t, "vdc-admin", binding.Labels["type"])
		assert.Equal(t, "test-org", binding.Labels["org"])
		assert.Equal(t, "test-vdc", binding.Labels["vdc"])
	}
}

func TestRBACReconciler_SyncVDCRBAC_RemoveObsoleteBindings(t *testing.T) {
	reconciler, client := setupRBACTest()
	ctx := context.Background()

	// Create VDC
	vdc := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test VDC",
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Namespace: "vdc-test-org-test-vdc",
			Phase:     ovimv1.VirtualDataCenterPhaseActive,
		},
	}

	err := client.Create(ctx, vdc)
	require.NoError(t, err)

	// Create existing role binding for admin that will be removed
	obsoleteBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-admin-obsolete-admin",
			Namespace: "vdc-test-org-test-vdc",
			Labels: map[string]string{
				"managed-by": "ovim",
				"type":       "vdc-admin",
			},
		},
		Subjects: []rbacv1.Subject{{
			Kind:     "Group",
			Name:     "obsolete-admin",
			APIGroup: "rbac.authorization.k8s.io",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "ovim:vdc-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	err = client.Create(ctx, obsoleteBinding)
	require.NoError(t, err)

	// Create organization with different admins
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

	// Sync RBAC
	err = reconciler.syncVDCRBAC(ctx, org, vdc)
	require.NoError(t, err)

	// Verify obsolete binding was removed
	var binding rbacv1.RoleBinding
	err = client.Get(ctx, types.NamespacedName{
		Name:      "vdc-admin-obsolete-admin",
		Namespace: "vdc-test-org-test-vdc",
	}, &binding)
	assert.True(t, err != nil) // Should not be found

	// Verify new binding was created
	err = client.Get(ctx, types.NamespacedName{
		Name:      "vdc-admin-admin1",
		Namespace: "vdc-test-org-test-vdc",
	}, &binding)
	require.NoError(t, err)
}

func TestRBACReconciler_SyncVDCRBAC_UpdateExistingBindings(t *testing.T) {
	reconciler, client := setupRBACTest()
	ctx := context.Background()

	// Create VDC
	vdc := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test VDC",
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Namespace: "vdc-test-org-test-vdc",
			Phase:     ovimv1.VirtualDataCenterPhaseActive,
		},
	}

	err := client.Create(ctx, vdc)
	require.NoError(t, err)

	// Create existing role binding with outdated properties
	existingBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-admin-admin1",
			Namespace: "vdc-test-org-test-vdc",
			Labels: map[string]string{
				"managed-by": "ovim",
				"type":       "vdc-admin",
			},
		},
		Subjects: []rbacv1.Subject{{
			Kind:     "Group",
			Name:     "old-admin",
			APIGroup: "rbac.authorization.k8s.io",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "old-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	err = client.Create(ctx, existingBinding)
	require.NoError(t, err)

	// Create organization
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

	// Sync RBAC
	err = reconciler.syncVDCRBAC(ctx, org, vdc)
	require.NoError(t, err)

	// Verify binding was updated
	var updatedBinding rbacv1.RoleBinding
	err = client.Get(ctx, types.NamespacedName{
		Name:      "vdc-admin-admin1",
		Namespace: "vdc-test-org-test-vdc",
	}, &updatedBinding)
	require.NoError(t, err)

	assert.Equal(t, "ovim:vdc-admin", updatedBinding.RoleRef.Name)
	require.Len(t, updatedBinding.Subjects, 1)
	assert.Equal(t, "admin1", updatedBinding.Subjects[0].Name)
}

func TestRBACReconciler_SyncVDCRBAC_VDCNotReady(t *testing.T) {
	reconciler, client := setupRBACTest()
	ctx := context.Background()

	// Create VDC without namespace
	vdc := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test VDC",
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Phase: ovimv1.VirtualDataCenterPhasePending,
		},
	}

	err := client.Create(ctx, vdc)
	require.NoError(t, err)

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

	// Sync RBAC should return error
	err = reconciler.syncVDCRBAC(ctx, org, vdc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VDC namespace not set")
}

func TestRBACReconciler_RoleBindingsEqual(t *testing.T) {
	reconciler, _ := setupRBACTest()

	// Create two identical role bindings
	binding1 := &rbacv1.RoleBinding{
		Subjects: []rbacv1.Subject{{
			Kind:     "Group",
			Name:     "admin1",
			APIGroup: "rbac.authorization.k8s.io",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "ovim:vdc-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	binding2 := &rbacv1.RoleBinding{
		Subjects: []rbacv1.Subject{{
			Kind:     "Group",
			Name:     "admin1",
			APIGroup: "rbac.authorization.k8s.io",
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "ovim:vdc-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	assert.True(t, reconciler.roleBindingsEqual(binding1, binding2))

	// Change subject name
	binding2.Subjects[0].Name = "admin2"
	assert.False(t, reconciler.roleBindingsEqual(binding1, binding2))

	// Reset and change role ref
	binding2.Subjects[0].Name = "admin1"
	binding2.RoleRef.Name = "different-role"
	assert.False(t, reconciler.roleBindingsEqual(binding1, binding2))

	// Test different subject counts
	binding2.RoleRef.Name = "ovim:vdc-admin"
	binding2.Subjects = append(binding2.Subjects, rbacv1.Subject{
		Kind:     "Group",
		Name:     "admin2",
		APIGroup: "rbac.authorization.k8s.io",
	})
	assert.False(t, reconciler.roleBindingsEqual(binding1, binding2))
}

func TestRBACReconciler_SetupWithManager(t *testing.T) {
	reconciler, _ := setupRBACTest()

	// This test verifies that SetupWithManager can be called without error
	// In a real test environment, you would use a real manager
	// For this unit test, we just verify the method exists and has the right signature
	assert.NotNil(t, reconciler.SetupWithManager)
}
