package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

const (
	// VDCFinalizer is the finalizer for VirtualDataCenter resources
	VDCFinalizer = "ovim.io/vdc-finalizer"
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

	// Fetch the VirtualDataCenter instance
	var vdc ovimv1.VirtualDataCenter
	if err := r.Get(ctx, req.NamespacedName, &vdc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch VirtualDataCenter")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if vdc.DeletionTimestamp != nil {
		return r.handleVDCDeletion(ctx, &vdc)
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

	// Update status
	vdc.Status.Namespace = vdcNamespace
	vdc.Status.Phase = ovimv1.VirtualDataCenterPhaseActive
	r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionTrue, "VDCReady", "VDC is ready and active")

	if err := r.Status().Update(ctx, &vdc); err != nil {
		logger.Error(err, "unable to update status")
		return ctrl.Result{}, err
	}

	// Sync to database
	if err := r.syncVDCToDatabase(ctx, &vdc); err != nil {
		logger.Error(err, "unable to sync VDC to database")
		// Don't fail reconciliation for database sync issues
	}

	logger.Info("VDC reconciled successfully")
	return ctrl.Result{}, nil
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

	quota := &corev1.ResourceQuota{
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
	quota.Labels["ovim.io/vdc-id"] = vdc.Name
	quota.Labels["ovim.io/vdc-namespace"] = vdc.Namespace

	// Try to create, if exists, update
	if err := r.Create(ctx, quota); err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing quota
			existingQuota := &corev1.ResourceQuota{}
			if err := r.Get(ctx, types.NamespacedName{Name: "vdc-quota", Namespace: namespaceName}, existingQuota); err != nil {
				return err
			}

			existingQuota.Spec.Hard = quota.Spec.Hard
			if err := r.Update(ctx, existingQuota); err != nil {
				return err
			}
			logger.Info("Updated VDC resource quota", "namespace", namespaceName)
		} else {
			return err
		}
	} else {
		logger.Info("Created VDC resource quota", "namespace", namespaceName)
	}

	return nil
}

// ensureLimitRange creates or updates LimitRange for the VDC
func (r *VirtualDataCenterReconciler) ensureLimitRange(ctx context.Context, vdc *ovimv1.VirtualDataCenter, namespaceName string) error {
	logger := log.FromContext(ctx)

	limitRange := &corev1.LimitRange{
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
	limitRange.Labels["ovim.io/vdc-id"] = vdc.Name
	limitRange.Labels["ovim.io/vdc-namespace"] = vdc.Namespace

	// Try to create, if exists, update
	if err := r.Create(ctx, limitRange); err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing limit range
			existingLimitRange := &corev1.LimitRange{}
			if err := r.Get(ctx, types.NamespacedName{Name: "vdc-limits", Namespace: namespaceName}, existingLimitRange); err != nil {
				return err
			}

			existingLimitRange.Spec.Limits = limitRange.Spec.Limits
			if err := r.Update(ctx, existingLimitRange); err != nil {
				return err
			}
			logger.Info("Updated VDC limit range", "namespace", namespaceName)
		} else {
			return err
		}
	} else {
		logger.Info("Created VDC limit range", "namespace", namespaceName)
	}

	return nil
}

// setupVDCRBAC creates role bindings for VDC admins (inherits from org)
func (r *VirtualDataCenterReconciler) setupVDCRBAC(ctx context.Context, vdc *ovimv1.VirtualDataCenter, orgCR *ovimv1.Organization, namespaceName string) error {
	logger := log.FromContext(ctx)

	// Clean up existing bindings first
	existingBindings := &rbacv1.RoleBindingList{}
	if err := r.List(ctx, existingBindings,
		client.InNamespace(namespaceName),
		client.MatchingLabels{"managed-by": "ovim", "type": "vdc-admin"}); err != nil {
		return err
	}

	for _, binding := range existingBindings.Items {
		if err := r.Delete(ctx, &binding); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete existing role binding", "binding", binding.Name)
		}
	}

	// Create RoleBindings for org admins in VDC namespace
	for _, adminGroup := range orgCR.Spec.Admins {
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("vdc-admin-%s", adminGroup),
				Namespace: namespaceName,
				Labels: map[string]string{
					"managed-by": "ovim",
					"type":       "vdc-admin",
					"org":        vdc.Spec.OrganizationRef,
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

		if err := r.Create(ctx, roleBinding); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	logger.Info("Set up VDC RBAC", "namespace", namespaceName, "admins", len(orgCR.Spec.Admins))
	return nil
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

	// Try to create, if exists, update
	if err := r.Create(ctx, networkPolicy); err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing network policy
			existingPolicy := &networkingv1.NetworkPolicy{}
			if err := r.Get(ctx, types.NamespacedName{Name: networkPolicy.Name, Namespace: namespaceName}, existingPolicy); err != nil {
				return err
			}

			existingPolicy.Spec = networkPolicy.Spec
			existingPolicy.Labels = networkPolicy.Labels
			existingPolicy.Annotations = networkPolicy.Annotations

			if err := r.Update(ctx, existingPolicy); err != nil {
				return err
			}
			logger.Info("Updated VDC network policy", "namespace", namespaceName, "policy", vdc.Spec.NetworkPolicy)
		} else {
			return err
		}
	} else {
		logger.Info("Created VDC network policy", "namespace", namespaceName, "policy", vdc.Spec.NetworkPolicy)
	}

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

	// Delete VDC namespace if it exists
	if vdc.Status.Namespace != "" {
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
			logger.Error(err, "unable to delete VDC from database")
			// Don't block deletion for database issues
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(vdc, VDCFinalizer)
	if err := r.Update(ctx, vdc); err != nil {
		logger.Error(err, "unable to remove finalizer")
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
	_, err := r.Storage.GetVDC(vdc.Name)
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
		// Update existing VDC
		if err := r.Storage.UpdateVDC(dbVDC); err != nil {
			return err
		}
		logger.Info("Updated VDC in database", "vdc", vdc.Name)
	}

	return nil
}

// updateVDCCondition updates a condition in the VDC status
func (r *VirtualDataCenterReconciler) updateVDCCondition(vdc *ovimv1.VirtualDataCenter, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Find existing condition and update it, or append new one
	for i, existing := range vdc.Status.Conditions {
		if existing.Type == conditionType {
			vdc.Status.Conditions[i] = condition
			return
		}
	}

	vdc.Status.Conditions = append(vdc.Status.Conditions, condition)
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

// SetupWithManager sets up the controller with the Manager
func (r *VirtualDataCenterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.VirtualDataCenter{}).
		// Note: Cannot use Owns() for cluster-scoped resources like Namespace
		// due to cross-scope ownership restrictions. Using labels for tracking.
		Owns(&corev1.ResourceQuota{}).
		Owns(&corev1.LimitRange{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Named("ovim-vdc-controller").
		Complete(r)
}
