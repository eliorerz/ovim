package controllers

import (
	"context"
	"fmt"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

// Reconcile ensures RBAC consistency across organization and VDCs
func (r *RBACReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("organization", req.NamespacedName)

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

	// Update org status with last RBAC sync time
	org.Status.LastRBACSync = &metav1.Time{Time: time.Now()}
	org.Status.VDCCount = len(vdcList.Items)

	if err := r.Status().Update(ctx, &org); err != nil {
		logger.Error(err, "unable to update organization status")
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

		// Set owner reference to the VDC
		if err := controllerutil.SetControllerReference(vdc, roleBinding, r.Scheme); err != nil {
			return err
		}

		if errors.IsNotFound(err) {
			// Create new binding
			if err := r.Create(ctx, roleBinding); err != nil {
				logger.Error(err, "failed to create role binding", "admin", adminGroup)
				return err
			}
			logger.Info("Created admin binding", "admin", adminGroup)
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

// SetupWithManager sets up the controller with the Manager
func (r *RBACReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.Organization{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
