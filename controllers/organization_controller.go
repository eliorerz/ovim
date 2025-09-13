package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

const (
	// OrganizationFinalizer is the finalizer for Organization resources
	OrganizationFinalizer = "ovim.io/org-finalizer"

	// ConditionReady indicates if the organization is ready
	ConditionReady = "Ready"

	// ConditionReadyForDeletion indicates if the organization can be deleted
	ConditionReadyForDeletion = "ReadyForDeletion"
)

// OrganizationReconciler reconciles a Organization object
type OrganizationReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Storage storage.Storage
}

// +kubebuilder:rbac:groups=ovim.io,resources=organizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ovim.io,resources=organizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ovim.io,resources=organizations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch

// Reconcile handles Organization resource changes
func (r *OrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("organization", req.NamespacedName)

	// Fetch the Organization instance
	var org ovimv1.Organization
	if err := r.Get(ctx, req.NamespacedName, &org); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return without error (it was deleted)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch Organization")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if org.DeletionTimestamp != nil {
		return r.handleOrgDeletion(ctx, &org)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&org, OrganizationFinalizer) {
		controllerutil.AddFinalizer(&org, OrganizationFinalizer)
		if err := r.Update(ctx, &org); err != nil {
			logger.Error(err, "unable to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Create organization namespace if it doesn't exist
	orgNamespace := fmt.Sprintf("org-%s", strings.ToLower(org.Name))
	if err := r.ensureOrgNamespace(ctx, &org, orgNamespace); err != nil {
		logger.Error(err, "unable to ensure organization namespace")
		r.updateOrgCondition(&org, ConditionReady, metav1.ConditionFalse, "NamespaceCreationFailed", err.Error())
		if err := r.Status().Update(ctx, &org); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Set up RBAC for org admins
	if err := r.setupOrgRBAC(ctx, &org, orgNamespace); err != nil {
		logger.Error(err, "unable to setup organization RBAC")
		r.updateOrgCondition(&org, ConditionReady, metav1.ConditionFalse, "RBACSetupFailed", err.Error())
		if err := r.Status().Update(ctx, &org); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update status
	org.Status.Namespace = orgNamespace
	org.Status.Phase = ovimv1.OrganizationPhaseActive
	r.updateOrgCondition(&org, ConditionReady, metav1.ConditionTrue, "OrganizationReady", "Organization is ready and active")

	if err := r.Status().Update(ctx, &org); err != nil {
		logger.Error(err, "unable to update status")
		return ctrl.Result{}, err
	}

	// Sync to database
	if err := r.syncToDatabase(ctx, &org); err != nil {
		logger.Error(err, "unable to sync to database")
		// Don't fail reconciliation for database sync issues
	}

	logger.Info("Organization reconciled successfully")
	return ctrl.Result{}, nil
}

// ensureOrgNamespace creates organization namespace if it doesn't exist
func (r *OrganizationReconciler) ensureOrgNamespace(ctx context.Context, org *ovimv1.Organization, namespaceName string) error {
	logger := log.FromContext(ctx)

	// Check if namespace exists
	var ns corev1.Namespace
	err := r.Get(ctx, types.NamespacedName{Name: namespaceName}, &ns)
	if err == nil {
		// Namespace exists, we're done
		return nil
	}

	if !errors.IsNotFound(err) {
		return err
	}

	// Create namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "organization",
				"app.kubernetes.io/managed-by": "ovim",
				"ovim.io/organization-id":      org.Name,
				"ovim.io/organization-name":    org.Name,
				"type":                         "org",
				"org":                          org.Name,
			},
			Annotations: map[string]string{
				"ovim.io/organization-description": org.Spec.Description,
				"ovim.io/created-by":               "ovim-controller",
				"ovim.io/created-at":               time.Now().Format(time.RFC3339),
			},
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(org, namespace, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, namespace); err != nil {
		return err
	}

	logger.Info("Created organization namespace", "namespace", namespaceName)
	return nil
}

// setupOrgRBAC creates role bindings for organization admins
func (r *OrganizationReconciler) setupOrgRBAC(ctx context.Context, org *ovimv1.Organization, namespaceName string) error {
	logger := log.FromContext(ctx)

	// Clean up existing bindings first
	existingBindings := &rbacv1.RoleBindingList{}
	if err := r.List(ctx, existingBindings,
		client.InNamespace(namespaceName),
		client.MatchingLabels{"managed-by": "ovim", "type": "org-admin"}); err != nil {
		return err
	}

	for _, binding := range existingBindings.Items {
		if err := r.Delete(ctx, &binding); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete existing role binding", "binding", binding.Name)
		}
	}

	// Create role bindings for org admins
	for _, adminGroup := range org.Spec.Admins {
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("org-admin-%s", adminGroup),
				Namespace: namespaceName,
				Labels: map[string]string{
					"managed-by": "ovim",
					"type":       "org-admin",
					"org":        org.Name,
				},
			},
			Subjects: []rbacv1.Subject{{
				Kind:     "Group",
				Name:     adminGroup,
				APIGroup: "rbac.authorization.k8s.io",
			}},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     "ovim:org-admin",
				APIGroup: "rbac.authorization.k8s.io",
			},
		}

		// Set owner reference
		if err := controllerutil.SetControllerReference(org, roleBinding, r.Scheme); err != nil {
			return err
		}

		if err := r.Create(ctx, roleBinding); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	logger.Info("Set up organization RBAC", "namespace", namespaceName, "admins", len(org.Spec.Admins))
	return nil
}

// handleOrgDeletion handles organization deletion with proper cleanup
func (r *OrganizationReconciler) handleOrgDeletion(ctx context.Context, org *ovimv1.Organization) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("organization", org.Name)

	// Check if any VDCs still exist in this organization
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := r.List(ctx, vdcList, client.InNamespace(org.Status.Namespace)); err != nil {
		logger.Error(err, "unable to list VDCs")
		return ctrl.Result{}, err
	}

	if len(vdcList.Items) > 0 {
		// Update status to indicate VDCs must be deleted first
		org.Status.Phase = ovimv1.OrganizationPhaseFailed
		r.updateOrgCondition(org, ConditionReadyForDeletion, metav1.ConditionFalse, "VDCsExist",
			fmt.Sprintf("%d VDCs must be deleted before organization", len(vdcList.Items)))

		if err := r.Status().Update(ctx, org); err != nil {
			logger.Error(err, "unable to update status")
		}

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// All VDCs deleted, safe to remove org
	// Delete org namespace if it exists
	if org.Status.Namespace != "" {
		orgNamespace := &corev1.Namespace{}
		err := r.Get(ctx, types.NamespacedName{Name: org.Status.Namespace}, orgNamespace)
		if err == nil {
			if err := r.Delete(ctx, orgNamespace); err != nil {
				logger.Error(err, "unable to delete organization namespace")
				return ctrl.Result{}, err
			}
			logger.Info("Deleted organization namespace", "namespace", org.Status.Namespace)
		} else if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Remove from database
	if r.Storage != nil {
		if err := r.Storage.DeleteOrganization(org.Name); err != nil {
			logger.Error(err, "unable to delete organization from database")
			// Don't block deletion for database issues
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(org, OrganizationFinalizer)
	if err := r.Update(ctx, org); err != nil {
		logger.Error(err, "unable to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.Info("Organization deleted successfully")
	return ctrl.Result{}, nil
}

// syncToDatabase synchronizes organization data to the database
func (r *OrganizationReconciler) syncToDatabase(ctx context.Context, org *ovimv1.Organization) error {
	if r.Storage == nil {
		return nil // No database configured
	}

	logger := log.FromContext(ctx)

	// Check if organization exists in database
	_, err := r.Storage.GetOrganization(org.Name)
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	dbOrg := &models.Organization{
		ID:          org.Name,
		Name:        org.Spec.DisplayName,
		Description: org.Spec.Description,
		Namespace:   org.Status.Namespace,
		IsEnabled:   org.Spec.IsEnabled,
		CRName:      org.Name,
		CRNamespace: org.Namespace,
	}

	if err == storage.ErrNotFound {
		// Create new organization
		if err := r.Storage.CreateOrganization(dbOrg); err != nil {
			return err
		}
		logger.Info("Created organization in database", "org", org.Name)
	} else {
		// Update existing organization
		if err := r.Storage.UpdateOrganization(dbOrg); err != nil {
			return err
		}
		logger.Info("Updated organization in database", "org", org.Name)
	}

	return nil
}

// updateOrgCondition updates a condition in the organization status
func (r *OrganizationReconciler) updateOrgCondition(org *ovimv1.Organization, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Find existing condition and update it, or append new one
	for i, existing := range org.Status.Conditions {
		if existing.Type == conditionType {
			org.Status.Conditions[i] = condition
			return
		}
	}

	org.Status.Conditions = append(org.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager
func (r *OrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.Organization{}).
		Owns(&corev1.Namespace{}).
		Owns(&rbacv1.RoleBinding{}).
		Named("ovim-organization-controller").
		Complete(r)
}
