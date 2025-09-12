package kubevirt

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

func TestNewMockClient(t *testing.T) {
	client := NewMockClient()
	assert.NotNil(t, client)
	assert.NotNil(t, client.vms)
	assert.Len(t, client.vms, 0)
}

func TestMockClient_CreateVM(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	vm := &models.VirtualMachine{
		ID:     "test-vm",
		Name:   "test-vm",
		Status: "pending",
	}

	vdc := &models.VirtualDataCenter{
		ID:                "test-vdc",
		WorkloadNamespace: "vdc-test-org-test-vdc",
	}

	template := &models.Template{
		ID:   "test-template",
		Name: "test-template",
	}

	// Test successful VM creation
	err := client.CreateVM(ctx, vm, vdc, template)
	require.NoError(t, err)

	// Verify VM was created with correct properties
	key := "vdc-test-org-test-vdc/test-vm"
	mockVM, exists := client.vms[key]
	require.True(t, exists)
	assert.Equal(t, "test-vm", mockVM.ID)
	assert.Equal(t, "vdc-test-org-test-vdc", mockVM.Namespace)
	assert.Equal(t, "Stopped", mockVM.Status)
	assert.Equal(t, "", mockVM.IP)
	assert.False(t, mockVM.Running)
	assert.WithinDuration(t, time.Now(), mockVM.CreatedAt, time.Second)

	// Test creating duplicate VM (should fail)
	err = client.CreateVM(ctx, vm, vdc, template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestMockClient_GetVMStatus(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Test VM not found
	status, err := client.GetVMStatus(ctx, "non-existent", "test-namespace")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "not found")

	// Create VM first
	key := "test-namespace/test-vm"
	client.vms[key] = &mockVM{
		ID:        "test-vm",
		Namespace: "test-namespace",
		Status:    "Running",
		IP:        "192.168.1.100",
		CreatedAt: time.Now(),
		Running:   true,
	}

	// Test successful status retrieval
	status, err = client.GetVMStatus(ctx, "test-vm", "test-namespace")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "Running", status.Phase)
	assert.True(t, status.Ready)
	assert.Equal(t, "192.168.1.100", status.IPAddress)

	// Verify conditions
	require.Len(t, status.Conditions, 1)
	condition := status.Conditions[0]
	assert.Equal(t, "Ready", condition.Type)
	assert.Equal(t, "True", condition.Status)
	assert.Equal(t, "Mock simulation", condition.Reason)

	// Verify interfaces
	require.Len(t, status.Interfaces, 1)
	iface := status.Interfaces[0]
	assert.Equal(t, "default", iface.Name)
	assert.Equal(t, "192.168.1.100", iface.IP)
	assert.Equal(t, "52:54:00:12:34:56", iface.MAC)

	// Verify annotations
	assert.Equal(t, "true", status.Annotations["ovim.io/mock"])
	assert.NotEmpty(t, status.Annotations["ovim.io/created-at"])
}

func TestMockClient_GetVMStatus_StoppedVM(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Create stopped VM
	key := "test-namespace/stopped-vm"
	client.vms[key] = &mockVM{
		ID:        "stopped-vm",
		Namespace: "test-namespace",
		Status:    "Stopped",
		IP:        "",
		CreatedAt: time.Now(),
		Running:   false,
	}

	status, err := client.GetVMStatus(ctx, "stopped-vm", "test-namespace")
	require.NoError(t, err)

	assert.Equal(t, "Stopped", status.Phase)
	assert.False(t, status.Ready)
	assert.Equal(t, "", status.IPAddress)

	// Verify condition for stopped VM
	require.Len(t, status.Conditions, 1)
	condition := status.Conditions[0]
	assert.Equal(t, "Ready", condition.Type)
	assert.Equal(t, "False", condition.Status)
}

func TestMockClient_StartVM(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Test VM not found
	err := client.StartVM(ctx, "non-existent", "test-namespace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Create stopped VM
	key := "test-namespace/test-vm"
	client.vms[key] = &mockVM{
		ID:        "test-vm",
		Namespace: "test-namespace",
		Status:    "Stopped",
		IP:        "",
		CreatedAt: time.Now(),
		Running:   false,
	}

	// Test successful start
	err = client.StartVM(ctx, "test-vm", "test-namespace")
	require.NoError(t, err)

	vm := client.vms[key]
	assert.True(t, vm.Running)
	assert.Equal(t, "Running", vm.Status)
	assert.NotEmpty(t, vm.IP)
	assert.Contains(t, vm.IP, "192.168.1.")

	// Test starting already running VM (should fail)
	err = client.StartVM(ctx, "test-vm", "test-namespace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestMockClient_StopVM(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Test VM not found
	err := client.StopVM(ctx, "non-existent", "test-namespace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Create running VM
	key := "test-namespace/test-vm"
	client.vms[key] = &mockVM{
		ID:        "test-vm",
		Namespace: "test-namespace",
		Status:    "Running",
		IP:        "192.168.1.100",
		CreatedAt: time.Now(),
		Running:   true,
	}

	// Test successful stop
	err = client.StopVM(ctx, "test-vm", "test-namespace")
	require.NoError(t, err)

	vm := client.vms[key]
	assert.False(t, vm.Running)
	assert.Equal(t, "Stopped", vm.Status)
	assert.Equal(t, "", vm.IP)

	// Test stopping already stopped VM (should fail)
	err = client.StopVM(ctx, "test-vm", "test-namespace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already stopped")
}

func TestMockClient_RestartVM(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Test VM not found
	err := client.RestartVM(ctx, "non-existent", "test-namespace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test restart stopped VM (should fail)
	key := "test-namespace/test-vm"
	client.vms[key] = &mockVM{
		ID:        "test-vm",
		Namespace: "test-namespace",
		Status:    "Stopped",
		IP:        "",
		CreatedAt: time.Now(),
		Running:   false,
	}

	err = client.RestartVM(ctx, "test-vm", "test-namespace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be running to restart")

	// Create running VM
	originalIP := "192.168.1.100"
	client.vms[key] = &mockVM{
		ID:        "test-vm",
		Namespace: "test-namespace",
		Status:    "Running",
		IP:        originalIP,
		CreatedAt: time.Now(),
		Running:   true,
	}

	// Test successful restart
	err = client.RestartVM(ctx, "test-vm", "test-namespace")
	require.NoError(t, err)

	vm := client.vms[key]
	assert.True(t, vm.Running)
	assert.Equal(t, "Running", vm.Status)
	assert.NotEmpty(t, vm.IP)
	// IP should be refreshed (potentially different)
	assert.Contains(t, vm.IP, "192.168.1.")
}

func TestMockClient_DeleteVM(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Test VM not found
	err := client.DeleteVM(ctx, "non-existent", "test-namespace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Create VM
	key := "test-namespace/test-vm"
	client.vms[key] = &mockVM{
		ID:        "test-vm",
		Namespace: "test-namespace",
		Status:    "Running",
		IP:        "192.168.1.100",
		CreatedAt: time.Now(),
		Running:   true,
	}

	// Verify VM exists
	_, exists := client.vms[key]
	assert.True(t, exists)

	// Test successful deletion
	err = client.DeleteVM(ctx, "test-vm", "test-namespace")
	require.NoError(t, err)

	// Verify VM was deleted
	_, exists = client.vms[key]
	assert.False(t, exists)
}

func TestMockClient_GetVMIPAddress(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Test VM not found
	ip, err := client.GetVMIPAddress(ctx, "non-existent", "test-namespace")
	assert.Error(t, err)
	assert.Empty(t, ip)

	// Create VM without IP
	key := "test-namespace/test-vm"
	client.vms[key] = &mockVM{
		ID:        "test-vm",
		Namespace: "test-namespace",
		Status:    "Stopped",
		IP:        "",
		CreatedAt: time.Now(),
		Running:   false,
	}

	// Test VM without IP
	ip, err = client.GetVMIPAddress(ctx, "test-vm", "test-namespace")
	assert.Error(t, err)
	assert.Empty(t, ip)
	assert.Contains(t, err.Error(), "does not have an IP address assigned")

	// Create VM with IP
	client.vms[key].IP = "192.168.1.100"
	client.vms[key].Running = true

	// Test successful IP retrieval
	ip, err = client.GetVMIPAddress(ctx, "test-vm", "test-namespace")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestMockClient_CheckConnection(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Mock client connection always succeeds
	err := client.CheckConnection(ctx)
	assert.NoError(t, err)
}

func TestMockClient_ListVMs(t *testing.T) {
	client := NewMockClient()

	// Test empty list
	vms := client.ListVMs()
	assert.NotNil(t, vms)
	assert.Len(t, vms, 0)

	// Add some VMs
	client.vms["ns1/vm1"] = &mockVM{
		ID:        "vm1",
		Namespace: "ns1",
		Status:    "Running",
		Running:   true,
	}
	client.vms["ns2/vm2"] = &mockVM{
		ID:        "vm2",
		Namespace: "ns2",
		Status:    "Stopped",
		Running:   false,
	}

	// Test list with VMs
	vms = client.ListVMs()
	assert.Len(t, vms, 2)

	vm1, exists := vms["ns1/vm1"]
	assert.True(t, exists)
	assert.Equal(t, "vm1", vm1.ID)
	assert.Equal(t, "ns1", vm1.Namespace)
	assert.True(t, vm1.Running)

	vm2, exists := vms["ns2/vm2"]
	assert.True(t, exists)
	assert.Equal(t, "vm2", vm2.ID)
	assert.Equal(t, "ns2", vm2.Namespace)
	assert.False(t, vm2.Running)
}

func TestMockClient_IPAssignment(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Create multiple VMs to test IP assignment
	for i := 1; i <= 5; i++ {
		vm := &models.VirtualMachine{
			ID:   fmt.Sprintf("vm%d", i),
			Name: fmt.Sprintf("vm%d", i),
		}
		vdc := &models.VirtualDataCenter{
			WorkloadNamespace: "test-namespace",
		}
		template := &models.Template{ID: "test-template"}

		err := client.CreateVM(ctx, vm, vdc, template)
		require.NoError(t, err)

		err = client.StartVM(ctx, vm.ID, vdc.WorkloadNamespace)
		require.NoError(t, err)
	}

	// Verify each VM got a unique IP in the 192.168.1.x range
	ips := make(map[string]bool)
	for i := 1; i <= 5; i++ {
		ip, err := client.GetVMIPAddress(ctx, fmt.Sprintf("vm%d", i), "test-namespace")
		require.NoError(t, err)
		assert.Contains(t, ip, "192.168.1.")

		// Ensure IP is unique
		assert.False(t, ips[ip], "IP %s was assigned to multiple VMs", ip)
		ips[ip] = true
	}
}

func TestMockClient_ConcurrentAccess(t *testing.T) {
	client := NewMockClient()
	ctx := context.Background()

	// Test concurrent access to mock client
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			vmID := fmt.Sprintf("vm%d", id)
			namespace := "test-namespace"

			vm := &models.VirtualMachine{ID: vmID, Name: vmID}
			vdc := &models.VirtualDataCenter{WorkloadNamespace: namespace}
			template := &models.Template{ID: "test-template"}

			// Create VM
			err := client.CreateVM(ctx, vm, vdc, template)
			assert.NoError(t, err)

			// Start VM
			err = client.StartVM(ctx, vmID, namespace)
			assert.NoError(t, err)

			// Get status
			status, err := client.GetVMStatus(ctx, vmID, namespace)
			assert.NoError(t, err)
			assert.NotNil(t, status)

			// Stop VM
			err = client.StopVM(ctx, vmID, namespace)
			assert.NoError(t, err)

			// Delete VM
			err = client.DeleteVM(ctx, vmID, namespace)
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all VMs were cleaned up
	vms := client.ListVMs()
	assert.Len(t, vms, 0)
}
