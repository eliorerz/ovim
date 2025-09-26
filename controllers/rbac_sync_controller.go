package controllers

import (
	"context"
	"fmt"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
)

// RBACReconciler ensures org admin changes propagate to all VDCs
type RBACReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ovim.io,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=ovim.io,resources=organizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/attach;pods/exec;pods/portforward;pods/proxy,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;replicasets;statefulsets;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/scale;replicasets/scale;statefulsets/scale,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metrics.k8s.io,resources=nodes;pods,verbs=get;list

// Reconcile ensures RBAC consistency across organization and VDCs
func (r *RBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("organization", req.NamespacedName)

	// Determine what triggered this reconciliation
	trigger := r.determineReconcileTrigger(ctx, req)
	logger = logger.WithValues("trigger", trigger)
	logger.Info("Starting RBAC sync reconciliation", "trigger", trigger)

	// Fetch the Organization instance
	var org ovimv1.Organization
	if err := r.Get(ctx, req.NamespacedName, &org); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch Organization")
		return ctrl.Result{}, err
	}

	// Skip if organization is being deleted
	if org.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	// Skip if organization is not ready
	if org.Status.Phase != ovimv1.OrganizationPhaseActive || org.Status.Namespace == "" {
		logger.Info("Organization not ready, skipping RBAC sync")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// List all VDCs belonging to this org
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := r.List(ctx, vdcList, client.InNamespace(org.Status.Namespace)); err != nil {
		logger.Error(err, "unable to list VDCs")
		return ctrl.Result{}, err
	}

	syncedVDCs := 0

	// For each VDC, ensure admin RoleBindings are current
	for _, vdc := range vdcList.Items {
		if vdc.Status.Namespace == "" {
			continue // VDC not ready yet
		}

		if err := r.syncVDCRBAC(ctx, &org, &vdc); err != nil {
			logger.Error(err, "failed to sync RBAC for VDC", "vdc", vdc.Name)
			continue
		}

		syncedVDCs++
	}

	// Update org status with last RBAC sync time using retry on conflict - only if needed
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get latest version of the resource
		if getErr := r.Get(ctx, client.ObjectKeyFromObject(&org), &org); getErr != nil {
			return getErr
		}

		// Check if status update is needed (idempotent pattern)
		needsUpdate := false

		// Only update VDC count if it changed
		if org.Status.VDCCount != len(vdcList.Items) {
			org.Status.VDCCount = len(vdcList.Items)
			needsUpdate = true
		}

		// Only update LastRBACSync if we actually synced some VDCs
		if syncedVDCs > 0 {
			org.Status.LastRBACSync = &metav1.Time{Time: time.Now()}
			needsUpdate = true
		}

		// Only write to etcd if something actually changed
		if needsUpdate {
			return r.Status().Update(ctx, &org)
		}

		// No-op: nothing changed, skip write
		return nil
	}); err != nil {
		logger.Error(err, "unable to update organization status after retries")
		return ctrl.Result{}, err
	}

	logger.Info("RBAC sync completed", "vdcs", len(vdcList.Items), "synced", syncedVDCs)

	// Requeue after 10 minutes for regular sync
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// syncVDCRBAC synchronizes RBAC for a single VDC
func (r *RBACReconciler) syncVDCRBAC(ctx context.Context, org *ovimv1.Organization, vdc *ovimv1.VirtualDataCenter) error {
	logger := log.FromContext(ctx).WithValues("org", org.Name, "vdc", vdc.Name)

	vdcNamespace := vdc.Status.Namespace
	if vdcNamespace == "" {
		return fmt.Errorf("VDC namespace not set")
	}

	// Get existing VDC admin bindings
	existingBindings := &rbacv1.RoleBindingList{}
	if err := r.List(ctx, existingBindings,
		client.InNamespace(vdcNamespace),
		client.MatchingLabels{"managed-by": "ovim", "type": "vdc-admin"}); err != nil {
		return err
	}

	// Create a map of current admin groups for easy lookup
	currentAdmins := make(map[string]bool)
	for _, admin := range org.Spec.Admins {
		currentAdmins[admin] = true
	}

	// Remove bindings for admins that are no longer in the org
	for _, binding := range existingBindings.Items {
		// Extract admin group from binding name (format: vdc-admin-{group})
		if len(binding.Subjects) > 0 {
			adminGroup := binding.Subjects[0].Name
			if !currentAdmins[adminGroup] {
				if err := r.Delete(ctx, &binding); err != nil && !errors.IsNotFound(err) {
					logger.Error(err, "failed to delete obsolete role binding", "binding", binding.Name)
				} else {
					logger.Info("Removed obsolete admin binding", "admin", adminGroup)
				}
			}
		}
	}

	// Create or update bindings for current admins
	for _, adminGroup := range org.Spec.Admins {
		bindingName := fmt.Sprintf("vdc-admin-%s", adminGroup)

		// Check if binding already exists
		existingBinding := &rbacv1.RoleBinding{}
		err := r.Get(ctx, client.ObjectKey{
			Name:      bindingName,
			Namespace: vdcNamespace,
		}, existingBinding)

		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: vdcNamespace,
				Labels: map[string]string{
					"managed-by": "ovim",
					"type":       "vdc-admin",
					"org":        org.Name,
					"vdc":        vdc.Name,
				},
			},
			Subjects: []rbacv1.Subject{{
				Kind:     "Group",
				Name:     adminGroup,
				APIGroup: "rbac.authorization.k8s.io",
			}},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     "ovim:vdc-admin",
				APIGroup: "rbac.authorization.k8s.io",
			},
		}

		// Note: Cannot set cross-namespace owner references, using labels for tracking
		roleBinding.Labels["ovim.io/vdc-id"] = vdc.Name
		roleBinding.Labels["ovim.io/vdc-namespace"] = vdc.Namespace

		if errors.IsNotFound(err) {
			// Create new binding
			if err := r.Create(ctx, roleBinding); err != nil {
				if errors.IsAlreadyExists(err) {
					// Handle race condition - another reconcile created it
					logger.Info("Role binding already exists (race condition)", "admin", adminGroup)
				} else {
					logger.Error(err, "failed to create role binding", "admin", adminGroup)
					return err
				}
			} else {
				logger.Info("Created admin binding", "admin", adminGroup)
			}
		} else {
			// Update existing binding if needed
			if !r.roleBindingsEqual(existingBinding, roleBinding) {
				existingBinding.Subjects = roleBinding.Subjects
				existingBinding.RoleRef = roleBinding.RoleRef
				existingBinding.Labels = roleBinding.Labels

				if err := r.Update(ctx, existingBinding); err != nil {
					logger.Error(err, "failed to update role binding", "admin", adminGroup)
					return err
				}
				logger.Info("Updated admin binding", "admin", adminGroup)
			}
		}
	}

	return nil
}

// roleBindingsEqual compares two role bindings for equality
func (r *RBACReconciler) roleBindingsEqual(a, b *rbacv1.RoleBinding) bool {
	// Compare subjects
	if len(a.Subjects) != len(b.Subjects) {
		return false
	}

	for i, subject := range a.Subjects {
		if i >= len(b.Subjects) ||
			subject.Kind != b.Subjects[i].Kind ||
			subject.Name != b.Subjects[i].Name ||
			subject.APIGroup != b.Subjects[i].APIGroup {
			return false
		}
	}

	// Compare role ref
	if a.RoleRef.Kind != b.RoleRef.Kind ||
		a.RoleRef.Name != b.RoleRef.Name ||
		a.RoleRef.APIGroup != b.RoleRef.APIGroup {
		return false
	}

	return true
}

// determineReconcileTrigger analyzes the context and resource to determine what triggered the reconciliation
func (r *RBACReconciler) determineReconcileTrigger(ctx context.Context, req ctrl.Request) string {
	// Get the current organization to analyze
	var org ovimv1.Organization
	if err := r.Get(ctx, req.NamespacedName, &org); err != nil {
		if errors.IsNotFound(err) {
			return "resource-deleted"
		}
		return "resource-fetch-error"
	}

	// Check if this is a deletion
	if org.DeletionTimestamp != nil {
		return "resource-deletion"
	}

	// Check if this is a new resource (recently created)
	if org.CreationTimestamp.Add(10 * time.Second).After(time.Now()) {
		return "resource-creation"
	}

	// Check if this is a new resource or has recent changes (admin list changes)
	if org.Generation > 1 {
		return "admin-list-change"
	}

	// Check if this is a scheduled RBAC sync (every 10 minutes)
	if org.Status.LastRBACSync == nil {
		return "initial-rbac-sync"
	}
	if org.Status.LastRBACSync.Add(10 * time.Minute).Before(time.Now()) {
		return "scheduled-rbac-sync"
	}

	// Check if organization became ready (triggering RBAC setup)
	if org.Status.Phase == ovimv1.OrganizationPhaseActive && org.Status.Namespace != "" {
		// Check for recent condition changes
		for _, condition := range org.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
				if condition.LastTransitionTime.Add(1 * time.Minute).After(time.Now()) {
					return "org-activation"
				}
			}
		}
	}

	// Check if VDC count changed (new/deleted VDCs requiring RBAC updates)
	// This is indirect - we can't easily detect VDC changes from here, but we can infer
	return "rbac-consistency-check"
}

// SetupWithManager sets up the controller with the Manager
func (r *RBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.Organization{}).
		// Removed Owns() relationship to prevent reconciliation loops
		// RoleBindings are managed but not watched to avoid conflicts
		Named("ovim-rbac-sync-controller").
		Complete(r)
}
