package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/spoke"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// HubDeletionAnnotation is set by the hub server to trigger VDC deletion on spoke
	HubDeletionAnnotation = "ovim.io/hub-delete-requested"
	// HubDeletionValue is the value set when hub requests deletion
	HubDeletionValue = "true"
)

// SpokeVDCReconciler reconciles VirtualDataCenter objects on spoke clusters
type SpokeVDCReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	K8sClient    kubernetes.Interface
	HubClient    spoke.HubClient
	VDCManager   spoke.VDCManager
	ClusterID    string
	ReconcileDep map[string]time.Time
}

// SetupWithManager sets up the controller with the Manager
func (r *SpokeVDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.VirtualDataCenter{}).
		Complete(r)
}

// Reconcile handles VirtualDataCenter reconciliation on spoke clusters
func (r *SpokeVDCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Determine what triggered this reconciliation
	trigger := r.determineReconcileTrigger(ctx, req)
	klog.V(4).Infof("Reconciling VirtualDataCenter %s/%s, trigger: %s", req.Namespace, req.Name, trigger)

	// Get the VDC resource
	vdc := &ovimv1.VirtualDataCenter{}
	if err := r.Get(ctx, req.NamespacedName, vdc); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("VirtualDataCenter %s/%s not found, likely deleted", req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Failed to get VirtualDataCenter %s/%s: %v", req.Namespace, req.Name, err)
		return ctrl.Result{}, err
	}

	// On spoke clusters, process all VDCs (they are all spoke-local by definition)
	klog.V(4).Infof("Processing VDC %s/%s on spoke cluster", req.Namespace, req.Name)

	// Check if hub has requested deletion by setting the deletion annotation
	if vdc.Annotations != nil && vdc.Annotations[HubDeletionAnnotation] == HubDeletionValue {
		klog.Infof("Hub deletion requested for VDC %s/%s, initiating deletion", req.Namespace, req.Name)

		// Remove the hub deletion annotation to avoid reprocessing
		delete(vdc.Annotations, HubDeletionAnnotation)
		if err := r.Update(ctx, vdc); err != nil {
			klog.Errorf("Failed to remove hub deletion annotation: %v", err)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		// Set deletion timestamp to trigger the deletion process
		if err := r.Delete(ctx, vdc); err != nil {
			klog.Errorf("Failed to delete VDC %s/%s: %v", req.Namespace, req.Name, err)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		klog.Infof("Successfully initiated deletion for VDC %s/%s", req.Namespace, req.Name)
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if !vdc.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, vdc)
	}

	// Ensure finalizer is present
	finalizerName := "spokevdc.ovim.io/finalizer"
	if !controllerutil.ContainsFinalizer(vdc, finalizerName) {
		controllerutil.AddFinalizer(vdc, finalizerName)
		if err := r.Update(ctx, vdc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle creation/update
	return r.handleCreateOrUpdate(ctx, vdc)
}

// handleCreateOrUpdate processes VDC creation or updates
func (r *SpokeVDCReconciler) handleCreateOrUpdate(ctx context.Context, vdc *ovimv1.VirtualDataCenter) (ctrl.Result, error) {
	klog.Infof("Processing spoke-local VDC %s/%s", vdc.Namespace, vdc.Name)

	// Update last reconcile timestamp
	now := metav1.NewTime(time.Now())
	vdc.Status.LastReconcile = &now

	// Ensure organization namespace exists
	orgNamespace := vdc.Spec.OrgNamespace
	if orgNamespace == "" {
		orgNamespace = fmt.Sprintf("org-%s", vdc.Spec.OrganizationRef)
		vdc.Spec.OrgNamespace = orgNamespace
	}

	if err := r.ensureNamespace(ctx, orgNamespace); err != nil {
		klog.Errorf("Failed to ensure org namespace %s: %v", orgNamespace, err)
		r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to create organization namespace")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Ensure workload namespace exists using VDC manager
	workloadNamespace := vdc.Spec.TargetNamespace
	if workloadNamespace == "" {
		workloadNamespace = fmt.Sprintf("vdc-%s-%s", vdc.Spec.OrganizationRef, vdc.Name)
		vdc.Spec.TargetNamespace = workloadNamespace
	}

	// Use VDC manager for workload namespace creation if available
	if r.VDCManager != nil {
		if vdcMgr, ok := r.VDCManager.(interface {
			CreateVDCWorkloadNamespace(ctx context.Context, vdc *ovimv1.VirtualDataCenter) error
		}); ok {
			if err := vdcMgr.CreateVDCWorkloadNamespace(ctx, vdc); err != nil {
				klog.Errorf("Failed to create VDC workload namespace %s: %v", workloadNamespace, err)
				r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to create workload namespace")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
		} else {
			// Fallback to basic namespace creation
			if err := r.ensureNamespace(ctx, workloadNamespace); err != nil {
				klog.Errorf("Failed to ensure workload namespace %s: %v", workloadNamespace, err)
				r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to create workload namespace")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}
	} else {
		// Fallback to basic namespace creation
		if err := r.ensureNamespace(ctx, workloadNamespace); err != nil {
			klog.Errorf("Failed to ensure workload namespace %s: %v", workloadNamespace, err)
			r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to create workload namespace")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Update VDC status
	vdc.Status.OrgNamespace = orgNamespace
	vdc.Status.WorkloadNamespace = workloadNamespace
	vdc.Status.Namespace = workloadNamespace

	// Check if we need to reconcile with hub
	if vdc.Spec.ReconcileUntilSuccess && vdc.Status.HubSyncStatus != ovimv1.HubSyncStatusSuccess {
		result, err := r.reconcileWithHub(ctx, vdc)
		if err != nil {
			klog.Errorf("Hub reconciliation failed for VDC %s/%s: %v", vdc.Namespace, vdc.Name, err)
			r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, fmt.Sprintf("Hub sync failed: %v", err))
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if !result {
			// Hub sync still pending, requeue
			r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhasePending, "Hub synchronization pending")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// Mark as active
	r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseActive, "VDC is active and ready")

	// Only update the VDC resource if spec fields actually changed
	needsSpecUpdate := false

	// Check if organization namespace needs to be set
	if vdc.Spec.OrgNamespace != orgNamespace {
		vdc.Spec.OrgNamespace = orgNamespace
		needsSpecUpdate = true
	}

	// Check if target namespace needs to be set
	if vdc.Spec.TargetNamespace != workloadNamespace {
		vdc.Spec.TargetNamespace = workloadNamespace
		needsSpecUpdate = true
	}

	// Only call Update() if spec actually changed to avoid reconcile storms
	if needsSpecUpdate {
		if err := r.Update(ctx, vdc); err != nil {
			return ctrl.Result{}, err
		}
	}

	klog.Infof("Successfully reconciled spoke-local VDC %s/%s", vdc.Namespace, vdc.Name)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// handleDeletion processes VDC deletion
func (r *SpokeVDCReconciler) handleDeletion(ctx context.Context, vdc *ovimv1.VirtualDataCenter) (ctrl.Result, error) {
	klog.Infof("Deleting spoke-local VDC %s/%s", vdc.Namespace, vdc.Name)
	klog.Infof("VDC deletion status - WorkloadNamespace: '%s', OrgNamespace: '%s', Phase: '%s'",
		vdc.Status.WorkloadNamespace, vdc.Status.OrgNamespace, vdc.Status.Phase)

	// Update phase to suspended (deletion in progress)
	r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseSuspended, "VDC deletion in progress")

	// Clean up all VDC resources before deleting namespace
	workloadNamespace := vdc.Status.WorkloadNamespace

	// If WorkloadNamespace is not set in status, try to infer it from the VDC spec or construct it
	if workloadNamespace == "" {
		if vdc.Spec.TargetNamespace != "" {
			workloadNamespace = vdc.Spec.TargetNamespace
			klog.Infof("Using TargetNamespace from spec: %s", workloadNamespace)
		} else {
			// Construct the expected workload namespace name
			workloadNamespace = fmt.Sprintf("vdc-%s-%s", vdc.Spec.OrganizationRef, vdc.Name)
			klog.Infof("Constructed workload namespace name: %s", workloadNamespace)
		}
	}

	if workloadNamespace != "" {
		// Clean up individual VDC resources first (ResourceQuota, LimitRange, RoleBindings, NetworkPolicies)
		klog.Infof("Starting cleanup of VDC resources in namespace %s for VDC %s", workloadNamespace, vdc.Name)
		if err := r.cleanupVDCResources(ctx, workloadNamespace, vdc.Name); err != nil {
			klog.Errorf("Failed to cleanup VDC resources in namespace %s: %v", workloadNamespace, err)
			r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to cleanup VDC resources")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		klog.Infof("Successfully completed cleanup of VDC resources in namespace %s", workloadNamespace)

		// Now delete the workload namespace using VDC manager if available
		if r.VDCManager != nil {
			if vdcMgr, ok := r.VDCManager.(interface {
				DeleteVDCWorkloadNamespace(ctx context.Context, workloadNamespace string) error
			}); ok {
				if err := vdcMgr.DeleteVDCWorkloadNamespace(ctx, workloadNamespace); err != nil {
					klog.Errorf("Failed to delete VDC workload namespace %s: %v", workloadNamespace, err)
					r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to delete workload namespace")
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
			} else {
				// Fallback to direct deletion
				if err := r.deleteNamespace(ctx, workloadNamespace); err != nil && !errors.IsNotFound(err) {
					klog.Errorf("Failed to delete workload namespace %s: %v", workloadNamespace, err)
					r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to delete workload namespace")
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
			}
		} else {
			// Fallback to direct deletion
			if err := r.deleteNamespace(ctx, workloadNamespace); err != nil && !errors.IsNotFound(err) {
				klog.Errorf("Failed to delete workload namespace %s: %v", workloadNamespace, err)
				r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to delete workload namespace")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}

		klog.Infof("Successfully cleaned up VDC workload namespace %s", workloadNamespace)
	} else {
		klog.Warningf("No workload namespace found for VDC %s/%s - skipping workload cleanup", vdc.Namespace, vdc.Name)
	}

	// Check if organization namespace can be safely deleted
	orgNamespace := vdc.Status.OrgNamespace

	// If OrgNamespace is not set in status, try to infer it from the VDC spec or construct it
	if orgNamespace == "" {
		if vdc.Spec.OrgNamespace != "" {
			orgNamespace = vdc.Spec.OrgNamespace
			klog.Infof("Using OrgNamespace from spec: %s", orgNamespace)
		} else {
			// Construct the expected org namespace name
			orgNamespace = fmt.Sprintf("org-%s", vdc.Spec.OrganizationRef)
			klog.Infof("Constructed org namespace name: %s", orgNamespace)
		}
	}

	if orgNamespace != "" {
		// Use VDC manager for organization namespace deletion if available
		if r.VDCManager != nil {
			if vdcMgr, ok := r.VDCManager.(interface {
				CanDeleteOrganizationNamespace(ctx context.Context, orgNamespace, excludeVDC string) (bool, error)
				DeleteOrganizationNamespace(ctx context.Context, orgNamespace string) error
			}); ok {
				canDelete, err := vdcMgr.CanDeleteOrganizationNamespace(ctx, orgNamespace, vdc.Name)
				if err != nil {
					klog.Errorf("Failed to check if org namespace %s can be deleted: %v", orgNamespace, err)
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}

				if canDelete {
					if err := vdcMgr.DeleteOrganizationNamespace(ctx, orgNamespace); err != nil {
						klog.Errorf("Failed to delete org namespace %s: %v", orgNamespace, err)
						r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to delete organization namespace")
						return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
					}
					klog.Infof("Deleted empty organization namespace %s", orgNamespace)
				} else {
					klog.Infof("Organization namespace %s still has other VDCs, not deleting", orgNamespace)
				}
			} else {
				// Fallback to controller logic
				canDelete, err := r.canDeleteOrgNamespace(ctx, orgNamespace, vdc.Name)
				if err != nil {
					klog.Errorf("Failed to check if org namespace %s can be deleted: %v", orgNamespace, err)
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}

				if canDelete {
					if err := r.deleteNamespace(ctx, orgNamespace); err != nil && !errors.IsNotFound(err) {
						klog.Errorf("Failed to delete org namespace %s: %v", orgNamespace, err)
						r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to delete organization namespace")
						return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
					}
					klog.Infof("Deleted empty organization namespace %s", orgNamespace)
				} else {
					klog.Infof("Organization namespace %s still has other VDCs, not deleting", orgNamespace)
				}
			}
		} else {
			// Fallback to controller logic
			canDelete, err := r.canDeleteOrgNamespace(ctx, orgNamespace, vdc.Name)
			if err != nil {
				klog.Errorf("Failed to check if org namespace %s can be deleted: %v", orgNamespace, err)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			if canDelete {
				if err := r.deleteNamespace(ctx, orgNamespace); err != nil && !errors.IsNotFound(err) {
					klog.Errorf("Failed to delete org namespace %s: %v", orgNamespace, err)
					r.updateStatus(ctx, vdc, ovimv1.VirtualDataCenterPhaseFailed, "Failed to delete organization namespace")
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
				klog.Infof("Deleted empty organization namespace %s", orgNamespace)
			} else {
				klog.Infof("Organization namespace %s still has other VDCs, not deleting", orgNamespace)
			}
		}
	} else {
		klog.Warningf("No organization namespace found for VDC %s/%s - skipping org cleanup", vdc.Namespace, vdc.Name)
	}

	// Remove finalizer first
	finalizerName := "spokevdc.ovim.io/finalizer"
	klog.Infof("Removing finalizer %s from VDC %s/%s", finalizerName, vdc.Namespace, vdc.Name)

	hasFinalizer := controllerutil.ContainsFinalizer(vdc, finalizerName)
	if !hasFinalizer {
		klog.Warningf("VDC %s/%s does not have finalizer %s, but deletion is proceeding", vdc.Namespace, vdc.Name, finalizerName)
	}

	controllerutil.RemoveFinalizer(vdc, finalizerName)
	if err := r.Update(ctx, vdc); err != nil {
		klog.Errorf("Failed to remove finalizer from VDC %s/%s: %v", vdc.Namespace, vdc.Name, err)
		return ctrl.Result{}, err
	}

	klog.Infof("Successfully removed finalizer from VDC %s/%s", vdc.Namespace, vdc.Name)

	// Notify hub after all cleanup is complete and finalizer is removed
	if vdc.Spec.ReconcileUntilSuccess {
		if err := r.notifyHubDeletion(ctx, vdc); err != nil {
			klog.Errorf("Failed to notify hub of VDC deletion: %v", err)
			// Don't fail the deletion process if hub notification fails
			// The VDC has been successfully cleaned up locally
		} else {
			klog.Infof("Successfully notified hub of VDC deletion for %s/%s", vdc.Namespace, vdc.Name)
		}
	}

	klog.Infof("Successfully deleted spoke-local VDC %s/%s", vdc.Namespace, vdc.Name)
	return ctrl.Result{}, nil
}

// ensureNamespace creates a namespace if it doesn't exist using the VDC manager
func (r *SpokeVDCReconciler) ensureNamespace(ctx context.Context, name string) error {
	// For organization namespaces, use VDC manager's method
	if strings.HasPrefix(name, "org-") {
		// Extract organization name from namespace
		orgName := strings.TrimPrefix(name, "org-")
		if r.VDCManager != nil {
			// Use a type assertion to access the enhanced VDC manager methods
			if vdcMgr, ok := r.VDCManager.(interface {
				EnsureOrganizationNamespace(ctx context.Context, organizationName, clusterID string) error
			}); ok {
				return vdcMgr.EnsureOrganizationNamespace(ctx, orgName, r.ClusterID)
			}
		}
	}

	// Fallback to direct namespace creation
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"ovim.io/managed-by": "spoke-agent",
				"ovim.io/cluster-id": r.ClusterID,
			},
		},
	}

	if err := r.Create(ctx, namespace); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil // Namespace already exists
		}
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	klog.Infof("Created namespace %s", name)
	return nil
}

// deleteNamespace deletes a namespace
func (r *SpokeVDCReconciler) deleteNamespace(ctx context.Context, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := r.Delete(ctx, namespace); err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", name, err)
	}

	klog.Infof("Deleted namespace %s", name)
	return nil
}

// canDeleteOrgNamespace checks if an organization namespace can be safely deleted
func (r *SpokeVDCReconciler) canDeleteOrgNamespace(ctx context.Context, orgNamespace, currentVDCName string) (bool, error) {
	// List all VDCs in the same organization namespace
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := r.List(ctx, vdcList, client.InNamespace(orgNamespace)); err != nil {
		return false, fmt.Errorf("failed to list VDCs in namespace %s: %w", orgNamespace, err)
	}

	// Check if there are other VDCs besides the current one
	for _, vdc := range vdcList.Items {
		if vdc.Name != currentVDCName && vdc.DeletionTimestamp.IsZero() {
			return false, nil // Other VDCs still exist
		}
	}

	return true, nil // No other VDCs found, safe to delete
}

// reconcileWithHub syncs the VDC state with the hub
func (r *SpokeVDCReconciler) reconcileWithHub(ctx context.Context, vdc *ovimv1.VirtualDataCenter) (bool, error) {
	if r.HubClient == nil {
		klog.Warning("Hub client not configured, skipping hub reconciliation")
		return true, nil
	}

	// Increment retry count
	vdc.Status.RetryCount++

	// Prepare VDC data for hub
	vdcData := map[string]interface{}{
		"name":             vdc.Name,
		"namespace":        vdc.Namespace,
		"phase":            string(vdc.Status.Phase),
		"org_namespace":    vdc.Status.OrgNamespace,
		"target_namespace": vdc.Status.WorkloadNamespace,
		"operation_id":     vdc.Spec.HubOperationID,
		"cluster_id":       r.ClusterID,
		"retry_count":      vdc.Status.RetryCount,
	}

	// Send status to hub
	response, err := r.HubClient.SendVDCStatus(ctx, vdcData)
	if err != nil {
		vdc.Status.HubSyncStatus = ovimv1.HubSyncStatusFailed
		return false, fmt.Errorf("failed to send VDC status to hub: %w", err)
	}

	// Check hub response
	if response["status"] == "success" {
		vdc.Status.HubSyncStatus = ovimv1.HubSyncStatusSuccess
		vdc.Status.LastHubSync = &metav1.Time{Time: time.Now()}
		return true, nil
	}

	// Hub sync still pending
	vdc.Status.HubSyncStatus = ovimv1.HubSyncStatusPending
	return false, nil
}

// notifyHubDeletion notifies the hub that a VDC has been deleted
func (r *SpokeVDCReconciler) notifyHubDeletion(ctx context.Context, vdc *ovimv1.VirtualDataCenter) error {
	if r.HubClient == nil {
		klog.Warning("Hub client not configured, skipping hub deletion notification")
		return nil
	}

	deletionData := map[string]interface{}{
		"name":               vdc.Name,
		"namespace":          vdc.Namespace,
		"operation_id":       vdc.Spec.HubOperationID,
		"cluster_id":         r.ClusterID,
		"status":             "deleted",
		"workload_namespace": vdc.Status.WorkloadNamespace,
		"org_namespace":      vdc.Status.OrgNamespace,
		"cleanup_completed":  true,
		"deleted_at":         time.Now().Format(time.RFC3339),
		"resources_cleaned": []string{
			"ResourceQuota",
			"LimitRange",
			"RoleBindings",
			"NetworkPolicies",
			"ServiceAccounts",
			"Namespaces",
		},
	}

	if _, err := r.HubClient.SendVDCDeletion(ctx, deletionData); err != nil {
		return fmt.Errorf("failed to notify hub of VDC deletion: %w", err)
	}

	return nil
}

// updateStatus updates the VDC status only if something actually changed
func (r *SpokeVDCReconciler) updateStatus(ctx context.Context, vdc *ovimv1.VirtualDataCenter, phase ovimv1.VirtualDataCenterPhase, message string) {
	needsUpdate := false

	// Only update phase if it changed
	if vdc.Status.Phase != phase {
		vdc.Status.Phase = phase
		needsUpdate = true
	}

	// Prepare condition
	conditionStatus := metav1.ConditionFalse
	if phase == ovimv1.VirtualDataCenterPhaseActive {
		conditionStatus = metav1.ConditionTrue
	}

	// Only update condition if it actually changed
	conditionChanged := false
	found := false
	for i, existing := range vdc.Status.Conditions {
		if existing.Type == "Ready" {
			found = true
			// Only update if status, reason, or message changed
			if existing.Status != conditionStatus || existing.Reason != string(phase) || existing.Message != message {
				existing.Status = conditionStatus
				existing.Reason = string(phase)
				existing.Message = message
				existing.LastTransitionTime = metav1.NewTime(time.Now()) // Only update timestamp when values change
				vdc.Status.Conditions[i] = existing
				conditionChanged = true
			}
			break
		}
	}

	// Add new condition if not found
	if !found {
		condition := metav1.Condition{
			Type:               "Ready",
			Status:             conditionStatus,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Reason:             string(phase),
			Message:            message,
		}
		vdc.Status.Conditions = append(vdc.Status.Conditions, condition)
		conditionChanged = true
	}

	if conditionChanged {
		needsUpdate = true
	}

	// Only update LastReconcile timestamp when we actually need to write
	if needsUpdate {
		vdc.Status.LastReconcile = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, vdc); err != nil {
			klog.Errorf("Failed to update VDC status: %v", err)
		}
	}
	// If nothing changed, skip the write (idempotent)
}

// determineReconcileTrigger analyzes the context and resource to determine what triggered the reconciliation
func (r *SpokeVDCReconciler) determineReconcileTrigger(ctx context.Context, req ctrl.Request) string {
	// Get the current VDC to analyze
	vdc := &ovimv1.VirtualDataCenter{}
	if err := r.Get(ctx, req.NamespacedName, vdc); err != nil {
		if errors.IsNotFound(err) {
			return "resource-deleted"
		}
		return "resource-fetch-error"
	}

	// Check if this is a deletion
	if !vdc.DeletionTimestamp.IsZero() {
		return "resource-deletion"
	}

	// Check if hub has requested deletion via annotation
	if vdc.Annotations != nil && vdc.Annotations[HubDeletionAnnotation] == HubDeletionValue {
		return "hub-deletion-request"
	}

	// Check if this is a new resource (recently created)
	if vdc.CreationTimestamp.Add(10 * time.Second).After(time.Now()) {
		return "resource-creation"
	}

	// Check for spec changes (admin or configuration updates from hub)
	if vdc.Generation > 1 {
		return "spec-change"
	}

	// Check for recent status changes
	for _, condition := range vdc.Status.Conditions {
		if condition.LastTransitionTime.Add(30 * time.Second).After(time.Now()) {
			return "status-change"
		}
	}

	// Check if this is hub synchronization related
	if vdc.Spec.ReconcileUntilSuccess && vdc.Status.HubSyncStatus != ovimv1.HubSyncStatusSuccess {
		return "hub-sync-retry"
	}

	// Check if this is a requeue from a previous reconciliation
	if vdc.Status.LastReconcile != nil && vdc.Status.LastReconcile.Add(5*time.Minute).After(time.Now()) {
		return "periodic-requeue"
	}

	// Check for finalizer operations
	if len(vdc.Finalizers) > 0 && vdc.DeletionTimestamp.IsZero() {
		return "finalizer-update"
	}

	// Check if namespaces need to be created
	if vdc.Status.OrgNamespace == "" || vdc.Status.WorkloadNamespace == "" {
		return "namespace-creation"
	}

	// Default cases
	if vdc.Status.LastReconcile == nil {
		return "initial-reconcile"
	}

	return "periodic-reconcile"
}

// cleanupVDCResources cleans up all VDC-related resources from the workload namespace
func (r *SpokeVDCReconciler) cleanupVDCResources(ctx context.Context, namespace, vdcName string) error {
	klog.Infof("Cleaning up VDC resources in namespace %s for VDC %s", namespace, vdcName)

	// Clean up ResourceQuota
	resourceQuota := &corev1.ResourceQuota{}
	err := r.Get(ctx, types.NamespacedName{Name: "vdc-quota", Namespace: namespace}, resourceQuota)
	if err == nil {
		if err := r.Delete(ctx, resourceQuota); err != nil && !errors.IsNotFound(err) {
			klog.Errorf("Failed to delete ResourceQuota: %v", err)
			return fmt.Errorf("failed to delete ResourceQuota: %w", err)
		}
		klog.Infof("Deleted ResourceQuota vdc-quota from namespace %s", namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get ResourceQuota: %w", err)
	}

	// Clean up LimitRange
	limitRange := &corev1.LimitRange{}
	err = r.Get(ctx, types.NamespacedName{Name: "vdc-limits", Namespace: namespace}, limitRange)
	if err == nil {
		if err := r.Delete(ctx, limitRange); err != nil && !errors.IsNotFound(err) {
			klog.Errorf("Failed to delete LimitRange: %v", err)
			return fmt.Errorf("failed to delete LimitRange: %w", err)
		}
		klog.Infof("Deleted LimitRange vdc-limits from namespace %s", namespace)
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
			klog.Errorf("Failed to delete RoleBinding %s: %v", binding.Name, err)
			return fmt.Errorf("failed to delete RoleBinding %s: %w", binding.Name, err)
		}
		klog.Infof("Deleted RoleBinding %s from namespace %s", binding.Name, namespace)
	}

	// Clean up NetworkPolicies (skip if networking API not available)
	networkPolicyList := &networkingv1.NetworkPolicyList{}
	if err := r.List(ctx, networkPolicyList,
		client.InNamespace(namespace),
		client.MatchingLabels{"managed-by": "ovim", "type": "vdc-network-policy"}); err != nil {
		// If networking API is not available or scheme issue, log warning and continue
		klog.Warningf("Failed to list NetworkPolicies (may not exist or API unavailable): %v", err)
	} else {
		for _, policy := range networkPolicyList.Items {
			if err := r.Delete(ctx, &policy); err != nil && !errors.IsNotFound(err) {
				klog.Errorf("Failed to delete NetworkPolicy %s: %v", policy.Name, err)
				return fmt.Errorf("failed to delete NetworkPolicy %s: %w", policy.Name, err)
			}
			klog.Infof("Deleted NetworkPolicy %s from namespace %s", policy.Name, namespace)
		}
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
			klog.Errorf("Failed to delete ServiceAccount %s: %v", sa.Name, err)
			return fmt.Errorf("failed to delete ServiceAccount %s: %w", sa.Name, err)
		}
		klog.Infof("Deleted ServiceAccount %s from namespace %s", sa.Name, namespace)
	}

	klog.Infof("Successfully cleaned up all VDC resources in namespace %s", namespace)
	return nil
}
