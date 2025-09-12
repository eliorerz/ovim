package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	// Fetch the VirtualDataCenter instance
	var vdc ovimv1.VirtualDataCenter
	if err := r.Get(ctx, req.NamespacedName, &vdc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch VirtualDataCenter")
		return ctrl.Result{}, err
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

	// Update VDC status with collected metrics
	vdc.Status.ResourceUsage = metrics.ResourceUsage
	vdc.Status.TotalPods = metrics.TotalPods
	vdc.Status.TotalVMs = metrics.TotalVMs
	vdc.Status.LastMetricsUpdate = &metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, &vdc); err != nil {
		logger.Error(err, "unable to update VDC status with metrics")
		return ctrl.Result{}, err
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

// SetupWithManager sets up the controller with the Manager
func (r *MetricsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovimv1.VirtualDataCenter{}).
		Complete(r)
}
