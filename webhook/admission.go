package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WorkloadWebhook handles admission control for workloads and namespaces
type WorkloadWebhook struct {
	Client client.Client
}

// +kubebuilder:webhook:path=/validate-workloads,mutating=false,failurePolicy=fail,sideEffects=None,groups="";apps;kubevirt.io,resources=pods;deployments;statefulsets;daemonsets;virtualmachines,verbs=create;update,versions=v1,name=vworkload.ovim.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-namespaces,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=namespaces,verbs=create;update,versions=v1,name=vnamespace.ovim.io,admissionReviewVersions=v1

// Handle processes admission requests
func (w *WorkloadWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "workload", "kind", req.Kind.Kind, "namespace", req.Namespace)

	switch req.Kind.Kind {
	case "Pod":
		return w.handlePod(ctx, req)
	case "Deployment", "StatefulSet", "DaemonSet":
		return w.handleWorkload(ctx, req)
	case "VirtualMachine":
		return w.handleVirtualMachine(ctx, req)
	case "Namespace":
		return w.handleNamespace(ctx, req)
	default:
		logger.Info("Unsupported resource kind, allowing", "kind", req.Kind.Kind)
		return admission.Allowed("")
	}
}

// handlePod validates pod creation in organization and VDC namespaces
func (w *WorkloadWebhook) handlePod(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("operation", "pod")

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		logger.Error(err, "failed to decode pod")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Block workloads in org namespaces (should only be in VDC namespaces)
	if strings.HasPrefix(req.Namespace, "org-") && !strings.HasPrefix(req.Namespace, "vdc-") {
		logger.Info("Blocking pod creation in organization namespace", "namespace", req.Namespace)
		return admission.Denied("Workloads are not allowed in organization namespaces. Use VDC namespaces instead.")
	}

	// If this is a VDC namespace, ensure it's properly configured
	if strings.HasPrefix(req.Namespace, "vdc-") {
		if err := w.validateVDCNamespace(ctx, req.Namespace); err != nil {
			logger.Error(err, "VDC namespace validation failed")
			return admission.Denied(fmt.Sprintf("VDC namespace validation failed: %v", err))
		}
	}

	logger.Info("Pod creation allowed", "namespace", req.Namespace, "pod", pod.Name)
	return admission.Allowed("")
}

// handleWorkload validates workload creation (Deployment, StatefulSet, DaemonSet)
func (w *WorkloadWebhook) handleWorkload(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("operation", "workload", "kind", req.Kind.Kind)

	// Block workloads in org namespaces
	if strings.HasPrefix(req.Namespace, "org-") && !strings.HasPrefix(req.Namespace, "vdc-") {
		logger.Info("Blocking workload creation in organization namespace", "namespace", req.Namespace, "kind", req.Kind.Kind)
		return admission.Denied(fmt.Sprintf("%s workloads are not allowed in organization namespaces. Use VDC namespaces instead.", req.Kind.Kind))
	}

	// If this is a VDC namespace, ensure it's properly configured
	if strings.HasPrefix(req.Namespace, "vdc-") {
		if err := w.validateVDCNamespace(ctx, req.Namespace); err != nil {
			logger.Error(err, "VDC namespace validation failed")
			return admission.Denied(fmt.Sprintf("VDC namespace validation failed: %v", err))
		}
	}

	logger.Info("Workload creation allowed", "namespace", req.Namespace, "kind", req.Kind.Kind)
	return admission.Allowed("")
}

// handleVirtualMachine validates KubeVirt VirtualMachine creation
func (w *WorkloadWebhook) handleVirtualMachine(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("operation", "virtualmachine")

	var vm unstructured.Unstructured
	if err := json.Unmarshal(req.Object.Raw, &vm); err != nil {
		logger.Error(err, "failed to decode virtual machine")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Block VMs in org namespaces
	if strings.HasPrefix(req.Namespace, "org-") && !strings.HasPrefix(req.Namespace, "vdc-") {
		logger.Info("Blocking VM creation in organization namespace", "namespace", req.Namespace)
		return admission.Denied("Virtual Machines are not allowed in organization namespaces. Use VDC namespaces instead.")
	}

	// If this is a VDC namespace, ensure it's properly configured
	if strings.HasPrefix(req.Namespace, "vdc-") {
		if err := w.validateVDCNamespace(ctx, req.Namespace); err != nil {
			logger.Error(err, "VDC namespace validation failed")
			return admission.Denied(fmt.Sprintf("VDC namespace validation failed: %v", err))
		}
	}

	logger.Info("Virtual Machine creation allowed", "namespace", req.Namespace, "vm", vm.GetName())
	return admission.Allowed("")
}

// handleNamespace validates namespace creation and management
func (w *WorkloadWebhook) handleNamespace(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("operation", "namespace")

	var ns corev1.Namespace
	if err := json.Unmarshal(req.Object.Raw, &ns); err != nil {
		logger.Error(err, "failed to decode namespace")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Block direct creation of org/vdc namespaces unless managed by OVIM
	if (ns.Labels["type"] == "org" || ns.Labels["type"] == "vdc") && ns.Labels["managed-by"] != "ovim" {
		logger.Info("Blocking unauthorized org/vdc namespace creation", "namespace", ns.Name, "type", ns.Labels["type"])
		return admission.Denied("Organization and VDC namespaces can only be created by OVIM controllers")
	}

	// Validate naming conventions for OVIM-managed namespaces
	if ns.Labels["managed-by"] == "ovim" {
		if err := w.validateOVIMNamespace(ctx, &ns); err != nil {
			logger.Error(err, "OVIM namespace validation failed")
			return admission.Denied(fmt.Sprintf("OVIM namespace validation failed: %v", err))
		}
	}

	logger.Info("Namespace creation allowed", "namespace", ns.Name)
	return admission.Allowed("")
}

// validateVDCNamespace ensures VDC namespace is properly configured
func (w *WorkloadWebhook) validateVDCNamespace(ctx context.Context, namespaceName string) error {
	// Check if namespace exists and has proper labels
	ns := &corev1.Namespace{}
	if err := w.Client.Get(ctx, client.ObjectKey{Name: namespaceName}, ns); err != nil {
		return fmt.Errorf("VDC namespace %s not found or inaccessible: %w", namespaceName, err)
	}

	// Validate it's a properly managed VDC namespace
	if ns.Labels["type"] != "vdc" || ns.Labels["managed-by"] != "ovim" {
		return fmt.Errorf("namespace %s is not a properly managed VDC namespace", namespaceName)
	}

	// Check if it has a valid organization reference
	if ns.Labels["org"] == "" {
		return fmt.Errorf("VDC namespace %s missing organization reference", namespaceName)
	}

	return nil
}

// validateOVIMNamespace validates OVIM-managed namespace structure
func (w *WorkloadWebhook) validateOVIMNamespace(ctx context.Context, ns *corev1.Namespace) error {
	nsType := ns.Labels["type"]

	switch nsType {
	case "org":
		// Organization namespace validation
		if !strings.HasPrefix(ns.Name, "org-") {
			return fmt.Errorf("organization namespace must have 'org-' prefix")
		}
		if ns.Labels["org"] == "" {
			return fmt.Errorf("organization namespace missing 'org' label")
		}

	case "vdc":
		// VDC namespace validation
		if !strings.HasPrefix(ns.Name, "vdc-") {
			return fmt.Errorf("VDC namespace must have 'vdc-' prefix")
		}
		if ns.Labels["org"] == "" || ns.Labels["vdc"] == "" {
			return fmt.Errorf("VDC namespace missing 'org' or 'vdc' labels")
		}

		// Ensure parent organization namespace exists
		orgNamespace := fmt.Sprintf("org-%s", ns.Labels["org"])
		orgNS := &corev1.Namespace{}
		if err := w.Client.Get(ctx, client.ObjectKey{Name: orgNamespace}, orgNS); err != nil {
			return fmt.Errorf("parent organization namespace %s not found: %w", orgNamespace, err)
		}

	default:
		return fmt.Errorf("unsupported OVIM namespace type: %s", nsType)
	}

	// Validate required labels
	requiredLabels := []string{"app.kubernetes.io/name", "app.kubernetes.io/managed-by"}
	for _, label := range requiredLabels {
		if ns.Labels[label] == "" {
			return fmt.Errorf("required label '%s' missing", label)
		}
	}

	return nil
}

// SetupWithManager sets up the webhook with the Manager
func (w *WorkloadWebhook) SetupWithManager(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().Register("/validate-workloads", &admission.Webhook{
		Handler: w,
	})
	mgr.GetWebhookServer().Register("/validate-namespaces", &admission.Webhook{
		Handler: w,
	})
	return nil
}
