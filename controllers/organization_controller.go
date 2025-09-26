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
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
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
	Scheme   *runtime.Scheme
	Storage  storage.Storage
	Recorder record.EventRecorder
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

	// Determine what triggered this reconciliation
	trigger := r.determineReconcileTrigger(ctx, req)
	logger = logger.WithValues("trigger", trigger)
	logger.Info("Starting organization reconciliation", "trigger", trigger)

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
			r.recordEvent(&org, corev1.EventTypeWarning, "FinalizerFailed", fmt.Sprintf("Failed to add finalizer: %v", err))
			return ctrl.Result{}, err
		}
		r.recordEvent(&org, corev1.EventTypeNormal, "OrganizationCreated", "Organization created and finalizer added")
		return ctrl.Result{}, nil
	}

	// Create organization namespace if it doesn't exist
	orgNamespace := fmt.Sprintf("org-%s", strings.ToLower(org.Name))
	if err := r.ensureOrgNamespace(ctx, &org, orgNamespace); err != nil {
		logger.Error(err, "unable to ensure organization namespace")
		r.recordEvent(&org, corev1.EventTypeWarning, "NamespaceCreationFailed", fmt.Sprintf("Failed to create namespace %s: %v", orgNamespace, err))
		r.updateOrgCondition(&org, ConditionReady, metav1.ConditionFalse, "NamespaceCreationFailed", err.Error())
		if err := r.Status().Update(ctx, &org); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Set up RBAC for org admins
	if err := r.setupOrgRBAC(ctx, &org, orgNamespace); err != nil {
		logger.Error(err, "unable to setup organization RBAC")
		r.recordEvent(&org, corev1.EventTypeWarning, "RBACSetupFailed", fmt.Sprintf("Failed to setup RBAC for namespace %s: %v", orgNamespace, err))
		r.updateOrgCondition(&org, ConditionReady, metav1.ConditionFalse, "RBACSetupFailed", err.Error())
		if err := r.Status().Update(ctx, &org); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update status with retry on conflict - only if something actually changed
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get latest version of the resource
		if getErr := r.Get(ctx, client.ObjectKeyFromObject(&org), &org); getErr != nil {
			return getErr
		}

		// Check if status update is needed (idempotent pattern)
		needsUpdate := false

		if org.Status.Namespace != orgNamespace {
			org.Status.Namespace = orgNamespace
			needsUpdate = true
		}

		if org.Status.Phase != ovimv1.OrganizationPhaseActive {
			org.Status.Phase = ovimv1.OrganizationPhaseActive
			needsUpdate = true
		}

		// Only update condition if it's actually different
		if r.shouldUpdateOrgCondition(&org, ConditionReady, metav1.ConditionTrue, "OrganizationReady", "Organization is ready and active") {
			r.updateOrgCondition(&org, ConditionReady, metav1.ConditionTrue, "OrganizationReady", "Organization is ready and active")
			needsUpdate = true
		}

		// Only write to etcd if something actually changed
		if needsUpdate {
			return r.Status().Update(ctx, &org)
		}

		// No-op: nothing changed, skip write
		return nil
	}); err != nil {
		logger.Error(err, "unable to update status after retries")
		return ctrl.Result{}, err
	}

	// Record successful organization activation
	r.recordEvent(&org, corev1.EventTypeNormal, "OrganizationActivated", fmt.Sprintf("Organization %s is now active with namespace %s", org.Name, orgNamespace))

	// Sync to database
	if err := r.syncToDatabase(ctx, &org); err != nil {
		logger.Error(err, "unable to sync to database")
		// Don't fail reconciliation for database sync issues
	}

	logger.Info("Organization reconciled successfully")
	// Requeue after 5 minutes to avoid continuous reconciliation
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
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
	r.recordEvent(org, corev1.EventTypeNormal, "NamespaceCreated", fmt.Sprintf("Created namespace %s for organization", namespaceName))
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
	r.recordEvent(org, corev1.EventTypeNormal, "RBACConfigured", fmt.Sprintf("Configured RBAC for %d admin groups in namespace %s", len(org.Spec.Admins), namespaceName))
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

		r.recordEvent(org, corev1.EventTypeWarning, "DeletionBlocked", fmt.Sprintf("Cannot delete organization: %d VDCs must be removed first", len(vdcList.Items)))
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
			r.recordEvent(org, corev1.EventTypeNormal, "NamespaceDeleted", fmt.Sprintf("Deleted namespace %s during organization cleanup", org.Status.Namespace))
		} else if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Remove from database
	if r.Storage != nil {
		if err := r.Storage.DeleteOrganization(org.Name); err != nil {
			// Only log error if it's not "not found" - organization may not exist in DB
			if err != storage.ErrNotFound {
				logger.Error(err, "unable to delete organization from database")
			}
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
	r.recordEvent(org, corev1.EventTypeNormal, "OrganizationDeleted", fmt.Sprintf("Organization %s has been successfully deleted", org.Name))
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

// updateOrgCondition updates a condition in the organization status only if something changed
func (r *OrganizationReconciler) updateOrgCondition(org *ovimv1.Organization, conditionType string, status metav1.ConditionStatus, reason, message string) {
	// Find existing condition and only update if something actually changed
	for i, existing := range org.Status.Conditions {
		if existing.Type == conditionType {
			// Only update if status, reason, or message changed
			if existing.Status != status || existing.Reason != reason || existing.Message != message {
				existing.Status = status
				existing.Reason = reason
				existing.Message = message
				existing.LastTransitionTime = metav1.Now() // Only update timestamp when values change
				org.Status.Conditions[i] = existing
			}
			// If nothing changed, do nothing (idempotent)
			return
		}
	}

	// Add new condition if not found
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	org.Status.Conditions = append(org.Status.Conditions, condition)
}

// shouldUpdateOrgCondition checks if a condition needs updating (helper for idempotency checks)
func (r *OrganizationReconciler) shouldUpdateOrgCondition(org *ovimv1.Organization, conditionType string, status metav1.ConditionStatus, reason, message string) bool {
	for _, existing := range org.Status.Conditions {
		if existing.Type == conditionType {
			return existing.Status != status || existing.Reason != reason || existing.Message != message
		}
	}
	return true // Condition doesn't exist, needs to be added
}

// recordEvent records an event for the given organization
func (r *OrganizationReconciler) recordEvent(org *ovimv1.Organization, eventType, reason, message string) {
	if r.Recorder != nil {
		r.Recorder.Event(org, eventType, reason, message)
	}
}

// determineReconcileTrigger analyzes the context and resource to determine what triggered the reconciliation
func (r *OrganizationReconciler) determineReconcileTrigger(ctx context.Context, req ctrl.Request) string {
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
		return "spec-change"
	}

	// Check for status updates by looking at recent condition changes
	for _, condition := range org.Status.Conditions {
		if condition.LastTransitionTime.Add(30 * time.Second).After(time.Now()) {
			return "status-change"
		}
	}

	// Check if this is triggered by RBAC sync
	if org.Status.LastRBACSync != nil && org.Status.LastRBACSync.Add(1*time.Minute).After(time.Now()) {
		return "rbac-sync-trigger"
	}

	// Check for finalizer operations
	if len(org.Finalizers) > 0 && org.DeletionTimestamp == nil {
		return "finalizer-update"
	}

	// Check if organization namespace doesn't exist yet
	if org.Status.Namespace == "" {
		return "namespace-creation-needed"
	}

	// Default cases
	if org.Status.Phase == "" || org.Status.Phase != ovimv1.OrganizationPhaseActive {
		return "activation-reconcile"
	}

	return "periodic-reconcile"
}

// SetupWithManager sets up the controller with the Manager
func (r *OrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.Organization{}).
		// Removed Owns() relationships to prevent reconciliation loops
		// Resources are managed but not watched to avoid conflicts
		Named("ovim-organization-controller").
		Complete(r)
}
