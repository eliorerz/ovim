package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

const (
	// VDCFinalizer is the finalizer for VirtualDataCenter resources
	VDCFinalizer = "ovim.io/vdc-finalizer"

	// HubDeletionAnnotation is set by the hub server to trigger VDC deletion on spoke
	HubDeletionAnnotation = "ovim.io/hub-delete-requested"
	// HubDeletionValue is the value set when hub requests deletion
	HubDeletionValue = "true"
)

// VirtualDataCenterReconciler reconciles a VirtualDataCenter object
type VirtualDataCenterReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Storage storage.Storage
}

// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters/finalizers,verbs=update
// +kubebuilder:rbac:groups=ovim.io,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/attach;pods/exec;pods/portforward;pods/proxy,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;replicasets;statefulsets;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/scale;replicasets/scale;statefulsets/scale,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metrics.k8s.io,resources=nodes;pods,verbs=get;list

// Reconcile handles VirtualDataCenter resource changes
func (r *VirtualDataCenterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vdc", req.NamespacedName)

	// Determine what triggered this reconciliation
	trigger := r.determineReconcileTrigger(ctx, req)
	logger = logger.WithValues("trigger", trigger)
	logger.Info("Starting VDC reconciliation", "trigger", trigger)

	// Skip full reconciliation if this was only triggered by metrics updates
	// to break the feedback loop between VDC and metrics controllers
	if trigger == "metrics-update-trigger" {
		logger.V(4).Info("Skipping VDC reconciliation - triggered only by metrics update")
		return ctrl.Result{}, nil
	}

	// Skip full reconciliation if this was only triggered by condition changes
	// to break the feedback loop where VDC controller updates conditions and triggers itself
	if strings.HasPrefix(trigger, "condition-change-") {
		logger.V(4).Info("Skipping VDC reconciliation - triggered only by condition change", "trigger", trigger)
		return ctrl.Result{}, nil
	}

	// Fetch the VirtualDataCenter instance
	var vdc ovimv1.VirtualDataCenter
	if err := r.Get(ctx, req.NamespacedName, &vdc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch VirtualDataCenter")
		return ctrl.Result{}, err
	}

	// Hub controller should only process hub-managed VDCs, not spoke VDCs
	// Spoke VDCs are identified by the "ovim.io/managed-by": "spoke-agent" label
	if managedBy, exists := vdc.Labels["ovim.io/managed-by"]; exists && managedBy == "spoke-agent" {
		logger.V(4).Info("Skipping spoke-managed VDC on hub controller", "vdc", vdc.Name, "managed-by", managedBy)
		return ctrl.Result{}, nil
	}

	// For safety in same-cluster deployments: if VDC has no managed-by label and
	// we detect spoke agent presence, add hub label to claim ownership
	if managedBy, exists := vdc.Labels["ovim.io/managed-by"]; !exists || managedBy == "" {
		// Add hub managed-by label to claim this VDC
		if vdc.Labels == nil {
			vdc.Labels = make(map[string]string)
		}
		vdc.Labels["ovim.io/managed-by"] = "hub-controller"
		if err := r.Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to add hub management label")
			return ctrl.Result{}, err
		}
		logger.Info("Claimed VDC for hub management", "vdc", vdc.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle deletion
	if vdc.DeletionTimestamp != nil {
		return r.handleVDCDeletion(ctx, &vdc)
	}

	// Check if VDC is marked for deletion via OVIM API annotations
	if vdc.Annotations != nil {
		if deletedAt, exists := vdc.Annotations["ovim.io/deleted-at"]; exists && deletedAt != "" {
			if deletionStatus, statusExists := vdc.Annotations["ovim.io/deletion-status"]; statusExists && deletionStatus == "pending" {
				logger.Info("VDC marked for deletion via OVIM API, triggering Kubernetes deletion",
					"deleted-at", deletedAt, "deleted-by", vdc.Annotations["ovim.io/deleted-by"])

				// Trigger actual Kubernetes deletion
				if err := r.Delete(ctx, &vdc); err != nil {
					logger.Error(err, "unable to trigger VDC deletion")
					return ctrl.Result{RequeueAfter: 10 * time.Second}, err
				}
				logger.Info("Successfully triggered VDC deletion")
				return ctrl.Result{}, nil
			}
		}
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&vdc, VDCFinalizer) {
		controllerutil.AddFinalizer(&vdc, VDCFinalizer)
		if err := r.Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get parent organization
	orgCR := &ovimv1.Organization{}
	if err := r.Get(ctx, types.NamespacedName{Name: vdc.Spec.OrganizationRef}, orgCR); err != nil {
		logger.Error(err, "unable to get parent organization")
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "OrganizationNotFound",
			fmt.Sprintf("Parent organization %s not found", vdc.Spec.OrganizationRef))
		if err := r.Status().Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Create VDC workload namespace - ensure uniqueness across organizations
	// Format: vdc-{org}-{vdc-name}
	// Since VDC names are unique within each organization namespace, this ensures global uniqueness
	vdcNamespace := fmt.Sprintf("vdc-%s-%s",
		strings.ToLower(vdc.Spec.OrganizationRef),
		strings.ToLower(vdc.Name))

	if err := r.ensureVDCNamespace(ctx, &vdc, vdcNamespace); err != nil {
		logger.Error(err, "unable to ensure VDC namespace")
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "NamespaceCreationFailed", err.Error())
		if err := r.Status().Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Apply ResourceQuota
	if err := r.ensureResourceQuota(ctx, &vdc, vdcNamespace); err != nil {
		logger.Error(err, "unable to ensure resource quota")
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "ResourceQuotaFailed", err.Error())
		if err := r.Status().Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Apply LimitRange if specified
	if vdc.Spec.LimitRange != nil {
		if err := r.ensureLimitRange(ctx, &vdc, vdcNamespace); err != nil {
			logger.Error(err, "unable to ensure limit range")
			r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "LimitRangeFailed", err.Error())
			if err := r.Status().Update(ctx, &vdc); err != nil {
				logger.Error(err, "unable to update status")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
	}

	// Set up RBAC - inherit org admins from parent Org
	if err := r.setupVDCRBAC(ctx, &vdc, orgCR, vdcNamespace); err != nil {
		logger.Error(err, "unable to setup VDC RBAC")
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "RBACSetupFailed", err.Error())
		if err := r.Status().Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Apply NetworkPolicy if specified
	if err := r.ensureNetworkPolicy(ctx, &vdc, vdcNamespace); err != nil {
		logger.Error(err, "unable to ensure network policy")
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "NetworkPolicyFailed", err.Error())
		if err := r.Status().Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update status with retry on conflict - only if something actually changed
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get latest version of the resource
		if getErr := r.Get(ctx, client.ObjectKeyFromObject(&vdc), &vdc); getErr != nil {
			return getErr
		}

		// Check if status update is needed (idempotent pattern)
		needsUpdate := false

		if vdc.Status.Namespace != vdcNamespace {
			vdc.Status.Namespace = vdcNamespace
			needsUpdate = true
		}

		if vdc.Status.Phase != ovimv1.VirtualDataCenterPhaseActive {
			vdc.Status.Phase = ovimv1.VirtualDataCenterPhaseActive
			needsUpdate = true
		}

		// Only update condition if it's actually different
		if r.shouldUpdateVDCCondition(&vdc, ConditionReady, metav1.ConditionTrue, "VDCReady", "VDC is ready and active") {
			r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionTrue, "VDCReady", "VDC is ready and active")
			needsUpdate = true
		}

		// Only write to etcd if something actually changed
		if needsUpdate {
			return r.Status().Update(ctx, &vdc)
		}

		// No-op: nothing changed, skip write
		return nil
	}); err != nil {
		logger.Error(err, "unable to update status after retries")
		return ctrl.Result{}, err
	}

	// Sync to database
	if err := r.syncVDCToDatabase(ctx, &vdc); err != nil {
		logger.Error(err, "unable to sync VDC to database")
		// Don't fail reconciliation for database sync issues
	}

	logger.Info("VDC reconciled successfully")
	// Requeue after 5 minutes to avoid continuous reconciliation
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// ensureVDCNamespace creates VDC workload namespace if it doesn't exist
func (r *VirtualDataCenterReconciler) ensureVDCNamespace(ctx context.Context, vdc *ovimv1.VirtualDataCenter, namespaceName string) error {
	logger := log.FromContext(ctx)

	// Check if namespace exists
	var ns corev1.Namespace
	err := r.Get(ctx, types.NamespacedName{Name: namespaceName}, &ns)
	if err == nil {
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
				"app.kubernetes.io/component":  "vdc",
				"app.kubernetes.io/managed-by": "ovim",
				"ovim.io/organization":         vdc.Spec.OrganizationRef,
				"ovim.io/vdc":                  vdc.Name,
				"type":                         "vdc",
				"org":                          vdc.Spec.OrganizationRef,
				"vdc":                          vdc.Name,
			},
			Annotations: map[string]string{
				"ovim.io/vdc-description": vdc.Spec.Description,
				"ovim.io/created-by":      "ovim-controller",
				"ovim.io/created-at":      time.Now().Format(time.RFC3339),
			},
		},
	}

	// Note: Cannot set owner reference for cluster-scoped resources (Namespace)
	// from namespace-scoped resources (VDC). Using labels for tracking instead.
	namespace.Labels["ovim.io/vdc-id"] = vdc.Name
	namespace.Labels["ovim.io/vdc-namespace"] = vdc.Namespace

	if err := r.Create(ctx, namespace); err != nil {
		return err
	}

	logger.Info("Created VDC namespace", "namespace", namespaceName)
	return nil
}

// ensureResourceQuota creates or updates ResourceQuota for the VDC
func (r *VirtualDataCenterReconciler) ensureResourceQuota(ctx context.Context, vdc *ovimv1.VirtualDataCenter, namespaceName string) error {
	logger := log.FromContext(ctx)

	desiredQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-quota",
			Namespace: namespaceName,
			Labels: map[string]string{
				"managed-by": "ovim",
				"type":       "vdc-quota",
				"org":        vdc.Spec.OrganizationRef,
				"vdc":        vdc.Name,
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsCPU:     resource.MustParse(vdc.Spec.Quota.CPU),
				corev1.ResourceRequestsMemory:  resource.MustParse(vdc.Spec.Quota.Memory),
				corev1.ResourceRequestsStorage: resource.MustParse(vdc.Spec.Quota.Storage),
			},
		},
	}

	// Note: Cannot set cross-namespace owner references, using labels for tracking
	desiredQuota.Labels["ovim.io/vdc-id"] = vdc.Name
	desiredQuota.Labels["ovim.io/vdc-namespace"] = vdc.Namespace

	// Check if quota already exists
	existingQuota := &corev1.ResourceQuota{}
	err := r.Get(ctx, types.NamespacedName{Name: "vdc-quota", Namespace: namespaceName}, existingQuota)

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		// Create new quota
		if err := r.Create(ctx, desiredQuota); err != nil {
			return err
		}
		logger.Info("Created VDC resource quota", "namespace", namespaceName)
		return nil
	}

	// Check if update is needed (idempotent check)
	needsUpdate := false

	// Compare hard limits
	if !equality.Semantic.DeepEqual(existingQuota.Spec.Hard, desiredQuota.Spec.Hard) {
		existingQuota.Spec.Hard = desiredQuota.Spec.Hard
		needsUpdate = true
	}

	// Compare relevant labels
	if existingQuota.Labels == nil {
		existingQuota.Labels = make(map[string]string)
	}
	for key, value := range desiredQuota.Labels {
		if existingQuota.Labels[key] != value {
			existingQuota.Labels[key] = value
			needsUpdate = true
		}
	}

	// Only update if something actually changed
	if needsUpdate {
		if err := r.Update(ctx, existingQuota); err != nil {
			return err
		}
		logger.Info("Updated VDC resource quota", "namespace", namespaceName)
	}
	// No log if nothing changed - this prevents reconcile storm logs

	return nil
}

// ensureLimitRange creates or updates LimitRange for the VDC
func (r *VirtualDataCenterReconciler) ensureLimitRange(ctx context.Context, vdc *ovimv1.VirtualDataCenter, namespaceName string) error {
	logger := log.FromContext(ctx)

	desiredLimitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-limits",
			Namespace: namespaceName,
			Labels: map[string]string{
				"managed-by": "ovim",
				"type":       "vdc-limits",
				"org":        vdc.Spec.OrganizationRef,
				"vdc":        vdc.Name,
			},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{{
				Type: corev1.LimitTypeContainer,
				Min: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", vdc.Spec.LimitRange.MinCpu)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", vdc.Spec.LimitRange.MinMemory)),
				},
				Max: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", vdc.Spec.LimitRange.MaxCpu)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", vdc.Spec.LimitRange.MaxMemory)),
				},
			}},
		},
	}

	// Note: Cannot set cross-namespace owner references, using labels for tracking
	desiredLimitRange.Labels["ovim.io/vdc-id"] = vdc.Name
	desiredLimitRange.Labels["ovim.io/vdc-namespace"] = vdc.Namespace

	// Check if limit range already exists
	existingLimitRange := &corev1.LimitRange{}
	err := r.Get(ctx, types.NamespacedName{Name: "vdc-limits", Namespace: namespaceName}, existingLimitRange)

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		// Create new limit range
		if err := r.Create(ctx, desiredLimitRange); err != nil {
			return err
		}
		logger.Info("Created VDC limit range", "namespace", namespaceName)
		return nil
	}

	// Check if update is needed (idempotent check)
	needsUpdate := false

	// Compare limits
	if !equality.Semantic.DeepEqual(existingLimitRange.Spec.Limits, desiredLimitRange.Spec.Limits) {
		existingLimitRange.Spec.Limits = desiredLimitRange.Spec.Limits
		needsUpdate = true
	}

	// Compare relevant labels
	if existingLimitRange.Labels == nil {
		existingLimitRange.Labels = make(map[string]string)
	}
	for key, value := range desiredLimitRange.Labels {
		if existingLimitRange.Labels[key] != value {
			existingLimitRange.Labels[key] = value
			needsUpdate = true
		}
	}

	// Only update if something actually changed
	if needsUpdate {
		if err := r.Update(ctx, existingLimitRange); err != nil {
			return err
		}
		logger.Info("Updated VDC limit range", "namespace", namespaceName)
	}
	// No log if nothing changed - this prevents reconcile storm logs

	return nil
}

// setupVDCRBAC creates role bindings for VDC admins (inherits from org)
func (r *VirtualDataCenterReconciler) setupVDCRBAC(ctx context.Context, vdc *ovimv1.VirtualDataCenter, orgCR *ovimv1.Organization, namespaceName string) error {
	logger := log.FromContext(ctx)

	// Get existing bindings
	existingBindings := &rbacv1.RoleBindingList{}
	if err := r.List(ctx, existingBindings,
		client.InNamespace(namespaceName),
		client.MatchingLabels{"managed-by": "ovim", "type": "vdc-admin"}); err != nil {
		return err
	}

	// Create a map of current admin groups for easy lookup
	currentAdmins := make(map[string]bool)
	for _, admin := range orgCR.Spec.Admins {
		currentAdmins[admin] = true
	}

	// Create a map of existing bindings for comparison
	existingAdmins := make(map[string]*rbacv1.RoleBinding)
	for i := range existingBindings.Items {
		binding := &existingBindings.Items[i]
		if len(binding.Subjects) > 0 {
			adminGroup := binding.Subjects[0].Name
			existingAdmins[adminGroup] = binding
		}
	}

	changesNeeded := false

	// Remove bindings for admins that are no longer in the org
	for adminGroup, binding := range existingAdmins {
		if !currentAdmins[adminGroup] {
			if err := r.Delete(ctx, binding); err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "failed to delete obsolete role binding", "binding", binding.Name)
				return err
			}
			logger.Info("Removed obsolete admin binding", "admin", adminGroup)
			changesNeeded = true
		}
	}

	// Create or update bindings for current admins
	for _, adminGroup := range orgCR.Spec.Admins {
		bindingName := fmt.Sprintf("vdc-admin-%s", adminGroup)

		desiredBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: namespaceName,
				Labels: map[string]string{
					"managed-by":            "ovim",
					"type":                  "vdc-admin",
					"org":                   vdc.Spec.OrganizationRef,
					"vdc":                   vdc.Name,
					"ovim.io/vdc-id":        vdc.Name,
					"ovim.io/vdc-namespace": vdc.Namespace,
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

		existingBinding, exists := existingAdmins[adminGroup]
		if !exists {
			// Create new binding
			if err := r.Create(ctx, desiredBinding); err != nil {
				if errors.IsAlreadyExists(err) {
					// Handle race condition - another reconcile created it
					logger.V(4).Info("Role binding already exists (race condition)", "admin", adminGroup)
				} else {
					logger.Error(err, "failed to create role binding", "admin", adminGroup)
					return err
				}
			} else {
				logger.Info("Created admin binding", "admin", adminGroup)
				changesNeeded = true
			}
		} else {
			// Check if existing binding needs update
			needsUpdate := false

			// Compare subjects
			if !r.roleBindingsEqual(existingBinding, desiredBinding) {
				existingBinding.Subjects = desiredBinding.Subjects
				existingBinding.RoleRef = desiredBinding.RoleRef
				if existingBinding.Labels == nil {
					existingBinding.Labels = make(map[string]string)
				}
				for key, value := range desiredBinding.Labels {
					if existingBinding.Labels[key] != value {
						existingBinding.Labels[key] = value
						needsUpdate = true
					}
				}
				needsUpdate = true
			}

			if needsUpdate {
				if err := r.Update(ctx, existingBinding); err != nil {
					logger.Error(err, "failed to update role binding", "admin", adminGroup)
					return err
				}
				logger.Info("Updated admin binding", "admin", adminGroup)
				changesNeeded = true
			}
		}
	}

	// Only log if changes were actually made
	if changesNeeded {
		logger.Info("Set up VDC RBAC", "namespace", namespaceName, "admins", len(orgCR.Spec.Admins))
	}
	// No log if nothing changed - this prevents reconcile storm logs

	return nil
}

// roleBindingsEqual compares two role bindings for equality
func (r *VirtualDataCenterReconciler) roleBindingsEqual(a, b *rbacv1.RoleBinding) bool {
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
func (r *VirtualDataCenterReconciler) determineReconcileTrigger(ctx context.Context, req ctrl.Request) string {
	// Get the current VDC to analyze
	var vdc ovimv1.VirtualDataCenter
	if err := r.Get(ctx, req.NamespacedName, &vdc); err != nil {
		if errors.IsNotFound(err) {
			return "resource-deleted"
		}
		return "resource-fetch-error"
	}

	// Check if this is a deletion
	if vdc.DeletionTimestamp != nil {
		return "resource-deletion"
	}

	// Check if this is a new resource (recently created)
	if vdc.CreationTimestamp.Add(10 * time.Second).After(time.Now()) {
		return "resource-creation"
	}

	// Check if this is a new resource or has recent changes
	if vdc.Generation > 1 {
		return "spec-change"
	}

	// Check if this was triggered by deletion annotation (OVIM API deletion)
	if vdc.Annotations != nil {
		if deletionStatus, exists := vdc.Annotations["ovim.io/deletion-status"]; exists && deletionStatus == "pending" {
			if deletedAt, exists := vdc.Annotations["ovim.io/deleted-at"]; exists && deletedAt != "" {
				return "ovim-api-deletion"
			}
		}
	}

	// Check for status updates by looking at recent condition changes
	for _, condition := range vdc.Status.Conditions {
		if condition.LastTransitionTime.Add(30 * time.Second).After(time.Now()) {
			return fmt.Sprintf("condition-change-%s", condition.Type)
		}
	}

	// Check if metrics were recently updated (this could be triggering VDC reconciliation)
	if vdc.Status.LastMetricsUpdate != nil && vdc.Status.LastMetricsUpdate.Add(30*time.Second).After(time.Now()) {
		return "metrics-update-trigger"
	}

	// Check if this could be a requeue from a previous reconciliation
	if vdc.Status.LastReconcile != nil && vdc.Status.LastReconcile.Add(1*time.Minute).After(time.Now()) {
		return "periodic-requeue"
	}

	// Check for owner reference changes (parent resource updates)
	for _, ownerRef := range vdc.OwnerReferences {
		if ownerRef.Kind == "Organization" {
			// This could be triggered by organization changes
			return "owner-reference-change"
		}
	}

	// Check if there are any finalizers (could indicate cleanup operations)
	if len(vdc.Finalizers) > 0 && vdc.DeletionTimestamp == nil {
		return "finalizer-update"
	}

	// Default cases
	if vdc.Status.LastReconcile == nil {
		return "initial-reconcile"
	}

	return "periodic-reconcile"
}

// ensureNetworkPolicy creates or updates NetworkPolicy for the VDC
func (r *VirtualDataCenterReconciler) ensureNetworkPolicy(ctx context.Context, vdc *ovimv1.VirtualDataCenter, namespaceName string) error {
	logger := log.FromContext(ctx)

	// Skip if no network policy is specified or if it's default (no restriction)
	if vdc.Spec.NetworkPolicy == "" || vdc.Spec.NetworkPolicy == models.NetworkPolicyDefault {
		// Clean up any existing network policies
		return r.cleanupNetworkPolicies(ctx, namespaceName)
	}

	var networkPolicy *networkingv1.NetworkPolicy

	switch vdc.Spec.NetworkPolicy {
	case models.NetworkPolicyIsolated:
		networkPolicy = r.createIsolatedNetworkPolicy(vdc, namespaceName)
	case models.NetworkPolicyCustom:
		networkPolicy = r.createCustomNetworkPolicy(vdc, namespaceName)
	default:
		// Unknown policy type, log warning and use default (no policy)
		logger.Info("Unknown network policy type, skipping NetworkPolicy creation",
			"policy", vdc.Spec.NetworkPolicy, "vdc", vdc.Name)
		return r.cleanupNetworkPolicies(ctx, namespaceName)
	}

	if networkPolicy == nil {
		return fmt.Errorf("failed to create network policy for type: %s", vdc.Spec.NetworkPolicy)
	}

	// Check if network policy already exists
	existingPolicy := &networkingv1.NetworkPolicy{}
	err := r.Get(ctx, types.NamespacedName{Name: networkPolicy.Name, Namespace: namespaceName}, existingPolicy)

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		// Create new network policy
		if err := r.Create(ctx, networkPolicy); err != nil {
			return err
		}
		logger.Info("Created VDC network policy", "namespace", namespaceName, "policy", vdc.Spec.NetworkPolicy)
		return nil
	}

	// Check if update is needed (idempotent check)
	needsUpdate := false

	// Compare specs
	if !equality.Semantic.DeepEqual(existingPolicy.Spec, networkPolicy.Spec) {
		existingPolicy.Spec = networkPolicy.Spec
		needsUpdate = true
	}

	// Compare relevant labels
	if existingPolicy.Labels == nil {
		existingPolicy.Labels = make(map[string]string)
	}
	for key, value := range networkPolicy.Labels {
		if existingPolicy.Labels[key] != value {
			existingPolicy.Labels[key] = value
			needsUpdate = true
		}
	}

	// Compare relevant annotations
	if existingPolicy.Annotations == nil {
		existingPolicy.Annotations = make(map[string]string)
	}
	for key, value := range networkPolicy.Annotations {
		if existingPolicy.Annotations[key] != value {
			existingPolicy.Annotations[key] = value
			needsUpdate = true
		}
	}

	// Only update if something actually changed
	if needsUpdate {
		if err := r.Update(ctx, existingPolicy); err != nil {
			return err
		}
		logger.Info("Updated VDC network policy", "namespace", namespaceName, "policy", vdc.Spec.NetworkPolicy)
	}
	// No log if nothing changed - this prevents reconcile storm logs

	return nil
}

// createIsolatedNetworkPolicy creates a NetworkPolicy that isolates the VDC namespace
func (r *VirtualDataCenterReconciler) createIsolatedNetworkPolicy(vdc *ovimv1.VirtualDataCenter, namespaceName string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-isolation-policy",
			Namespace: namespaceName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "vdc",
				"app.kubernetes.io/managed-by": "ovim",
				"managed-by":                   "ovim",
				"type":                         "vdc-network-policy",
				"org":                          vdc.Spec.OrganizationRef,
				"vdc":                          vdc.Name,
				"ovim.io/vdc-id":               vdc.Name,
				"ovim.io/vdc-namespace":        vdc.Namespace,
				"ovim.io/policy-type":          "isolated",
			},
			Annotations: map[string]string{
				"ovim.io/vdc-description": vdc.Spec.Description,
				"ovim.io/created-by":      "ovim-controller",
				"ovim.io/created-at":      time.Now().Format(time.RFC3339),
				"ovim.io/policy-purpose":  "Isolate VDC traffic to same namespace only",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			// Apply to all pods in the namespace
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			// Ingress rules: Allow traffic from same namespace and system namespaces
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					// Allow traffic from pods in the same namespace
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"name": namespaceName,
								},
							},
						},
						// Allow traffic from system namespaces (kube-system, ovim-system)
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "name",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"kube-system", "ovim-system", "openshift-system", "openshift-monitoring"},
									},
								},
							},
						},
					},
				},
			},
			// Egress rules: Allow traffic to same namespace, system namespaces, and external
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// Allow traffic to pods in the same namespace
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"name": namespaceName,
								},
							},
						},
						// Allow traffic to system namespaces
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "name",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"kube-system", "ovim-system", "openshift-system", "openshift-monitoring"},
									},
								},
							},
						},
					},
				},
				{
					// Allow DNS traffic (port 53)
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolUDP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
						},
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolTCP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
						},
					},
				},
				{
					// Allow HTTPS traffic to external services (port 443)
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolTCP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 443},
						},
					},
					// Allow to external IPs (not same cluster)
					To: []networkingv1.NetworkPolicyPeer{},
				},
			},
		},
	}
}

// createCustomNetworkPolicy creates a NetworkPolicy based on custom configuration
func (r *VirtualDataCenterReconciler) createCustomNetworkPolicy(vdc *ovimv1.VirtualDataCenter, namespaceName string) *networkingv1.NetworkPolicy {
	// Start with base NetworkPolicy
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-custom-policy",
			Namespace: namespaceName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/component":  "vdc",
				"app.kubernetes.io/managed-by": "ovim",
				"managed-by":                   "ovim",
				"type":                         "vdc-network-policy",
				"org":                          vdc.Spec.OrganizationRef,
				"vdc":                          vdc.Name,
				"ovim.io/vdc-id":               vdc.Name,
				"ovim.io/vdc-namespace":        vdc.Namespace,
				"ovim.io/policy-type":          "custom",
			},
			Annotations: map[string]string{
				"ovim.io/vdc-description": vdc.Spec.Description,
				"ovim.io/created-by":      "ovim-controller",
				"ovim.io/created-at":      time.Now().Format(time.RFC3339),
				"ovim.io/policy-purpose":  "Custom VDC network policy from configuration",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		},
	}

	// Parse custom network configuration if available
	if vdc.Spec.CustomNetworkConfig != nil {
		// Apply custom configuration to policy spec
		r.applyCustomNetworkConfig(policy, vdc.Spec.CustomNetworkConfig)
	} else {
		// Default custom policy: Allow same namespace + basic external access
		policy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"name": namespaceName,
							},
						},
					},
				},
			},
		}
		policy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
			{
				// Allow DNS
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: &[]corev1.Protocol{corev1.ProtocolUDP}[0],
						Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
					},
				},
			},
		}
	}

	return policy
}

// applyCustomNetworkConfig applies custom network configuration to NetworkPolicy
func (r *VirtualDataCenterReconciler) applyCustomNetworkConfig(policy *networkingv1.NetworkPolicy, config map[string]string) {
	// This is a simplified implementation for map[string]string config.
	// In a production environment, you would want more sophisticated parsing and validation.

	// Example custom config structure (as string values):
	// {
	//   "allow_same_namespace": "true",
	//   "allow_dns": "true",
	//   "policy_type": "isolate" // isolate, allow-all, custom
	// }

	// Simple string-based configuration
	if policyType, ok := config["policy_type"]; ok {
		switch policyType {
		case "allow-all":
			// Remove all restrictions - empty ingress/egress rules allow all
			policy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{}}
			policy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{}}
		case "isolate":
			// Default isolation - only allow same namespace and DNS
			policy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"name": policy.Namespace},
							},
						},
					},
				},
			}
			policy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolUDP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
						},
					},
				},
			}
		}
	}

	// Handle deny all settings (string values)
	if denyAllIngress, ok := config["deny_all_ingress"]; ok && (denyAllIngress == "true" || denyAllIngress == "1") {
		policy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{} // Empty rules = deny all
	}
	if denyAllEgress, ok := config["deny_all_egress"]; ok && (denyAllEgress == "true" || denyAllEgress == "1") {
		policy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{} // Empty rules = deny all
	}
}

// cleanupNetworkPolicies removes any existing NetworkPolicy resources from the namespace
func (r *VirtualDataCenterReconciler) cleanupNetworkPolicies(ctx context.Context, namespaceName string) error {
	logger := log.FromContext(ctx)

	// List all NetworkPolicies managed by OVIM in the namespace
	policies := &networkingv1.NetworkPolicyList{}
	if err := r.List(ctx, policies,
		client.InNamespace(namespaceName),
		client.MatchingLabels{"managed-by": "ovim", "type": "vdc-network-policy"}); err != nil {
		return err
	}

	// Delete each policy
	for _, policy := range policies.Items {
		if err := r.Delete(ctx, &policy); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete network policy", "policy", policy.Name, "namespace", namespaceName)
			return err
		}
		logger.Info("Deleted network policy", "policy", policy.Name, "namespace", namespaceName)
	}

	return nil
}

// handleVDCDeletion handles VDC deletion with proper cleanup
func (r *VirtualDataCenterReconciler) handleVDCDeletion(ctx context.Context, vdc *ovimv1.VirtualDataCenter) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vdc", vdc.Name)

	// Clean up NetworkPolicies before deleting namespace
	if vdc.Status.Namespace != "" {
		if err := r.cleanupNetworkPolicies(ctx, vdc.Status.Namespace); err != nil {
			logger.Error(err, "unable to cleanup network policies")
			// Don't block deletion for NetworkPolicy cleanup issues
		}
	}

	// Clean up VDC resources before deleting namespace
	if vdc.Status.Namespace != "" {
		// First clean up individual VDC resources
		logger.Info("Starting cleanup of VDC resources", "namespace", vdc.Status.Namespace)
		if err := r.cleanupVDCResources(ctx, vdc.Status.Namespace, vdc.Name); err != nil {
			logger.Error(err, "Failed to cleanup VDC resources")
			// Continue with namespace deletion even if resource cleanup fails
		} else {
			logger.Info("Successfully completed cleanup of VDC resources", "namespace", vdc.Status.Namespace)
		}

		// Then delete the VDC namespace
		vdcNamespace := &corev1.Namespace{}
		err := r.Get(ctx, types.NamespacedName{Name: vdc.Status.Namespace}, vdcNamespace)
		if err == nil {
			if err := r.Delete(ctx, vdcNamespace); err != nil {
				logger.Error(err, "unable to delete VDC namespace")
				return ctrl.Result{}, err
			}
			logger.Info("Deleted VDC namespace", "namespace", vdc.Status.Namespace)
		} else if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Remove from database
	if r.Storage != nil {
		if err := r.Storage.DeleteVDC(vdc.Name); err != nil {
			// Only log error if it's not "not found" - VDC may not exist in DB
			if err != storage.ErrNotFound {
				logger.Error(err, "unable to delete VDC from database")
			}
			// Don't block deletion for database issues
		}
	}

	// Remove both hub and spoke finalizers (for same-cluster deployments)
	controllerutil.RemoveFinalizer(vdc, VDCFinalizer)
	controllerutil.RemoveFinalizer(vdc, "spokevdc.ovim.io/finalizer")
	if err := r.Update(ctx, vdc); err != nil {
		logger.Error(err, "unable to remove finalizers")
		return ctrl.Result{}, err
	}

	logger.Info("VDC deleted successfully")
	return ctrl.Result{}, nil
}

// syncVDCToDatabase synchronizes VDC data to the database
func (r *VirtualDataCenterReconciler) syncVDCToDatabase(ctx context.Context, vdc *ovimv1.VirtualDataCenter) error {
	if r.Storage == nil {
		return nil // No database configured
	}

	logger := log.FromContext(ctx)

	// Parse resource quotas
	cpuQuota, _ := parseResourceQuantity(vdc.Spec.Quota.CPU)
	memoryQuota, _ := parseResourceQuantity(vdc.Spec.Quota.Memory)
	storageQuota, _ := parseResourceQuantity(vdc.Spec.Quota.Storage)

	dbVDC := &models.VirtualDataCenter{
		ID:                vdc.Name,
		Name:              vdc.Spec.DisplayName,
		Description:       vdc.Spec.Description,
		OrgID:             vdc.Spec.OrganizationRef,
		CRName:            vdc.Name,
		CRNamespace:       vdc.Namespace,
		WorkloadNamespace: vdc.Status.Namespace,
		CPUQuota:          cpuQuota,
		MemoryQuota:       memoryQuota,
		StorageQuota:      storageQuota,
		NetworkPolicy:     vdc.Spec.NetworkPolicy,
		Phase:             string(vdc.Status.Phase),
	}

	// Add LimitRange if specified
	if vdc.Spec.LimitRange != nil {
		dbVDC.MinCPU = &vdc.Spec.LimitRange.MinCpu
		dbVDC.MaxCPU = &vdc.Spec.LimitRange.MaxCpu
		dbVDC.MinMemory = &vdc.Spec.LimitRange.MinMemory
		dbVDC.MaxMemory = &vdc.Spec.LimitRange.MaxMemory
	}

	// Check if VDC exists in database
	existingVDC, err := r.Storage.GetVDC(vdc.Name)
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	if err == storage.ErrNotFound {
		// Create new VDC
		if err := r.Storage.CreateVDC(dbVDC); err != nil {
			return err
		}
		logger.Info("Created VDC in database", "vdc", vdc.Name)
	} else {
		// Check if update is needed (compare relevant fields)
		needsUpdate := false

		if existingVDC.Name != dbVDC.Name ||
			existingVDC.Description != dbVDC.Description ||
			existingVDC.OrgID != dbVDC.OrgID ||
			existingVDC.WorkloadNamespace != dbVDC.WorkloadNamespace ||
			existingVDC.CPUQuota != dbVDC.CPUQuota ||
			existingVDC.MemoryQuota != dbVDC.MemoryQuota ||
			existingVDC.StorageQuota != dbVDC.StorageQuota ||
			existingVDC.NetworkPolicy != dbVDC.NetworkPolicy ||
			existingVDC.Phase != dbVDC.Phase {
			needsUpdate = true
		}

		// Check LimitRange fields
		if !needsUpdate {
			if (existingVDC.MinCPU == nil) != (dbVDC.MinCPU == nil) ||
				(existingVDC.MaxCPU == nil) != (dbVDC.MaxCPU == nil) ||
				(existingVDC.MinMemory == nil) != (dbVDC.MinMemory == nil) ||
				(existingVDC.MaxMemory == nil) != (dbVDC.MaxMemory == nil) {
				needsUpdate = true
			} else {
				if existingVDC.MinCPU != nil && dbVDC.MinCPU != nil && *existingVDC.MinCPU != *dbVDC.MinCPU {
					needsUpdate = true
				}
				if existingVDC.MaxCPU != nil && dbVDC.MaxCPU != nil && *existingVDC.MaxCPU != *dbVDC.MaxCPU {
					needsUpdate = true
				}
				if existingVDC.MinMemory != nil && dbVDC.MinMemory != nil && *existingVDC.MinMemory != *dbVDC.MinMemory {
					needsUpdate = true
				}
				if existingVDC.MaxMemory != nil && dbVDC.MaxMemory != nil && *existingVDC.MaxMemory != *dbVDC.MaxMemory {
					needsUpdate = true
				}
			}
		}

		// Only update if something actually changed
		if needsUpdate {
			if err := r.Storage.UpdateVDC(dbVDC); err != nil {
				return err
			}
			logger.Info("Updated VDC in database", "vdc", vdc.Name)
		}
		// No log if nothing changed - this prevents reconcile storm logs
	}

	return nil
}

// updateVDCCondition updates a condition in the VDC status only if something changed
func (r *VirtualDataCenterReconciler) updateVDCCondition(vdc *ovimv1.VirtualDataCenter, conditionType string, status metav1.ConditionStatus, reason, message string) {
	// Find existing condition and only update if something actually changed
	for i, existing := range vdc.Status.Conditions {
		if existing.Type == conditionType {
			// Only update if status, reason, or message changed
			if existing.Status != status || existing.Reason != reason || existing.Message != message {
				existing.Status = status
				existing.Reason = reason
				existing.Message = message
				existing.LastTransitionTime = metav1.Now() // Only update timestamp when values change
				vdc.Status.Conditions[i] = existing
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
	vdc.Status.Conditions = append(vdc.Status.Conditions, condition)
}

// shouldUpdateVDCCondition checks if a condition needs updating (helper for idempotency checks)
func (r *VirtualDataCenterReconciler) shouldUpdateVDCCondition(vdc *ovimv1.VirtualDataCenter, conditionType string, status metav1.ConditionStatus, reason, message string) bool {
	for _, existing := range vdc.Status.Conditions {
		if existing.Type == conditionType {
			return existing.Status != status || existing.Reason != reason || existing.Message != message
		}
	}
	return true // Condition doesn't exist, needs to be added
}

// parseResourceQuantity parses a resource quantity string and returns the numeric value in decimal units
func parseResourceQuantity(quantityStr string) (int, error) {
	quantity, err := resource.ParseQuantity(quantityStr)
	if err != nil {
		return 0, err
	}

	// Convert to decimal base units (cores for CPU, GB for memory/storage)
	// All conversions get the value in bytes first, then convert to GB
	bytesValue := quantity.Value()

	switch {
	case strings.Contains(quantityStr, "Gi"):
		// Binary Gibibytes - already in bytes, convert to decimal GB
		// 1 byte = 1/(1000^3) GB in decimal
		return int(bytesValue / (1000 * 1000 * 1000)), nil
	case strings.Contains(quantityStr, "GB"):
		// Decimal Gigabytes - get GB value directly
		return int(quantity.ScaledValue(resource.Giga)), nil
	case strings.Contains(quantityStr, "Ti"):
		// Binary Tebibytes - already in bytes, convert to decimal GB
		return int(bytesValue / (1000 * 1000 * 1000)), nil
	case strings.Contains(quantityStr, "TB"):
		// Decimal Terabytes - convert to GB
		return int(quantity.ScaledValue(resource.Tera) / 1000), nil
	case strings.Contains(quantityStr, "Mi"):
		// Binary Mebibytes - already in bytes, convert to decimal GB
		return int(bytesValue / (1000 * 1000 * 1000)), nil
	case strings.Contains(quantityStr, "MB"):
		// Decimal Megabytes - convert to GB
		return int(quantity.ScaledValue(resource.Mega) / 1000), nil
	default:
		// CPU cores (no unit or just numbers)
		return int(quantity.Value()), nil
	}
}

// cleanupVDCResources cleans up all VDC-related resources from the workload namespace
func (r *VirtualDataCenterReconciler) cleanupVDCResources(ctx context.Context, namespace, vdcName string) error {
	logger := log.FromContext(ctx).WithValues("namespace", namespace, "vdc", vdcName)
	logger.Info("Cleaning up VDC resources")

	// Clean up ResourceQuota
	resourceQuota := &corev1.ResourceQuota{}
	err := r.Get(ctx, types.NamespacedName{Name: "vdc-quota", Namespace: namespace}, resourceQuota)
	if err == nil {
		if err := r.Delete(ctx, resourceQuota); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete ResourceQuota")
			return fmt.Errorf("failed to delete ResourceQuota: %w", err)
		}
		logger.Info("Deleted ResourceQuota vdc-quota")
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get ResourceQuota: %w", err)
	}

	// Clean up LimitRange
	limitRange := &corev1.LimitRange{}
	err = r.Get(ctx, types.NamespacedName{Name: "vdc-limits", Namespace: namespace}, limitRange)
	if err == nil {
		if err := r.Delete(ctx, limitRange); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete LimitRange")
			return fmt.Errorf("failed to delete LimitRange: %w", err)
		}
		logger.Info("Deleted LimitRange vdc-limits")
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get LimitRange: %w", err)
	}

	// Clean up RoleBindings (VDC admin bindings)
	roleBindingList := &rbacv1.RoleBindingList{}
	if err := r.List(ctx, roleBindingList,
		client.InNamespace(namespace),
		client.MatchingLabels{"managed-by": "ovim", "type": "vdc-admin", "vdc": vdcName}); err != nil {
		return fmt.Errorf("failed to list RoleBindings: %w", err)
	}

	for _, binding := range roleBindingList.Items {
		if err := r.Delete(ctx, &binding); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete RoleBinding", "binding", binding.Name)
			return fmt.Errorf("failed to delete RoleBinding %s: %w", binding.Name, err)
		}
		logger.Info("Deleted RoleBinding", "binding", binding.Name)
	}

	// Clean up NetworkPolicies
	networkPolicyList := &networkingv1.NetworkPolicyList{}
	if err := r.List(ctx, networkPolicyList,
		client.InNamespace(namespace),
		client.MatchingLabels{"managed-by": "ovim", "type": "vdc-network-policy"}); err != nil {
		return fmt.Errorf("failed to list NetworkPolicies: %w", err)
	}

	for _, policy := range networkPolicyList.Items {
		if err := r.Delete(ctx, &policy); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete NetworkPolicy", "policy", policy.Name)
			return fmt.Errorf("failed to delete NetworkPolicy %s: %w", policy.Name, err)
		}
		logger.Info("Deleted NetworkPolicy", "policy", policy.Name)
	}

	// Clean up any ServiceAccounts created for this VDC
	serviceAccountList := &corev1.ServiceAccountList{}
	if err := r.List(ctx, serviceAccountList,
		client.InNamespace(namespace),
		client.MatchingLabels{"ovim.io/vdc": vdcName}); err != nil {
		return fmt.Errorf("failed to list ServiceAccounts: %w", err)
	}

	for _, sa := range serviceAccountList.Items {
		if err := r.Delete(ctx, &sa); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete ServiceAccount", "serviceAccount", sa.Name)
			return fmt.Errorf("failed to delete ServiceAccount %s: %w", sa.Name, err)
		}
		logger.Info("Deleted ServiceAccount", "serviceAccount", sa.Name)
	}

	logger.Info("Successfully cleaned up all VDC resources")
	return nil
}

// TriggerSpokeDeletion sets the hub deletion annotation to trigger VDC deletion on spoke
func (r *VirtualDataCenterReconciler) TriggerSpokeDeletion(ctx context.Context, vdc *ovimv1.VirtualDataCenter) error {
	logger := log.FromContext(ctx).WithValues("vdc", vdc.Name, "namespace", vdc.Namespace)

	// Ensure annotations map exists
	if vdc.Annotations == nil {
		vdc.Annotations = make(map[string]string)
	}

	// Set the hub deletion annotation
	vdc.Annotations[HubDeletionAnnotation] = HubDeletionValue
	vdc.Annotations["ovim.io/deletion-initiated-by"] = "hub-server"
	vdc.Annotations["ovim.io/deletion-initiated-at"] = time.Now().Format(time.RFC3339)

	// Update the VDC to trigger spoke deletion
	if err := r.Update(ctx, vdc); err != nil {
		logger.Error(err, "Failed to set hub deletion annotation")
		return fmt.Errorf("failed to trigger spoke deletion: %w", err)
	}

	logger.Info("Successfully triggered VDC deletion on spoke cluster")
	return nil
}

// deletionAnnotationPredicate triggers reconciliation when deletion annotations are added/changed
func deletionAnnotationPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, oldOK := e.ObjectOld.(*ovimv1.VirtualDataCenter)
			newObj, newOK := e.ObjectNew.(*ovimv1.VirtualDataCenter)

			if !oldOK || !newOK {
				return true // Fall back to default behavior for non-VDC objects
			}

			// Check if deletion annotations were added or changed
			oldDeletionStatus := ""
			newDeletionStatus := ""

			if oldObj.Annotations != nil {
				oldDeletionStatus = oldObj.Annotations["ovim.io/deletion-status"]
			}
			if newObj.Annotations != nil {
				newDeletionStatus = newObj.Annotations["ovim.io/deletion-status"]
			}

			// Trigger immediate reconciliation if deletion-status annotation was added or changed to "pending"
			if oldDeletionStatus != newDeletionStatus && newDeletionStatus == "pending" {
				return true
			}

			// Also trigger on deletionTimestamp changes (normal Kubernetes deletion)
			if (oldObj.DeletionTimestamp == nil) != (newObj.DeletionTimestamp == nil) {
				return true
			}

			// Trigger on generation changes (spec updates)
			if oldObj.Generation != newObj.Generation {
				return true
			}

			// Trigger on finalizer changes
			if len(oldObj.Finalizers) != len(newObj.Finalizers) {
				return true
			}

			// Default to not reconciling for other annotation-only changes
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true // Always reconcile on create
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true // Always reconcile on delete
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true // Always reconcile on generic events
		},
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *VirtualDataCenterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.VirtualDataCenter{}).
		// Add custom predicate to trigger immediate reconciliation on deletion annotations
		WithEventFilter(deletionAnnotationPredicate()).
		// Only watch VDC resources directly to avoid reconciliation loops
		// Child resources are managed but not watched to prevent conflicts
		// with spoke agents that may also modify these resources
		Named("ovim-vdc-controller").
		Complete(r)
}
