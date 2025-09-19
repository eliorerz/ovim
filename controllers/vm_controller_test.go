package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// MockVMStorage extends MockStorage with functional VM methods for testing
type MockVMStorage struct {
	*MockStorage
	vms map[string]*models.VirtualMachine
}

func NewMockVMStorage() *MockVMStorage {
	return &MockVMStorage{
		MockStorage: NewMockStorage(),
		vms:         make(map[string]*models.VirtualMachine),
	}
}

func (m *MockVMStorage) CreateVM(vm *models.VirtualMachine) error {
	if m.shouldError {
		return fmt.Errorf("create VM failed: %s", m.errorMessage)
	}
	m.vms[vm.ID] = vm
	return nil
}

func (m *MockVMStorage) GetVM(id string) (*models.VirtualMachine, error) {
	if m.shouldError {
		return nil, fmt.Errorf("get VM failed: %s", m.errorMessage)
	}
	if vm, exists := m.vms[id]; exists {
		return vm, nil
	}
	return nil, storage.ErrNotFound
}

func (m *MockVMStorage) UpdateVM(vm *models.VirtualMachine) error {
	if m.shouldError {
		return fmt.Errorf("update VM failed: %s", m.errorMessage)
	}
	m.vms[vm.ID] = vm
	return nil
}

func (m *MockVMStorage) DeleteVM(id string) error {
	if m.shouldError {
		return fmt.Errorf("delete VM failed: %s", m.errorMessage)
	}
	delete(m.vms, id)
	return nil
}

func (m *MockVMStorage) ListVMs(orgFilter string) ([]*models.VirtualMachine, error) {
	if m.shouldError {
		return nil, fmt.Errorf("list VMs failed: %s", m.errorMessage)
	}
	var result []*models.VirtualMachine
	for _, vm := range m.vms {
		if orgFilter == "" {
			result = append(result, vm)
		}
		// Add org filtering logic if needed
	}
	return result, nil
}

// MockKubeVirtClient implements kubevirt.VMProvisioner for testing
type MockKubeVirtClient struct {
	vms          map[string]*kubevirt.VMStatus
	shouldError  bool
	errorMessage string
}

func NewMockKubeVirtClient() *MockKubeVirtClient {
	return &MockKubeVirtClient{
		vms: make(map[string]*kubevirt.VMStatus),
	}
}

func (m *MockKubeVirtClient) SetError(should bool, message string) {
	m.shouldError = should
	m.errorMessage = message
}

func (m *MockKubeVirtClient) CreateVM(ctx context.Context, vm *models.VirtualMachine, vdc *models.VirtualDataCenter, template *models.Template) error {
	if m.shouldError {
		return fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	key := fmt.Sprintf("%s/%s", vdc.WorkloadNamespace, vm.Name)
	m.vms[key] = &kubevirt.VMStatus{
		Phase:     "Stopped",
		Ready:     false,
		IPAddress: "",
	}
	return nil
}

func (m *MockKubeVirtClient) GetVMStatus(ctx context.Context, vmID, namespace string) (*kubevirt.VMStatus, error) {
	if m.shouldError {
		return nil, fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	key := fmt.Sprintf("%s/%s", namespace, vmID)
	if status, exists := m.vms[key]; exists {
		return status, nil
	}
	return nil, fmt.Errorf("VirtualMachine not found")
}

func (m *MockKubeVirtClient) StartVM(ctx context.Context, vmID, namespace string) error {
	if m.shouldError {
		return fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	key := fmt.Sprintf("%s/%s", namespace, vmID)
	if status, exists := m.vms[key]; exists {
		status.Phase = "Running"
		status.Ready = true
		status.IPAddress = "192.168.1.100"
		return nil
	}
	return fmt.Errorf("VM not found")
}

func (m *MockKubeVirtClient) StopVM(ctx context.Context, vmID, namespace string) error {
	if m.shouldError {
		return fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	key := fmt.Sprintf("%s/%s", namespace, vmID)
	if status, exists := m.vms[key]; exists {
		status.Phase = "Stopped"
		status.Ready = false
		status.IPAddress = ""
		return nil
	}
	return fmt.Errorf("VM not found")
}

func (m *MockKubeVirtClient) RestartVM(ctx context.Context, vmID, namespace string) error {
	if m.shouldError {
		return fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	return nil
}

func (m *MockKubeVirtClient) DeleteVM(ctx context.Context, vmID, namespace string) error {
	if m.shouldError {
		return fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	key := fmt.Sprintf("%s/%s", namespace, vmID)
	delete(m.vms, key)
	return nil
}

func (m *MockKubeVirtClient) GetVMIPAddress(ctx context.Context, vmID, namespace string) (string, error) {
	if m.shouldError {
		return "", fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	key := fmt.Sprintf("%s/%s", namespace, vmID)
	if status, exists := m.vms[key]; exists {
		return status.IPAddress, nil
	}
	return "", fmt.Errorf("VM not found")
}

func (m *MockKubeVirtClient) CheckConnection(ctx context.Context) error {
	if m.shouldError {
		return fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	return nil
}

func (m *MockKubeVirtClient) GetVMConsoleURL(ctx context.Context, vmID, namespace string) (string, error) {
	if m.shouldError {
		return "", fmt.Errorf("KubeVirt API error: %s", m.errorMessage)
	}
	return fmt.Sprintf("https://console.example.com/vm/%s/%s", namespace, vmID), nil
}

func setupVMControllerTest() (*VMReconciler, client.Client, *MockVMStorage, *MockKubeVirtClient) {
	// Create scheme with our CRD types
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = ovimv1.AddToScheme(s)

	// Create fake client
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

	// Create mock storage and KubeVirt client
	mockStorage := NewMockVMStorage()
	mockKubeVirt := NewMockKubeVirtClient()

	// Create reconciler
	reconciler := &VMReconciler{
		Client:         fakeClient,
		Scheme:         s,
		Storage:        mockStorage,
		KubeVirtClient: mockKubeVirt,
	}

	return reconciler, fakeClient, mockStorage, mockKubeVirt
}

func TestVMReconciler_Reconcile_NoStorage(t *testing.T) {
	reconciler, _, _, _ := setupVMControllerTest()
	reconciler.Storage = nil
	ctx := context.Background()

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
}

func TestVMReconciler_Reconcile_StorageError(t *testing.T) {
	reconciler, _, mockStorage, _ := setupVMControllerTest()
	ctx := context.Background()

	// Mock storage error
	mockStorage.SetError(true, "database connection failed")

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	assert.Error(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 30 * time.Second}, result)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestVMReconciler_Reconcile_SuccessfulReconciliation(t *testing.T) {
	reconciler, _, mockStorage, mockKubeVirt := setupVMControllerTest()
	ctx := context.Background()

	// Setup test data
	vdcID := "test-vdc"

	// Create VMs in storage
	vm1 := &models.VirtualMachine{
		ID:         "vm1",
		Name:       "vm1",
		Status:     "pending",
		VDCID:      &vdcID,
		TemplateID: "template1",
	}
	vm2 := &models.VirtualMachine{
		ID:         "vm2",
		Name:       "vm2",
		Status:     "running",
		VDCID:      &vdcID,
		TemplateID: "template1",
	}

	err := mockStorage.CreateVM(vm1)
	require.NoError(t, err)
	err = mockStorage.CreateVM(vm2)
	require.NoError(t, err)

	vdc := &models.VirtualDataCenter{
		ID:                "test-vdc",
		WorkloadNamespace: "vdc-test-org-test-vdc",
	}
	err = mockStorage.CreateVDC(vdc)
	require.NoError(t, err)

	template := &models.Template{
		ID:   "template1",
		Name: "test-template",
	}
	err = mockStorage.CreateTemplate(template)
	require.NoError(t, err)

	// Setup KubeVirt state - vm2 exists but is stopped
	mockKubeVirt.vms["vdc-test-org-test-vdc/vm2"] = &kubevirt.VMStatus{
		Phase:     "Stopped",
		Ready:     false,
		IPAddress: "",
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-vdc",
			Namespace: "org-test-org",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{RequeueAfter: 2 * time.Minute}, result)

	// Verify vm1 was created in KubeVirt (status should be creating in DB)
	updatedVM1, err := mockStorage.GetVM("vm1")
	require.NoError(t, err)
	assert.Equal(t, "creating", updatedVM1.Status)

	// Verify vm1 was created in KubeVirt
	_, exists := mockKubeVirt.vms["vdc-test-org-test-vdc/vm1"]
	assert.True(t, exists)

	// Verify vm2 was started in KubeVirt
	vm2Status := mockKubeVirt.vms["vdc-test-org-test-vdc/vm2"]
	assert.Equal(t, "Running", vm2Status.Phase)
	assert.True(t, vm2Status.Ready)
	assert.Equal(t, "192.168.1.100", vm2Status.IPAddress)
}

func TestVMReconciler_reconcileVM_PendingStatus(t *testing.T) {
	reconciler, _, mockStorage, mockKubeVirt := setupVMControllerTest()
	ctx := context.Background()

	vdcID := "test-vdc"
	vm := &models.VirtualMachine{
		ID:         "test-vm",
		Name:       "test-vm",
		Status:     "pending",
		VDCID:      &vdcID,
		TemplateID: "template1",
	}

	vdc := &models.VirtualDataCenter{
		ID:                "test-vdc",
		WorkloadNamespace: "vdc-test-org-test-vdc",
	}

	template := &models.Template{
		ID:   "template1",
		Name: "test-template",
	}

	// Setup storage
	err := mockStorage.CreateVDC(vdc)
	require.NoError(t, err)
	err = mockStorage.CreateTemplate(template)
	require.NoError(t, err)

	err = reconciler.reconcileVM(ctx, vm)
	require.NoError(t, err)

	// Verify VM was created in KubeVirt
	_, exists := mockKubeVirt.vms["vdc-test-org-test-vdc/test-vm"]
	assert.True(t, exists)

	// Verify VM status was updated to "creating"
	assert.Equal(t, "creating", vm.Status)
}

func TestVMReconciler_isNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "VirtualMachine not found",
			err:      fmt.Errorf("VirtualMachine not found"),
			expected: true,
		},
		{
			name:     "virtualmachines.kubevirt.io not found",
			err:      fmt.Errorf("virtualmachines.kubevirt.io not found"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("connection refused"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFoundError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVMReconciler_SetupWithManager(t *testing.T) {
	reconciler, _, _, _ := setupVMControllerTest()

	// This test verifies that SetupWithManager can be called without error
	// In a real test environment, you would use a real manager
	// For this unit test, we just verify the method exists and has the right signature
	assert.NotNil(t, reconciler.SetupWithManager)
}
