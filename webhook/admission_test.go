package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func setupWebhookTest() (*WorkloadWebhook, client.Client) {
	// Create scheme
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)

	// Create fake client
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

	// Create webhook
	webhook := &WorkloadWebhook{
		Client: fakeClient,
	}

	return webhook, fakeClient
}

func createAdmissionRequest(kind, namespace string, obj runtime.Object) admission.Request {
	raw, _ := json.Marshal(obj)
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Kind: metav1.GroupVersionKind{
				Kind: kind,
			},
			Namespace: namespace,
			Object: runtime.RawExtension{
				Raw: raw,
			},
		},
	}
}

func TestWorkloadWebhook_Handle_UnsupportedKind(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Kind: metav1.GroupVersionKind{
				Kind: "UnsupportedKind",
			},
		},
	}

	resp := webhook.Handle(ctx, req)
	assert.True(t, resp.Allowed)
	assert.Equal(t, "", resp.Result.Message)
}

func TestWorkloadWebhook_HandlePod_AllowedInVDCNamespace(t *testing.T) {
	webhook, client := setupWebhookTest()
	ctx := context.Background()

	// Create VDC namespace
	vdcNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vdc-test-org-test-vdc",
			Labels: map[string]string{
				"type":                         "vdc",
				"managed-by":                   "ovim",
				"org":                          "test-org",
				"vdc":                          "test-vdc",
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/managed-by": "ovim-controller",
			},
		},
	}
	err := client.Create(ctx, vdcNamespace)
	require.NoError(t, err)

	// Create pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "vdc-test-org-test-vdc",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
			}},
		},
	}

	req := createAdmissionRequest("Pod", "vdc-test-org-test-vdc", pod)
	resp := webhook.Handle(ctx, req)

	assert.True(t, resp.Allowed)
}

func TestWorkloadWebhook_HandlePod_DeniedInOrgNamespace(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "org-test-org",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
			}},
		},
	}

	req := createAdmissionRequest("Pod", "org-test-org", pod)
	resp := webhook.Handle(ctx, req)

	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "Workloads are not allowed in organization namespaces")
}

func TestWorkloadWebhook_HandlePod_AllowedInRegularNamespace(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
			}},
		},
	}

	req := createAdmissionRequest("Pod", "default", pod)
	resp := webhook.Handle(ctx, req)

	assert.True(t, resp.Allowed)
}

func TestWorkloadWebhook_HandlePod_DeniedInInvalidVDCNamespace(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "vdc-invalid-namespace",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-container",
				Image: "test-image",
			}},
		},
	}

	req := createAdmissionRequest("Pod", "vdc-invalid-namespace", pod)
	resp := webhook.Handle(ctx, req)

	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "VDC namespace validation failed")
}

func TestWorkloadWebhook_HandlePod_InvalidJSON(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Kind: metav1.GroupVersionKind{
				Kind: "Pod",
			},
			Namespace: "test-namespace",
			Object: runtime.RawExtension{
				Raw: []byte("invalid json"),
			},
		},
	}

	resp := webhook.Handle(ctx, req)

	assert.False(t, resp.Allowed)
	assert.Equal(t, http.StatusBadRequest, int(resp.Result.Code))
}

func TestWorkloadWebhook_HandleWorkload_DeploymentInOrgNamespace(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "org-test-org",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test-container",
						Image: "test-image",
					}},
				},
			},
		},
	}

	req := createAdmissionRequest("Deployment", "org-test-org", deployment)
	resp := webhook.Handle(ctx, req)

	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "Deployment workloads are not allowed in organization namespaces")
}

func TestWorkloadWebhook_HandleWorkload_StatefulSetInVDCNamespace(t *testing.T) {
	webhook, client := setupWebhookTest()
	ctx := context.Background()

	// Create VDC namespace
	vdcNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vdc-test-org-test-vdc",
			Labels: map[string]string{
				"type":                         "vdc",
				"managed-by":                   "ovim",
				"org":                          "test-org",
				"vdc":                          "test-vdc",
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/managed-by": "ovim-controller",
			},
		},
	}
	err := client.Create(ctx, vdcNamespace)
	require.NoError(t, err)

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "vdc-test-org-test-vdc",
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "test-service",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test-container",
						Image: "test-image",
					}},
				},
			},
		},
	}

	req := createAdmissionRequest("StatefulSet", "vdc-test-org-test-vdc", statefulSet)
	resp := webhook.Handle(ctx, req)

	assert.True(t, resp.Allowed)
}

func TestWorkloadWebhook_HandleVirtualMachine_DeniedInOrgNamespace(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	vm := &unstructured.Unstructured{}
	vm.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	})
	vm.SetName("test-vm")
	vm.SetNamespace("org-test-org")

	req := createAdmissionRequest("VirtualMachine", "org-test-org", vm)
	resp := webhook.Handle(ctx, req)

	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "Virtual Machines are not allowed in organization namespaces")
}

func TestWorkloadWebhook_HandleVirtualMachine_AllowedInVDCNamespace(t *testing.T) {
	webhook, client := setupWebhookTest()
	ctx := context.Background()

	// Create VDC namespace
	vdcNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vdc-test-org-test-vdc",
			Labels: map[string]string{
				"type":                         "vdc",
				"managed-by":                   "ovim",
				"org":                          "test-org",
				"vdc":                          "test-vdc",
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/managed-by": "ovim-controller",
			},
		},
	}
	err := client.Create(ctx, vdcNamespace)
	require.NoError(t, err)

	vm := &unstructured.Unstructured{}
	vm.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	})
	vm.SetName("test-vm")
	vm.SetNamespace("vdc-test-org-test-vdc")

	req := createAdmissionRequest("VirtualMachine", "vdc-test-org-test-vdc", vm)
	resp := webhook.Handle(ctx, req)

	assert.True(t, resp.Allowed)
}

func TestWorkloadWebhook_HandleNamespace_DeniedUnauthorizedOrgNamespace(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "org-unauthorized",
			Labels: map[string]string{
				"type": "org",
				// Missing "managed-by": "ovim"
			},
		},
	}

	req := createAdmissionRequest("Namespace", "", ns)
	resp := webhook.Handle(ctx, req)

	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "Organization and VDC namespaces can only be created by OVIM controllers")
}

func TestWorkloadWebhook_HandleNamespace_AllowedOVIMOrgNamespace(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "org-test-org",
			Labels: map[string]string{
				"type":                         "org",
				"managed-by":                   "ovim",
				"org":                          "test-org",
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/managed-by": "ovim-controller",
			},
		},
	}

	req := createAdmissionRequest("Namespace", "", ns)
	resp := webhook.Handle(ctx, req)

	assert.True(t, resp.Allowed)
}

func TestWorkloadWebhook_HandleNamespace_DeniedInvalidVDCNamespace(t *testing.T) {
	webhook, client := setupWebhookTest()
	ctx := context.Background()

	// Create parent organization namespace first
	orgNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "org-test-org",
			Labels: map[string]string{
				"type":       "org",
				"managed-by": "ovim",
				"org":        "test-org",
			},
		},
	}
	err := client.Create(ctx, orgNS)
	require.NoError(t, err)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vdc-test-org-test-vdc",
			Labels: map[string]string{
				"type":       "vdc",
				"managed-by": "ovim",
				"org":        "test-org",
				"vdc":        "test-vdc",
				// Missing required labels
			},
		},
	}

	req := createAdmissionRequest("Namespace", "", ns)
	resp := webhook.Handle(ctx, req)

	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "required label 'app.kubernetes.io/name' missing")
}

func TestWorkloadWebhook_HandleNamespace_AllowedRegularNamespace(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "regular-namespace",
		},
	}

	req := createAdmissionRequest("Namespace", "", ns)
	resp := webhook.Handle(ctx, req)

	assert.True(t, resp.Allowed)
}

func TestWorkloadWebhook_ValidateVDCNamespace_NamespaceNotFound(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	err := webhook.validateVDCNamespace(ctx, "non-existent-namespace")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found or inaccessible")
}

func TestWorkloadWebhook_ValidateVDCNamespace_InvalidLabels(t *testing.T) {
	webhook, client := setupWebhookTest()
	ctx := context.Background()

	// Create namespace with invalid labels
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-vdc-namespace",
			Labels: map[string]string{
				"type": "wrong-type", // Should be "vdc"
			},
		},
	}
	err := client.Create(ctx, ns)
	require.NoError(t, err)

	err = webhook.validateVDCNamespace(ctx, "invalid-vdc-namespace")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a properly managed VDC namespace")
}

func TestWorkloadWebhook_ValidateVDCNamespace_MissingOrgLabel(t *testing.T) {
	webhook, client := setupWebhookTest()
	ctx := context.Background()

	// Create namespace missing org label
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vdc-missing-org",
			Labels: map[string]string{
				"type":       "vdc",
				"managed-by": "ovim",
				// Missing "org" label
			},
		},
	}
	err := client.Create(ctx, ns)
	require.NoError(t, err)

	err = webhook.validateVDCNamespace(ctx, "vdc-missing-org")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing organization reference")
}

func TestWorkloadWebhook_ValidateOVIMNamespace_InvalidOrgPrefix(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-org-name",
			Labels: map[string]string{
				"type":       "org",
				"managed-by": "ovim",
				"org":        "test-org",
			},
		},
	}

	err := webhook.validateOVIMNamespace(ctx, ns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must have 'org-' prefix")
}

func TestWorkloadWebhook_ValidateOVIMNamespace_VDCMissingParentOrg(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vdc-test-org-test-vdc",
			Labels: map[string]string{
				"type":                         "vdc",
				"managed-by":                   "ovim",
				"org":                          "test-org",
				"vdc":                          "test-vdc",
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/managed-by": "ovim-controller",
			},
		},
	}

	err := webhook.validateOVIMNamespace(ctx, ns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent organization namespace org-test-org not found")
}

func TestWorkloadWebhook_ValidateOVIMNamespace_ValidVDCWithParentOrg(t *testing.T) {
	webhook, client := setupWebhookTest()
	ctx := context.Background()

	// Create parent organization namespace
	orgNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "org-test-org",
			Labels: map[string]string{
				"type":       "org",
				"managed-by": "ovim",
				"org":        "test-org",
			},
		},
	}
	err := client.Create(ctx, orgNS)
	require.NoError(t, err)

	vdcNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vdc-test-org-test-vdc",
			Labels: map[string]string{
				"type":                         "vdc",
				"managed-by":                   "ovim",
				"org":                          "test-org",
				"vdc":                          "test-vdc",
				"app.kubernetes.io/name":       "ovim",
				"app.kubernetes.io/managed-by": "ovim-controller",
			},
		},
	}

	err = webhook.validateOVIMNamespace(ctx, vdcNS)
	assert.NoError(t, err)
}

func TestWorkloadWebhook_ValidateOVIMNamespace_UnsupportedType(t *testing.T) {
	webhook, _ := setupWebhookTest()
	ctx := context.Background()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			Labels: map[string]string{
				"type":       "unsupported",
				"managed-by": "ovim",
			},
		},
	}

	err := webhook.validateOVIMNamespace(ctx, ns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported OVIM namespace type: unsupported")
}

func TestWorkloadWebhook_SetupWithManager(t *testing.T) {
	webhook, _ := setupWebhookTest()

	// This test verifies that SetupWithManager can be called without error
	// In a real test environment, you would use a real manager
	// For this unit test, we just verify the method exists and has the right signature
	assert.NotNil(t, webhook.SetupWithManager)
}
