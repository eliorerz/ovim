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
	Scheme        *runtime.Scheme
	Storage       storage.Storage
	SpokeHandlers *SpokeHandlers // Reference to spoke handlers for operation sending
}

// SpokeHandlers interface to avoid import cycles
type SpokeHandlers interface {
	QueueVDCCreation(agentID string, vdc *ovimv1.VirtualDataCenter) error
	QueueVDCDeletion(agentID, vdcName, vdcNamespace, targetNamespace string) error
	GetZoneAgentStatus(zoneID string) interface{}
}

// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters/finalizers,verbs=update
// +kubebuilder:rbac:groups=ovim.io,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// ResourceQuotas and LimitRanges are now managed by spoke agents
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch
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
	logger.Info("Starting VDC reconciliation", "name", req.Name, "namespace", req.Namespace)

	// Fetch the VirtualDataCenter instance
	var vdc ovimv1.VirtualDataCenter
	if err := r.Get(ctx, req.NamespacedName, &vdc); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("VDC not found, assuming deleted", "name", req.Name, "namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch VirtualDataCenter")
		return ctrl.Result{}, err
	}

	logger.Info("Found VDC for reconciliation",
		"vdc_name", vdc.Name,
		"organization", vdc.Spec.OrganizationRef,
		"zone_id", vdc.Spec.ZoneID,
		"phase", vdc.Status.Phase,
		"generation", vdc.Generation,
		"observed_generation", vdc.Status.ObservedGeneration)

	// Handle deletion
	if vdc.DeletionTimestamp != nil {
		logger.Info("VDC marked for deletion, processing cleanup", "deletion_timestamp", vdc.DeletionTimestamp)
		return r.handleVDCDeletion(ctx, &vdc)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&vdc, VDCFinalizer) {
		logger.Info("Adding finalizer to VDC", "finalizer", VDCFinalizer)
		controllerutil.AddFinalizer(&vdc, VDCFinalizer)
		if err := r.Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to add finalizer")
			return ctrl.Result{}, err
		}
		logger.Info("Successfully added finalizer to VDC")
		return ctrl.Result{}, nil
	}

	// Get parent organization
	logger.Info("Looking up parent organization", "organization", vdc.Spec.OrganizationRef)
	orgCR := &ovimv1.Organization{}
	if err := r.Get(ctx, types.NamespacedName{Name: vdc.Spec.OrganizationRef}, orgCR); err != nil {
		logger.Error(err, "unable to get parent organization", "organization", vdc.Spec.OrganizationRef)
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "OrganizationNotFound",
			fmt.Sprintf("Parent organization %s not found", vdc.Spec.OrganizationRef))
		if err := r.Status().Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to update status")
		}
		logger.Info("Requeuing VDC reconciliation due to missing organization", "requeue_after", "30s")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	logger.Info("Found parent organization", "organization", vdc.Spec.OrganizationRef, "org_display_name", orgCR.Spec.DisplayName)

	// Create VDC workload namespace - ensure uniqueness across organizations
	// Format: vdc-{org}-{vdc-name}
	// Since VDC names are unique within each organization namespace, this ensures global uniqueness
	vdcNamespace := fmt.Sprintf("vdc-%s-%s",
		strings.ToLower(vdc.Spec.OrganizationRef),
		strings.ToLower(vdc.Name))

	logger.Info("Ensuring VDC workload namespace", "target_namespace", vdcNamespace)
	if err := r.ensureVDCNamespace(ctx, &vdc, vdcNamespace); err != nil {
		logger.Error(err, "unable to ensure VDC namespace", "target_namespace", vdcNamespace)
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "NamespaceCreationFailed", err.Error())
		if err := r.Status().Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to update status")
		}
		logger.Info("Requeuing VDC reconciliation due to namespace creation failure", "requeue_after", "30s")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	logger.Info("VDC workload namespace ready", "namespace", vdcNamespace)

	// Replicate VDC to spoke cluster for actual resource creation
	// ResourceQuota and LimitRange will be created by spoke agents
	logger.Info("Initiating VDC replication to spoke cluster", "zone_id", vdc.Spec.ZoneID, "target_namespace", vdcNamespace)
	if err := r.replicateVDCToSpoke(ctx, &vdc, vdcNamespace); err != nil {
		logger.Error(err, "unable to replicate VDC to spoke cluster", "zone_id", vdc.Spec.ZoneID, "target_namespace", vdcNamespace)
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "SpokeReplicationFailed", err.Error())
		if err := r.Status().Update(ctx, &vdc); err != nil {
			logger.Error(err, "unable to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
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

	// Update status - check if spoke VDC is present
	vdc.Status.Namespace = vdcNamespace

	// Check for spoke VDC presence before marking as Active
	spokeVDCPresent, spokeStatus, err := r.checkSpokeVDCStatus(ctx, &vdc, vdcNamespace)
	if err != nil {
		logger.Error(err, "unable to check spoke VDC status")
		vdc.Status.Phase = ovimv1.VirtualDataCenterPhaseWaitingForSpoke
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "SpokeStatusCheckFailed", err.Error())
	} else if !spokeVDCPresent {
		logger.Info("VDC waiting for spoke VDC deployment", "zone_id", vdc.Spec.ZoneID, "target_namespace", vdcNamespace)
		vdc.Status.Phase = ovimv1.VirtualDataCenterPhaseWaitingForSpoke
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionFalse, "WaitingForSpoke", "Waiting for spoke VDC to be created in target zone")

		// Initialize spoke deployment status
		vdc.Status.SpokeDeploymentStatus = &ovimv1.SpokeDeploymentStatus{
			TotalSpokes:    1, // For now, assume 1 spoke per zone
			DeployedSpokes: 0,
			HealthySpokes:  0,
			FailedSpokes:   0,
			LastSyncTime:   &metav1.Time{Time: time.Now()},
		}
	} else {
		logger.Info("VDC spoke deployment confirmed active", "zone_id", vdc.Spec.ZoneID, "spoke_status", spokeStatus)
		vdc.Status.Phase = ovimv1.VirtualDataCenterPhaseActive
		r.updateVDCCondition(&vdc, ConditionReady, metav1.ConditionTrue, "VDCReady", "VDC is ready and active with healthy spoke deployment")

		// Update spoke deployment status
		vdc.Status.SpokeDeploymentStatus = &ovimv1.SpokeDeploymentStatus{
			TotalSpokes:    1,
			DeployedSpokes: 1,
			HealthySpokes:  1,
			FailedSpokes:   0,
			LastSyncTime:   &metav1.Time{Time: time.Now()},
		}
	}

	if err := r.Status().Update(ctx, &vdc); err != nil {
		logger.Error(err, "unable to update status")
		return ctrl.Result{}, err
	}

	// Sync to database
	if err := r.syncVDCToDatabase(ctx, &vdc); err != nil {
		logger.Error(err, "unable to sync VDC to database")
		// Don't fail reconciliation for database sync issues
	}

	logger.Info("VDC reconciliation completed successfully",
		"vdc_name", vdc.Name,
		"organization", vdc.Spec.OrganizationRef,
		"zone_id", vdc.Spec.ZoneID,
		"phase", vdc.Status.Phase,
		"namespace", vdc.Status.Namespace)

	// If VDC is waiting for spoke deployment, requeue to check again
	if vdc.Status.Phase == ovimv1.VirtualDataCenterPhaseWaitingForSpoke {
		logger.Info("Requeuing VDC reconciliation to check for spoke deployment", "requeue_after", "30s")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

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

// replicateVDCToSpoke replicates the VDC resource to the target spoke cluster
// The spoke agent will handle ResourceQuota and LimitRange creation
func (r *VirtualDataCenterReconciler) replicateVDCToSpoke(ctx context.Context, vdc *ovimv1.VirtualDataCenter, namespaceName string) error {
	logger := log.FromContext(ctx)

	// Skip replication for spoke VDCs to prevent infinite loops
	if vdc.Spec.VDCType == ovimv1.VDCTypeSpoke {
		logger.V(1).Info("Skipping replication for spoke VDC", "vdc_name", vdc.Name, "vdc_type", vdc.Spec.VDCType)
		return nil
	}

	if vdc.Annotations == nil {
		vdc.Annotations = make(map[string]string)
	}

	// Mark VDC as requiring spoke cluster replication (for monitoring)
	vdc.Annotations["ovim.io/spoke-replication-required"] = "true"
	vdc.Annotations["ovim.io/target-zone"] = vdc.Spec.ZoneID
	vdc.Annotations["ovim.io/target-namespace"] = namespaceName
	vdc.Annotations["ovim.io/replication-timestamp"] = time.Now().Format(time.RFC3339)

	// Update the VDC with replication annotations first
	if err := r.Update(ctx, vdc); err != nil {
		return fmt.Errorf("failed to add spoke replication annotations: %w", err)
	}

	// Use SpokeHandlers to directly send operation to spoke agent
	if r.SpokeHandlers != nil {
		// Find spoke agent for this zone
		agentStatus := (*r.SpokeHandlers).GetZoneAgentStatus(vdc.Spec.ZoneID)
		if agentStatus != nil {
			// Extract agent ID from status
			if statusMap, ok := agentStatus.(map[string]interface{}); ok {
				if agentID, ok := statusMap["agent_id"].(string); ok {
					logger.Info("Sending VDC creation operation to spoke agent",
						"vdc_name", vdc.Name,
						"zone_id", vdc.Spec.ZoneID,
						"agent_id", agentID,
						"target_namespace", namespaceName)

					// Queue VDC creation operation
					if err := (*r.SpokeHandlers).QueueVDCCreation(agentID, vdc); err != nil {
						logger.Error(err, "Failed to queue VDC creation operation", "agent_id", agentID)
						return fmt.Errorf("failed to queue VDC creation operation: %w", err)
					}

					logger.Info("Successfully queued VDC creation operation",
						"vdc_name", vdc.Name,
						"zone_id", vdc.Spec.ZoneID,
						"agent_id", agentID,
						"target_namespace", namespaceName)
				} else {
					logger.V(1).Info("Could not extract agent ID from status", "zone_id", vdc.Spec.ZoneID)
				}
			} else {
				logger.V(1).Info("Agent status not in expected format", "zone_id", vdc.Spec.ZoneID)
			}
		} else {
			logger.Info("No spoke agent found for zone, operation will be processed by monitoring",
				"zone_id", vdc.Spec.ZoneID,
				"vdc_name", vdc.Name)
		}
	} else {
		logger.Info("SpokeHandlers not available, VDC replication will be handled by monitoring",
			"vdc_name", vdc.Name,
			"zone_id", vdc.Spec.ZoneID)
	}

	logger.Info("VDC replication initiated",
		"vdc_name", vdc.Name,
		"zone_id", vdc.Spec.ZoneID,
		"target_namespace", namespaceName)

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

	// Signal spoke agents for cascade deletion
	// Add deletion annotation so spoke agents can clean up ResourceQuota, LimitRange, and VMs
	if vdc.Annotations == nil {
		vdc.Annotations = make(map[string]string)
	}

	if _, exists := vdc.Annotations["ovim.io/spoke-deletion-required"]; !exists {
		vdc.Annotations["ovim.io/spoke-deletion-required"] = "true"
		vdc.Annotations["ovim.io/deletion-timestamp"] = time.Now().Format(time.RFC3339)

		if err := r.Update(ctx, vdc); err != nil {
			logger.Error(err, "failed to add spoke deletion annotations")
			return ctrl.Result{}, err
		}

		logger.Info("Marked VDC for spoke cluster deletion", "vdc", vdc.Name, "zone", vdc.Spec.ZoneID)

		// Requeue after a short delay to allow spoke agents to process deletion
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Clean up NetworkPolicies before deleting namespace
	if vdc.Status.Namespace != "" {
		if err := r.cleanupNetworkPolicies(ctx, vdc.Status.Namespace); err != nil {
			logger.Error(err, "unable to cleanup network policies")
			// Don't block deletion for NetworkPolicy cleanup issues
		}
	}

	// Delete VDC namespace if it exists (hub namespace only - spoke cleanup is handled by agents)
	if vdc.Status.Namespace != "" {
		vdcNamespace := &corev1.Namespace{}
		err := r.Get(ctx, types.NamespacedName{Name: vdc.Status.Namespace}, vdcNamespace)
		if err == nil {
			if err := r.Delete(ctx, vdcNamespace); err != nil {
				logger.Error(err, "unable to delete VDC namespace")
				return ctrl.Result{}, err
			}
			logger.Info("Deleted VDC namespace on hub", "namespace", vdc.Status.Namespace)
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

// checkSpokeVDCStatus checks if a corresponding spoke VDC exists and is healthy
func (r *VirtualDataCenterReconciler) checkSpokeVDCStatus(ctx context.Context, vdc *ovimv1.VirtualDataCenter, targetNamespace string) (bool, string, error) {
	logger := log.FromContext(ctx)

	// Look for spoke VDCs that reference this hub VDC
	spokeVDCs := &ovimv1.VirtualDataCenterList{}
	if err := r.List(ctx, spokeVDCs); err != nil {
		return false, "", fmt.Errorf("failed to list VDCs: %w", err)
	}

	// Check for spoke VDCs that reference this hub VDC
	for _, spokeVDC := range spokeVDCs.Items {
		// Check if this is a spoke VDC that references our hub VDC
		if spokeVDC.Spec.VDCType == ovimv1.VDCTypeSpoke &&
			spokeVDC.Spec.HubVDCRef != nil &&
			spokeVDC.Spec.HubVDCRef.Name == vdc.Name &&
			spokeVDC.Spec.HubVDCRef.Namespace == vdc.Namespace &&
			spokeVDC.Spec.HubVDCRef.ZoneID == vdc.Spec.ZoneID {

			logger.Info("Found spoke VDC",
				"spoke_vdc", spokeVDC.Name,
				"spoke_namespace", spokeVDC.Namespace,
				"spoke_phase", spokeVDC.Status.Phase,
				"hub_vdc", vdc.Name)

			// Check if spoke VDC is active
			if spokeVDC.Status.Phase == ovimv1.VirtualDataCenterPhaseActive {
				return true, "active", nil
			} else {
				return true, string(spokeVDC.Status.Phase), nil
			}
		}
	}

	// Also check via spoke API if available (fallback approach)
	// This could be implemented later to query spoke agents directly

	logger.V(1).Info("No spoke VDC found", "hub_vdc", vdc.Name, "target_zone", vdc.Spec.ZoneID)
	return false, "not_found", nil
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
		// ResourceQuota and LimitRange are now handled by spoke agents.
		Owns(&rbacv1.RoleBinding{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Named("ovim-vdc-controller").
		Complete(r)
}
