package kubevirt

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// MockClient provides a mock implementation of VMProvisioner for testing and development
type MockClient struct {
	vms   map[string]*mockVM
	mutex sync.RWMutex
}

type mockVM struct {
	ID        string
	Namespace string
	Status    string
	IP        string
	CreatedAt time.Time
	Running   bool
}

// NewMockClient creates a new mock KubeVirt client
func NewMockClient() *MockClient {
	return &MockClient{
		vms: make(map[string]*mockVM),
	}
}

// CreateVM simulates creating a new virtual machine
func (m *MockClient) CreateVM(ctx context.Context, vm *models.VirtualMachine, vdc *models.VirtualDataCenter, template *models.Template) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	klog.V(4).Infof("Mock: Creating VM %s in namespace %s", vm.ID, vdc.Namespace)

	key := fmt.Sprintf("%s/%s", vdc.Namespace, vm.ID)
	if _, exists := m.vms[key]; exists {
		return fmt.Errorf("VM %s already exists in namespace %s", vm.ID, vdc.Namespace)
	}

	m.vms[key] = &mockVM{
		ID:        vm.ID,
		Namespace: vdc.Namespace,
		Status:    "Stopped",
		IP:        "",
		CreatedAt: time.Now(),
		Running:   false,
	}

	klog.Infof("Mock: Successfully created VM %s in namespace %s", vm.ID, vdc.Namespace)
	return nil
}

// GetVMStatus retrieves the current status of a mock virtual machine
func (m *MockClient) GetVMStatus(ctx context.Context, vmID, namespace string) (*VMStatus, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	klog.V(6).Infof("Mock: Getting status for VM %s in namespace %s", vmID, namespace)

	key := fmt.Sprintf("%s/%s", namespace, vmID)
	vm, exists := m.vms[key]
	if !exists {
		return nil, fmt.Errorf("VM %s not found in namespace %s", vmID, namespace)
	}

	status := &VMStatus{
		Phase:     vm.Status,
		Ready:     vm.Running,
		IPAddress: vm.IP,
		Conditions: []VMCondition{
			{
				Type:   "Ready",
				Status: func() string {
					if vm.Running {
						return "True"
					}
					return "False"
				}(),
				Reason: "Mock simulation",
			},
		},
		Interfaces: []VMInterface{
			{
				Name: "default",
				IP:   vm.IP,
				MAC:  "52:54:00:12:34:56",
			},
		},
		Annotations: map[string]string{
			"ovim.io/mock":        "true",
			"ovim.io/created-at":  vm.CreatedAt.Format(time.RFC3339),
		},
	}

	return status, nil
}

// StartVM simulates starting a virtual machine
func (m *MockClient) StartVM(ctx context.Context, vmID, namespace string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	klog.V(4).Infof("Mock: Starting VM %s in namespace %s", vmID, namespace)

	key := fmt.Sprintf("%s/%s", namespace, vmID)
	vm, exists := m.vms[key]
	if !exists {
		return fmt.Errorf("VM %s not found in namespace %s", vmID, namespace)
	}

	if vm.Running {
		return fmt.Errorf("VM %s is already running", vmID)
	}

	vm.Running = true
	vm.Status = "Running"
	vm.IP = fmt.Sprintf("192.168.1.%d", (len(m.vms)%254)+1) // Simulate IP assignment

	klog.Infof("Mock: Successfully started VM %s in namespace %s (IP: %s)", vmID, namespace, vm.IP)
	return nil
}

// StopVM simulates stopping a virtual machine
func (m *MockClient) StopVM(ctx context.Context, vmID, namespace string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	klog.V(4).Infof("Mock: Stopping VM %s in namespace %s", vmID, namespace)

	key := fmt.Sprintf("%s/%s", namespace, vmID)
	vm, exists := m.vms[key]
	if !exists {
		return fmt.Errorf("VM %s not found in namespace %s", vmID, namespace)
	}

	if !vm.Running {
		return fmt.Errorf("VM %s is already stopped", vmID)
	}

	vm.Running = false
	vm.Status = "Stopped"
	vm.IP = "" // Clear IP when stopped

	klog.Infof("Mock: Successfully stopped VM %s in namespace %s", vmID, namespace)
	return nil
}

// RestartVM simulates restarting a virtual machine
func (m *MockClient) RestartVM(ctx context.Context, vmID, namespace string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	klog.V(4).Infof("Mock: Restarting VM %s in namespace %s", vmID, namespace)

	key := fmt.Sprintf("%s/%s", namespace, vmID)
	vm, exists := m.vms[key]
	if !exists {
		return fmt.Errorf("VM %s not found in namespace %s", vmID, namespace)
	}

	if !vm.Running {
		return fmt.Errorf("VM %s must be running to restart", vmID)
	}

	// Simulate restart by keeping status but refreshing IP
	vm.IP = fmt.Sprintf("192.168.1.%d", (len(m.vms)%254)+1)

	klog.Infof("Mock: Successfully restarted VM %s in namespace %s (IP: %s)", vmID, namespace, vm.IP)
	return nil
}

// DeleteVM simulates deleting a virtual machine
func (m *MockClient) DeleteVM(ctx context.Context, vmID, namespace string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	klog.V(4).Infof("Mock: Deleting VM %s in namespace %s", vmID, namespace)

	key := fmt.Sprintf("%s/%s", namespace, vmID)
	if _, exists := m.vms[key]; !exists {
		return fmt.Errorf("VM %s not found in namespace %s", vmID, namespace)
	}

	delete(m.vms, key)

	klog.Infof("Mock: Successfully deleted VM %s in namespace %s", vmID, namespace)
	return nil
}

// GetVMIPAddress retrieves the IP address of a mock virtual machine
func (m *MockClient) GetVMIPAddress(ctx context.Context, vmID, namespace string) (string, error) {
	status, err := m.GetVMStatus(ctx, vmID, namespace)
	if err != nil {
		return "", err
	}

	if status.IPAddress == "" {
		return "", fmt.Errorf("VM %s does not have an IP address assigned", vmID)
	}

	return status.IPAddress, nil
}

// CheckConnection simulates checking connectivity to the cluster
func (m *MockClient) CheckConnection(ctx context.Context) error {
	klog.V(6).Info("Mock: Checking cluster connection (always succeeds)")
	return nil
}

// ListVMs returns all mock VMs for debugging
func (m *MockClient) ListVMs() map[string]*mockVM {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*mockVM)
	for k, v := range m.vms {
		result[k] = v
	}
	return result
}