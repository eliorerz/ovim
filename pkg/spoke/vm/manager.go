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

	// TODO: Implement KubeVirt VM creation
	// For now, return a placeholder response
	status := &spoke.VMStatus{
		Name:      req.Name,
		Namespace: req.Namespace,
		Phase:     "Pending",
		Status:    "VM creation not yet implemented",
	}

	m.logger.Info("VM creation requested", "vm_name", req.Name, "status", status.Phase)
	return status, nil
}

// DeleteVM deletes a virtual machine
func (m *Manager) DeleteVM(ctx context.Context, namespace, name string) error {
	m.logger.Info("Deleting VM", "vm_name", name, "namespace", namespace)

	// TODO: Implement KubeVirt VM deletion
	// For now, just log the request
	m.logger.Info("VM deletion requested", "vm_name", name)
	return nil
}

// GetVMStatus returns the status of a virtual machine
func (m *Manager) GetVMStatus(ctx context.Context, namespace, name string) (*spoke.VMStatus, error) {
	m.logger.Debug("Getting VM status", "vm_name", name, "namespace", namespace)

	// TODO: Implement KubeVirt VM status retrieval
	// For now, return a placeholder response
	status := &spoke.VMStatus{
		Name:      name,
		Namespace: namespace,
		Phase:     "Unknown",
		Status:    "VM status retrieval not yet implemented",
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

	// TODO: Implement KubeVirt VM start
	// For now, just log the request
	m.logger.Info("VM start requested", "vm_name", name)
	return nil
}

// StopVM stops a virtual machine
func (m *Manager) StopVM(ctx context.Context, namespace, name string) error {
	m.logger.Info("Stopping VM", "vm_name", name, "namespace", namespace)

	// TODO: Implement KubeVirt VM stop
	// For now, just log the request
	m.logger.Info("VM stop requested", "vm_name", name)
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
	name := vm.GetName()
	namespace := vm.GetNamespace()
	labels := vm.GetLabels()
	createdAt := vm.GetCreationTimestamp().Time

	// Extract VM status and phase from the VM object
	status := "Unknown"
	phase := "Unknown"

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
		CPU:       cpu,
		Memory:    memory,
		Storage:   storage,
		Labels:    labels,
		CreatedAt: createdAt,
		UpdatedAt: time.Now(),
	}

	return vmStatus, nil
}
