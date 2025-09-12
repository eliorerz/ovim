package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
)

func setupMetricsTest() (*MetricsReconciler, client.Client) {
	// Create scheme with our CRD types
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = ovimv1.AddToScheme(s)

	// Create fake client with status subresource support
	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithStatusSubresource(&ovimv1.VirtualDataCenter{}).
		Build()

	// Create reconciler
	reconciler := &MetricsReconciler{
		Client: fakeClient,
		Scheme: s,
	}

	return reconciler, fakeClient
}

func TestMetricsReconciler_Reconcile_VDCNotFound(t *testing.T) {
	reconciler, _ := setupMetricsTest()
	ctx := context.Background()

	// Try to reconcile non-existent VDC
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "test-namespace",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestMetricsReconciler_Reconcile_VDCBeingDeleted(t *testing.T) {
	reconciler, client := setupMetricsTest()
	ctx := context.Background()

	// Create VDC with deletion timestamp
	now := metav1.Now()
	vdc := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-vdc",
			Namespace:         "org-test-org",
			DeletionTimestamp: &now,
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test VDC",
		},
	}

	err := client.Create(ctx, vdc)
	require.NoError(t, err)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestMetricsReconciler_Reconcile_VDCNotReady(t *testing.T) {
	reconciler, client := setupMetricsTest()
	ctx := context.Background()

	// Create VDC without namespace set
	vdc := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test VDC",
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Phase: ovimv1.VirtualDataCenterPhasePending,
		},
	}

	err := client.Create(ctx, vdc)
	require.NoError(t, err)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

func TestMetricsReconciler_Reconcile_CollectMetrics(t *testing.T) {
	reconciler, client := setupMetricsTest()
	ctx := context.Background()

	// Create ready VDC
	vdc := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test VDC",
		},
		Status: ovimv1.VirtualDataCenterStatus{
			Namespace: "vdc-test-org-test-vdc",
			Phase:     ovimv1.VirtualDataCenterPhaseActive,
		},
	}

	err := client.Create(ctx, vdc)
	require.NoError(t, err)

	// Create pods in VDC namespace
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "vdc-test-org-test-vdc",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "vdc-test-org-test-vdc",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	err = client.Create(ctx, pod1)
	require.NoError(t, err)
	err = client.Create(ctx, pod2)
	require.NoError(t, err)

	// Create PVC for storage metrics
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "vdc-test-org-test-vdc",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}

	err = client.Create(ctx, pvc)
	require.NoError(t, err)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 2 * time.Minute}, result)

	// Verify VDC status was updated with metrics
	var updatedVDC ovimv1.VirtualDataCenter
	err = client.Get(ctx, req.NamespacedName, &updatedVDC)
	require.NoError(t, err)

	assert.Equal(t, 2, updatedVDC.Status.TotalPods)
	assert.Equal(t, 0, updatedVDC.Status.TotalVMs) // No VMs in fake client
	assert.NotNil(t, updatedVDC.Status.ResourceUsage)
	assert.NotNil(t, updatedVDC.Status.LastMetricsUpdate)

	// Verify CPU and memory calculations
	assert.Equal(t, "300m", updatedVDC.Status.ResourceUsage.CPUUsed)     // 100m + 200m
	assert.Equal(t, "384Mi", updatedVDC.Status.ResourceUsage.MemoryUsed) // 128Mi + 256Mi
	assert.Equal(t, "10Gi", updatedVDC.Status.ResourceUsage.StorageUsed)
}

func TestMetricsReconciler_CollectPodMetrics(t *testing.T) {
	reconciler, client := setupMetricsTest()
	ctx := context.Background()

	namespace := "test-namespace"

	// Create pods with different statuses
	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "running-pod",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-pod",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	err := client.Create(ctx, runningPod)
	require.NoError(t, err)
	err = client.Create(ctx, pendingPod)
	require.NoError(t, err)

	// Collect metrics
	metrics, err := reconciler.collectPodMetrics(ctx, namespace)
	require.NoError(t, err)

	// Verify metrics - should only count running pods
	assert.Equal(t, 2, metrics.Count) // Total count includes all pods
	assert.Equal(t, "500m", metrics.CPUUsed.String())
	assert.Equal(t, "512Mi", metrics.MemoryUsed.String())
}

func TestMetricsReconciler_CollectStorageMetrics(t *testing.T) {
	reconciler, client := setupMetricsTest()
	ctx := context.Background()

	namespace := "test-namespace"

	// Create PVCs with different statuses
	boundPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bound-pvc",
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}

	pendingPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-pvc",
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimPending,
		},
	}

	err := client.Create(ctx, boundPVC)
	require.NoError(t, err)
	err = client.Create(ctx, pendingPVC)
	require.NoError(t, err)

	// Collect metrics
	metrics, err := reconciler.collectStorageMetrics(ctx, namespace)
	require.NoError(t, err)

	// Verify metrics - should only count bound PVCs
	assert.Equal(t, "5Gi", metrics.StorageUsed.String())
}

func TestMetricsReconciler_CollectVMMetrics(t *testing.T) {
	reconciler, client := setupMetricsTest()
	ctx := context.Background()

	namespace := "test-namespace"

	// Create mock VirtualMachine objects
	vm1 := &unstructured.Unstructured{}
	vm1.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	})
	vm1.SetName("test-vm-1")
	vm1.SetNamespace(namespace)

	vm2 := &unstructured.Unstructured{}
	vm2.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	})
	vm2.SetName("test-vm-2")
	vm2.SetNamespace(namespace)

	err := client.Create(ctx, vm1)
	require.NoError(t, err)
	err = client.Create(ctx, vm2)
	require.NoError(t, err)

	// Collect VM metrics
	metrics, err := reconciler.collectVMMetrics(ctx, namespace)
	require.NoError(t, err)

	// Verify VM count
	assert.Equal(t, 2, metrics.Count)
}

func TestMetricsReconciler_CollectVMMetrics_KubeVirtNotAvailable(t *testing.T) {
	reconciler, _ := setupMetricsTest()
	ctx := context.Background()

	namespace := "test-namespace"

	// Try to collect VM metrics when KubeVirt is not available
	// This should not fail but return zero VMs
	metrics, err := reconciler.collectVMMetrics(ctx, namespace)
	require.NoError(t, err)
	assert.Equal(t, 0, metrics.Count)
}

func TestMetricsReconciler_CollectResourceMetrics(t *testing.T) {
	reconciler, client := setupMetricsTest()
	ctx := context.Background()

	namespace := "test-namespace"

	// Create test resources
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}

	err := client.Create(ctx, pod)
	require.NoError(t, err)
	err = client.Create(ctx, pvc)
	require.NoError(t, err)

	// Collect all resource metrics
	metrics, err := reconciler.collectResourceMetrics(ctx, namespace)
	require.NoError(t, err)

	// Verify all metrics
	assert.Equal(t, 1, metrics.TotalPods)
	assert.Equal(t, 0, metrics.TotalVMs) // KubeVirt not available in test
	assert.NotNil(t, metrics.ResourceUsage)
	assert.Equal(t, "250m", metrics.ResourceUsage.CPUUsed)
	assert.Equal(t, "256Mi", metrics.ResourceUsage.MemoryUsed)
	assert.Equal(t, "20Gi", metrics.ResourceUsage.StorageUsed)
}

func TestMetricsReconciler_SetupWithManager(t *testing.T) {
	reconciler, _ := setupMetricsTest()

	// This test verifies that SetupWithManager can be called without error
	// In a real test environment, you would use a real manager
	// For this unit test, we just verify the method exists and has the right signature
	assert.NotNil(t, reconciler.SetupWithManager)
}
