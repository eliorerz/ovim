package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
)

// MetricsReconciler collects and updates resource usage metrics for VDCs
type MetricsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters,verbs=get;list;watch
// +kubebuilder:rbac:groups=ovim.io,resources=virtualdatacenters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch

// Reconcile collects metrics for VirtualDataCenter resources
func (r *MetricsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vdc", req.NamespacedName)

	// Determine what triggered this reconciliation
	trigger := r.determineReconcileTrigger(ctx, req)
	logger = logger.WithValues("trigger", trigger)
	logger.Info("Starting metrics reconciliation", "trigger", trigger)

	// Fetch the VirtualDataCenter instance
	var vdc ovimv1.VirtualDataCenter
	if err := r.Get(ctx, req.NamespacedName, &vdc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch VirtualDataCenter")
		return ctrl.Result{}, err
	}

	// Hub metrics controller should only process hub-managed VDCs, not spoke VDCs
	// Spoke VDCs are identified by the "ovim.io/managed-by": "spoke-agent" label
	if managedBy, exists := vdc.Labels["ovim.io/managed-by"]; exists && managedBy == "spoke-agent" {
		logger.V(4).Info("Skipping spoke-managed VDC on hub metrics controller", "vdc", vdc.Name, "managed-by", managedBy)
		return ctrl.Result{}, nil
	}

	// Skip VDCs that aren't hub-managed (let VDC controller claim them first)
	if managedBy, exists := vdc.Labels["ovim.io/managed-by"]; !exists || (managedBy != "hub-controller" && managedBy != "") {
		logger.V(4).Info("Skipping unlabeled VDC on metrics controller", "vdc", vdc.Name, "managed-by", managedBy)
		return ctrl.Result{}, nil
	}

	// Skip if VDC is being deleted or not ready
	if vdc.DeletionTimestamp != nil || vdc.Status.Namespace == "" {
		return ctrl.Result{}, nil
	}

	// Collect resource usage from the VDC namespace
	metrics, err := r.collectResourceMetrics(ctx, vdc.Status.Namespace)
	if err != nil {
		logger.Error(err, "failed to collect resource metrics")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Update VDC status with collected metrics using retry on conflict - only if metrics changed
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get latest version of the resource
		if getErr := r.Get(ctx, client.ObjectKeyFromObject(&vdc), &vdc); getErr != nil {
			return getErr
		}

		// Check if metrics actually changed (idempotent pattern)
		needsUpdate := false

		// Use deep equality for resource usage comparison
		if !equality.Semantic.DeepEqual(vdc.Status.ResourceUsage, metrics.ResourceUsage) {
			vdc.Status.ResourceUsage = metrics.ResourceUsage
			needsUpdate = true
		}

		if vdc.Status.TotalPods != metrics.TotalPods {
			vdc.Status.TotalPods = metrics.TotalPods
			needsUpdate = true
		}

		if vdc.Status.TotalVMs != metrics.TotalVMs {
			vdc.Status.TotalVMs = metrics.TotalVMs
			needsUpdate = true
		}

		// Only update timestamp when metrics actually changed
		if needsUpdate {
			vdc.Status.LastMetricsUpdate = &metav1.Time{Time: time.Now()}
			return r.Status().Update(ctx, &vdc)
		}

		// No-op: metrics are identical, skip write to etcd
		return nil
	}); err != nil {
		logger.Error(err, "unable to update VDC status with metrics after retries")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	logger.Info("Metrics collected successfully",
		"pods", metrics.TotalPods,
		"vms", metrics.TotalVMs,
		"cpu", metrics.ResourceUsage.CPUUsed,
		"memory", metrics.ResourceUsage.MemoryUsed)

	// Requeue after 2 minutes for regular metrics collection
	return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}

// VDCMetrics holds collected metrics for a VDC
type VDCMetrics struct {
	ResourceUsage *ovimv1.ResourceUsage
	TotalPods     int
	TotalVMs      int
}

// collectResourceMetrics collects resource usage from the VDC namespace
func (r *MetricsReconciler) collectResourceMetrics(ctx context.Context, namespace string) (*VDCMetrics, error) {
	logger := log.FromContext(ctx).WithValues("namespace", namespace)

	metrics := &VDCMetrics{
		ResourceUsage: &ovimv1.ResourceUsage{
			CPUUsed:     "0",
			MemoryUsed:  "0",
			StorageUsed: "0",
		},
	}

	// Collect Pod metrics
	podMetrics, err := r.collectPodMetrics(ctx, namespace)
	if err != nil {
		logger.Error(err, "failed to collect pod metrics")
		return nil, err
	}

	metrics.TotalPods = podMetrics.Count
	metrics.ResourceUsage.CPUUsed = podMetrics.CPUUsed.String()
	metrics.ResourceUsage.MemoryUsed = podMetrics.MemoryUsed.String()

	// Collect VM metrics (KubeVirt VirtualMachines)
	vmMetrics, err := r.collectVMMetrics(ctx, namespace)
	if err != nil {
		// VMs might not be available in this cluster, log but don't fail
		logger.Info("Failed to collect VM metrics (KubeVirt might not be available)", "error", err)
		vmMetrics = &VMMetrics{Count: 0}
	}

	metrics.TotalVMs = vmMetrics.Count

	// Collect storage metrics from PersistentVolumeClaims
	storageMetrics, err := r.collectStorageMetrics(ctx, namespace)
	if err != nil {
		logger.Error(err, "failed to collect storage metrics")
		return nil, err
	}

	metrics.ResourceUsage.StorageUsed = storageMetrics.StorageUsed.String()

	return metrics, nil
}

// PodMetrics holds pod resource usage information
type PodMetrics struct {
	Count      int
	CPUUsed    resource.Quantity
	MemoryUsed resource.Quantity
}

// collectPodMetrics collects resource usage from pods in the namespace
func (r *MetricsReconciler) collectPodMetrics(ctx context.Context, namespace string) (*PodMetrics, error) {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	metrics := &PodMetrics{
		Count:      len(pods.Items),
		CPUUsed:    resource.MustParse("0"),
		MemoryUsed: resource.MustParse("0"),
	}

	// Calculate current usage from pod requests
	for _, pod := range pods.Items {
		// Only count running pods
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		for _, container := range pod.Spec.Containers {
			if cpu := container.Resources.Requests[corev1.ResourceCPU]; !cpu.IsZero() {
				metrics.CPUUsed.Add(cpu)
			}
			if memory := container.Resources.Requests[corev1.ResourceMemory]; !memory.IsZero() {
				metrics.MemoryUsed.Add(memory)
			}
		}
	}

	return metrics, nil
}

// VMMetrics holds VM information
type VMMetrics struct {
	Count int
}

// collectVMMetrics collects VirtualMachine metrics (KubeVirt)
func (r *MetricsReconciler) collectVMMetrics(ctx context.Context, namespace string) (*VMMetrics, error) {
	vms := &unstructured.UnstructuredList{}
	vms.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	})

	if err := r.List(ctx, vms, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	return &VMMetrics{
		Count: len(vms.Items),
	}, nil
}

// StorageMetrics holds storage usage information
type StorageMetrics struct {
	StorageUsed resource.Quantity
}

// collectStorageMetrics collects storage usage from PersistentVolumeClaims
func (r *MetricsReconciler) collectStorageMetrics(ctx context.Context, namespace string) (*StorageMetrics, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.List(ctx, pvcs, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	metrics := &StorageMetrics{
		StorageUsed: resource.MustParse("0"),
	}

	// Sum up all PVC storage requests
	for _, pvc := range pvcs.Items {
		if pvc.Status.Phase == corev1.ClaimBound {
			if storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; !storage.IsZero() {
				metrics.StorageUsed.Add(storage)
			}
		}
	}

	return metrics, nil
}

// determineReconcileTrigger analyzes the context and resource to determine what triggered the reconciliation
func (r *MetricsReconciler) determineReconcileTrigger(ctx context.Context, req ctrl.Request) string {
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

	// Check if metrics haven't been collected yet
	if vdc.Status.LastMetricsUpdate == nil {
		return "initial-metrics-collection"
	}

	// Check if this could be a scheduled metrics collection (every 2 minutes)
	if vdc.Status.LastMetricsUpdate.Add(2 * time.Minute).Before(time.Now()) {
		return "scheduled-metrics-collection"
	}

	// Check for recent status changes in the main VDC
	for _, condition := range vdc.Status.Conditions {
		if condition.LastTransitionTime.Add(30 * time.Second).After(time.Now()) {
			return fmt.Sprintf("vdc-condition-change-%s", condition.Type)
		}
	}

	// Check if VDC namespace changed (would trigger metrics collection)
	if vdc.Status.Namespace != "" {
		return "namespace-ready"
	}

	// Check if this is triggered by pod/VM changes in the workload namespace
	if vdc.Status.LastMetricsUpdate.Add(30 * time.Second).After(time.Now()) {
		return "recent-metrics-update"
	}

	return "metrics-periodic-check"
}

// SetupWithManager sets up the controller with the Manager
func (r *MetricsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.VirtualDataCenter{}).
		Named("ovim-metrics-controller").
		Complete(r)
}
