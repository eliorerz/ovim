package vm

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Manager implements the VMManager interface for KubeVirt VM management
type Manager struct {
	config        *config.SpokeConfig
	logger        *slog.Logger
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
}

var (
	// KubeVirt resource definitions
	vmGVR = schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}
	vmiGVR = schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachineinstances",
	}
)

// NewManager creates a new VM manager
func NewManager(cfg *config.SpokeConfig, logger *slog.Logger) (*Manager, error) {
	// Create Kubernetes client configuration
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
	}

	// Create Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create dynamic client for KubeVirt resources
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic Kubernetes client: %w", err)
	}

	return &Manager{
		config:        cfg,
		logger:        logger.With("component", "vm-manager"),
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		restConfig:    restConfig,
	}, nil
}

// CreateVM creates a new virtual machine
func (m *Manager) CreateVM(ctx context.Context, req *spoke.VMCreateRequest) (*spoke.VMStatus, error) {
	m.logger.Info("Creating VM", "vm_name", req.Name, "namespace", req.Namespace)

	// Create KubeVirt VirtualMachine resource
	vm := m.buildVirtualMachine(req)

	// Create the VM using dynamic client
	createdVM, err := m.dynamicClient.Resource(vmGVR).Namespace(req.Namespace).Create(ctx, vm, metav1.CreateOptions{})
	if err != nil {
		m.logger.Error("Failed to create VM", "vm_name", req.Name, "error", err)
		return nil, fmt.Errorf("failed to create VirtualMachine: %w", err)
	}

	m.logger.Info("VM created successfully", "vm_name", req.Name, "namespace", req.Namespace)

	// Convert created VM to VMStatus
	status, err := m.convertUnstructuredToVMStatus(createdVM)
	if err != nil {
		m.logger.Error("Failed to convert created VM to status", "vm_name", req.Name, "error", err)
		// Return a basic status if conversion fails
		return &spoke.VMStatus{
			Name:      req.Name,
			Namespace: req.Namespace,
			Phase:     "Pending",
			Status:    "Created",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	return status, nil
}

// DeleteVM deletes a virtual machine
func (m *Manager) DeleteVM(ctx context.Context, namespace, name string) error {
	m.logger.Info("Deleting VM", "vm_name", name, "namespace", namespace)

	// Check if VM exists first
	_, err := m.dynamicClient.Resource(vmGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		m.logger.Error("VM not found for deletion", "vm_name", name, "namespace", namespace, "error", err)
		return fmt.Errorf("VM %s not found in namespace %s: %w", name, namespace, err)
	}

	// Delete the VirtualMachine resource
	err = m.dynamicClient.Resource(vmGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: func() *int64 { v := int64(30); return &v }(), // 30 second grace period
	})
	if err != nil {
		m.logger.Error("Failed to delete VM", "vm_name", name, "namespace", namespace, "error", err)
		return fmt.Errorf("failed to delete VirtualMachine %s: %w", name, err)
	}

	m.logger.Info("VM deletion initiated successfully", "vm_name", name, "namespace", namespace)
	return nil
}

// GetVMStatus returns the status of a virtual machine
func (m *Manager) GetVMStatus(ctx context.Context, namespace, name string) (*spoke.VMStatus, error) {
	m.logger.Debug("Getting VM status", "vm_name", name, "namespace", namespace)

	// Get the VM using dynamic client
	vm, err := m.dynamicClient.Resource(vmGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		m.logger.Error("VM not found for status retrieval", "vm_name", name, "namespace", namespace, "error", err)
		return nil, fmt.Errorf("VM %s not found in namespace %s: %w", name, namespace, err)
	}

	// Convert to VMStatus
	status, err := m.convertUnstructuredToVMStatus(vm)
	if err != nil {
		m.logger.Error("Failed to convert VM to status", "vm_name", name, "error", err)
		return nil, fmt.Errorf("failed to convert VM %s to status: %w", name, err)
	}

	return status, nil
}

// ListVMs returns a list of virtual machines
func (m *Manager) ListVMs(ctx context.Context) ([]spoke.VMStatus, error) {
	m.logger.Debug("Listing VMs")

	// List all VirtualMachines across all namespaces
	vmList, err := m.dynamicClient.Resource(vmGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		m.logger.Error("Failed to list VirtualMachines", "error", err)
		return nil, fmt.Errorf("failed to list VirtualMachines: %w", err)
	}

	var vmStatuses []spoke.VMStatus
	for _, vm := range vmList.Items {
		vmStatus, err := m.convertUnstructuredToVMStatus(&vm)
		if err != nil {
			m.logger.Warn("Failed to convert VM to status", "vm_name", vm.GetName(), "error", err)
			continue
		}
		vmStatuses = append(vmStatuses, *vmStatus)
	}

	m.logger.Debug("Listed VMs successfully", "count", len(vmStatuses))
	return vmStatuses, nil
}

// StartVM starts a virtual machine
func (m *Manager) StartVM(ctx context.Context, namespace, name string) error {
	m.logger.Info("Starting VM", "vm_name", name, "namespace", namespace)

	// Get the current VM
	vm, err := m.dynamicClient.Resource(vmGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		m.logger.Error("VM not found for start", "vm_name", name, "namespace", namespace, "error", err)
		return fmt.Errorf("VM %s not found in namespace %s: %w", name, namespace, err)
	}

	// Set running to true in the spec
	if err := unstructured.SetNestedField(vm.Object, true, "spec", "running"); err != nil {
		m.logger.Error("Failed to set running field", "vm_name", name, "error", err)
		return fmt.Errorf("failed to set running field for VM %s: %w", name, err)
	}

	// Update the VM
	_, err = m.dynamicClient.Resource(vmGVR).Namespace(namespace).Update(ctx, vm, metav1.UpdateOptions{})
	if err != nil {
		m.logger.Error("Failed to start VM", "vm_name", name, "namespace", namespace, "error", err)
		return fmt.Errorf("failed to start VM %s: %w", name, err)
	}

	m.logger.Info("VM start initiated successfully", "vm_name", name, "namespace", namespace)
	return nil
}

// StopVM stops a virtual machine
func (m *Manager) StopVM(ctx context.Context, namespace, name string) error {
	m.logger.Info("Stopping VM", "vm_name", name, "namespace", namespace)

	// Get the current VM
	vm, err := m.dynamicClient.Resource(vmGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		m.logger.Error("VM not found for stop", "vm_name", name, "namespace", namespace, "error", err)
		return fmt.Errorf("VM %s not found in namespace %s: %w", name, namespace, err)
	}

	// Set running to false in the spec
	if err := unstructured.SetNestedField(vm.Object, false, "spec", "running"); err != nil {
		m.logger.Error("Failed to set running field", "vm_name", name, "error", err)
		return fmt.Errorf("failed to set running field for VM %s: %w", name, err)
	}

	// Update the VM
	_, err = m.dynamicClient.Resource(vmGVR).Namespace(namespace).Update(ctx, vm, metav1.UpdateOptions{})
	if err != nil {
		m.logger.Error("Failed to stop VM", "vm_name", name, "namespace", namespace, "error", err)
		return fmt.Errorf("failed to stop VM %s: %w", name, err)
	}

	m.logger.Info("VM stop initiated successfully", "vm_name", name, "namespace", namespace)
	return nil
}

// WatchVMs returns a channel for VM status updates
func (m *Manager) WatchVMs(ctx context.Context) (<-chan spoke.VMStatus, error) {
	m.logger.Debug("Starting VM watch")

	// TODO: Implement KubeVirt VM watching
	// For now, return a channel that will be closed immediately
	ch := make(chan spoke.VMStatus)
	go func() {
		defer close(ch)
		// No VMs to watch for now
	}()

	return ch, nil
}

// ValidateConfiguration validates the VM manager configuration
func (m *Manager) ValidateConfiguration() error {
	if m.kubeClient == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	if m.dynamicClient == nil {
		return fmt.Errorf("dynamic kubernetes client is not initialized")
	}

	// Test connectivity to Kubernetes API
	_, err := m.kubeClient.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}

	m.logger.Info("VM manager configuration validated successfully")
	return nil
}

// convertUnstructuredToVMStatus converts an unstructured KubeVirt VM to VMStatus
func (m *Manager) convertUnstructuredToVMStatus(vm *unstructured.Unstructured) (*spoke.VMStatus, error) {
	return m.convertUnstructuredToVMStatusWithContext(context.Background(), vm)
}

// convertUnstructuredToVMStatusWithContext converts an unstructured KubeVirt VM to VMStatus with context
func (m *Manager) convertUnstructuredToVMStatusWithContext(ctx context.Context, vm *unstructured.Unstructured) (*spoke.VMStatus, error) {
	name := vm.GetName()
	namespace := vm.GetNamespace()
	labels := vm.GetLabels()
	createdAt := vm.GetCreationTimestamp().Time

	// Extract VM status and phase from the VM object
	status := "Unknown"
	phase := "Unknown"
	nodeName := ""
	ipAddress := ""

	// Get status from the VM object
	if statusMap, found, err := unstructured.NestedMap(vm.Object, "status"); found && err == nil {
		if conditionsRaw, found, err := unstructured.NestedSlice(statusMap, "conditions"); found && err == nil {
			for _, conditionRaw := range conditionsRaw {
				if condition, ok := conditionRaw.(map[string]interface{}); ok {
					if condType, found := condition["type"].(string); found && condType == "Ready" {
						if condStatus, found := condition["status"].(string); found {
							if condStatus == "True" {
								status = "Running"
							} else {
								status = "NotReady"
							}
						}
					}
				}
			}
		}

		// Try to get phase from printableStatus
		if printableStatus, found, err := unstructured.NestedString(statusMap, "printableStatus"); found && err == nil {
			phase = printableStatus
		}
	}

	// Try to get VMI status if VM is running
	if status == "Running" || status == "Starting" {
		// Look for corresponding VMI (VirtualMachineInstance)
		vmiList, err := m.dynamicClient.Resource(vmiGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "kubevirt.io/created-by=" + name,
		})
		if err == nil && len(vmiList.Items) > 0 {
			vmi := &vmiList.Items[0]

			// Extract node name from VMI
			if vmiStatus, found, err := unstructured.NestedMap(vmi.Object, "status"); found && err == nil {
				if node, found, err := unstructured.NestedString(vmiStatus, "nodeName"); found && err == nil {
					nodeName = node
				}

				// Extract IP addresses from VMI interfaces
				if interfaces, found, err := unstructured.NestedSlice(vmiStatus, "interfaces"); found && err == nil {
					for _, iface := range interfaces {
						if ifaceMap, ok := iface.(map[string]interface{}); ok {
							if ip, found, err := unstructured.NestedString(ifaceMap, "ipAddress"); found && err == nil && ip != "" {
								ipAddress = ip
								break // Use first available IP
							}
						}
					}
				}
			}
		}
	}

	// If we couldn't determine status from conditions, check if VM is started
	if status == "Unknown" {
		if spec, found, err := unstructured.NestedMap(vm.Object, "spec"); found && err == nil {
			if running, found, err := unstructured.NestedBool(spec, "running"); found && err == nil {
				if running {
					status = "Starting"
					phase = "Starting"
				} else {
					status = "Stopped"
					phase = "Stopped"
				}
			}
		}
	}

	// Extract resource specifications
	cpu := ""
	memory := ""
	storage := ""

	if spec, found, err := unstructured.NestedMap(vm.Object, "spec"); found && err == nil {
		if template, found, err := unstructured.NestedMap(spec, "template"); found && err == nil {
			if templateSpec, found, err := unstructured.NestedMap(template, "spec"); found && err == nil {
				if domain, found, err := unstructured.NestedMap(templateSpec, "domain"); found && err == nil {
					if resources, found, err := unstructured.NestedMap(domain, "resources"); found && err == nil {
						if requests, found, err := unstructured.NestedMap(resources, "requests"); found && err == nil {
							if cpuVal, found, err := unstructured.NestedString(requests, "cpu"); found && err == nil {
								cpu = cpuVal
							}
							if memVal, found, err := unstructured.NestedString(requests, "memory"); found && err == nil {
								memory = memVal
							}
						}
					}
				}
			}
		}
	}

	vmStatus := &spoke.VMStatus{
		Name:      name,
		Namespace: namespace,
		Status:    status,
		Phase:     phase,
		NodeName:  nodeName,
		IPAddress: ipAddress,
		CPU:       cpu,
		Memory:    memory,
		Storage:   storage,
		Labels:    labels,
		CreatedAt: createdAt,
		UpdatedAt: time.Now(),
	}

	return vmStatus, nil
}

// buildVirtualMachine creates a KubeVirt VirtualMachine resource from the request
func (m *Manager) buildVirtualMachine(req *spoke.VMCreateRequest) *unstructured.Unstructured {
	// Build basic VM structure
	vm := &unstructured.Unstructured{}
	vm.SetAPIVersion("kubevirt.io/v1")
	vm.SetKind("VirtualMachine")
	vm.SetName(req.Name)
	vm.SetNamespace(req.Namespace)

	// Set labels
	labels := make(map[string]string)
	if req.Labels != nil {
		for k, v := range req.Labels {
			labels[k] = v
		}
	}
	labels["app.kubernetes.io/managed-by"] = "ovim"
	labels["ovim.io/vm-name"] = req.Name
	vm.SetLabels(labels)

	// Set annotations
	annotations := make(map[string]string)
	if req.Annotations != nil {
		for k, v := range req.Annotations {
			annotations[k] = v
		}
	}
	annotations["ovim.io/template"] = req.TemplateName
	vm.SetAnnotations(annotations)

	// Build VM spec
	spec := map[string]interface{}{
		"running": true,
		"template": map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": labels,
			},
			"spec": map[string]interface{}{
				"domain": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    req.CPU,
							"memory": req.Memory,
						},
					},
					"devices": map[string]interface{}{
						"disks": []interface{}{
							map[string]interface{}{
								"name": "disk0",
								"disk": map[string]interface{}{
									"bus": "virtio",
								},
							},
						},
						"interfaces": []interface{}{
							map[string]interface{}{
								"name":       "default",
								"masquerade": map[string]interface{}{},
							},
						},
					},
				},
				"networks": []interface{}{
					map[string]interface{}{
						"name": "default",
						"pod":  map[string]interface{}{},
					},
				},
				"volumes": []interface{}{
					map[string]interface{}{
						"name": "disk0",
						"containerDisk": map[string]interface{}{
							"image": m.getTemplateImage(req.TemplateName),
						},
					},
				},
			},
		},
	}

	// Add network configuration if specified
	if req.NetworkName != "" {
		networks := spec["template"].(map[string]interface{})["spec"].(map[string]interface{})["networks"].([]interface{})
		networks[0] = map[string]interface{}{
			"name": "default",
			"multus": map[string]interface{}{
				"networkName": req.NetworkName,
			},
		}
	}

	vm.Object["spec"] = spec
	return vm
}

// getTemplateImage returns the container image for a template
func (m *Manager) getTemplateImage(templateName string) string {
	// Default images for common templates
	templateImages := map[string]string{
		"ubuntu-20.04":  "quay.io/kubevirt/ubuntu-cloud-container-disk-demo:20.04",
		"ubuntu-22.04":  "quay.io/kubevirt/ubuntu-cloud-container-disk-demo:22.04",
		"centos-7":      "quay.io/kubevirt/centos7-container-disk-demo:latest",
		"centos-8":      "quay.io/kubevirt/centos8-container-disk-demo:latest",
		"fedora-latest": "quay.io/kubevirt/fedora-cloud-container-disk-demo:latest",
	}

	if image, exists := templateImages[templateName]; exists {
		return image
	}

	// Default fallback image
	return "quay.io/kubevirt/ubuntu-cloud-container-disk-demo:22.04"
}
