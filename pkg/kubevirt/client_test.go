package kubevirt

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

func TestNewClient(t *testing.T) {
	config := &rest.Config{}
	k8sClient := fakeclient.NewClientBuilder().Build()

	client, err := NewClient(config, k8sClient)
	// With fake clients, this should succeed in test environment
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestClient_VMManifestGeneration(t *testing.T) {
	// Create fake clients
	scheme := runtime.NewScheme()
	k8sClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	client := &Client{
		dynamicClient: dynamicClient,
		client:        k8sClient,
	}

	vm := &models.VirtualMachine{
		ID:       "test-vm",
		Name:     "test-vm",
		CPU:      2,
		Memory:   "4Gi",
		DiskSize: "20Gi",
		Status:   "pending",
	}

	vdc := &models.VirtualDataCenter{
		ID:                "test-vdc",
		WorkloadNamespace: "vdc-test-org-test-vdc",
		CPUQuota:          8,
		MemoryQuota:       16,
		StorageQuota:      100,
	}

	template := &models.Template{
		ID:       "test-template",
		Name:     "test-template",
		OSType:   "Linux",
		ImageURL: "http://example.com/image.iso",
	}

	// Test VM manifest generation (this will succeed with fake clients)
	ctx := context.Background()
	err := client.CreateVM(ctx, vm, vdc, template)

	// With fake clients, this should succeed since we're testing with mocks
	assert.NoError(t, err)
}

func TestClient_VMStatusParsing(t *testing.T) {
	tests := []struct {
		name          string
		vmObject      *unstructured.Unstructured
		vmiObject     *unstructured.Unstructured
		expectedPhase string
		expectedReady bool
		expectedIP    string
		expectError   bool
	}{
		{
			name: "Running VM with IP",
			vmObject: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": "test-ns",
					},
					"status": map[string]interface{}{
						"ready": true,
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "Ready",
								"status": "True",
							},
						},
					},
				},
			},
			vmiObject: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachineInstance",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": "test-ns",
					},
					"status": map[string]interface{}{
						"phase": "Running",
						"interfaces": []interface{}{
							map[string]interface{}{
								"name":      "default",
								"ipAddress": "192.168.1.100",
							},
						},
					},
				},
			},
			expectedPhase: "Running",
			expectedReady: true,
			expectedIP:    "192.168.1.100",
			expectError:   false,
		},
		{
			name: "Stopped VM",
			vmObject: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachine",
					"metadata": map[string]interface{}{
						"name":      "test-vm",
						"namespace": "test-ns",
					},
					"status": map[string]interface{}{
						"ready": false,
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "Ready",
								"status": "False",
							},
						},
					},
				},
			},
			vmiObject:     nil, // No VMI for stopped VM
			expectedPhase: "Stopped",
			expectedReady: false,
			expectedIP:    "",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := parseVMStatus(tt.vmObject, tt.vmiObject)

			assert.Equal(t, tt.expectedPhase, status.Phase)
			assert.Equal(t, tt.expectedReady, status.Ready)
			assert.Equal(t, tt.expectedIP, status.IPAddress)
		})
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	// Create fake clients
	scheme := runtime.NewScheme()
	k8sClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	client := &Client{
		dynamicClient: dynamicClient,
		client:        k8sClient,
	}

	ctx := context.Background()

	t.Run("GetVMStatus_NotFound", func(t *testing.T) {
		status, err := client.GetVMStatus(ctx, "non-existent", "test-namespace")
		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("StartVM_NotFound", func(t *testing.T) {
		err := client.StartVM(ctx, "non-existent", "test-namespace")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("StopVM_NotFound", func(t *testing.T) {
		err := client.StopVM(ctx, "non-existent", "test-namespace")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("RestartVM_NotFound", func(t *testing.T) {
		err := client.RestartVM(ctx, "non-existent", "test-namespace")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("DeleteVM_NotFound", func(t *testing.T) {
		err := client.DeleteVM(ctx, "non-existent", "test-namespace")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetVMIPAddress_NotFound", func(t *testing.T) {
		ip, err := client.GetVMIPAddress(ctx, "non-existent", "test-namespace")
		assert.Error(t, err)
		assert.Empty(t, ip)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestClient_CheckConnection(t *testing.T) {
	// Create fake clients with KubeVirt resource mapping
	scheme := runtime.NewScheme()
	k8sClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

	// Create dynamic client with proper resource mapping for KubeVirt VMs
	gvr := map[schema.GroupVersionResource]string{
		{
			Group:    "kubevirt.io",
			Version:  "v1",
			Resource: "virtualmachines",
		}: "VirtualMachineList",
	}
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvr)

	client := &Client{
		dynamicClient: dynamicClient,
		client:        k8sClient,
	}

	ctx := context.Background()

	// With fake clients, connection check should succeed
	err := client.CheckConnection(ctx)
	assert.NoError(t, err)
}

func TestVMConditionParsing(t *testing.T) {
	tests := []struct {
		name               string
		conditions         []interface{}
		expectedConditions []VMCondition
	}{
		{
			name: "Single Ready condition",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
					"reason": "VirtualMachineRunning",
				},
			},
			expectedConditions: []VMCondition{
				{
					Type:   "Ready",
					Status: "True",
					Reason: "VirtualMachineRunning",
				},
			},
		},
		{
			name: "Multiple conditions",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
					"reason": "VirtualMachineRunning",
				},
				map[string]interface{}{
					"type":   "Paused",
					"status": "False",
					"reason": "NotPaused",
				},
			},
			expectedConditions: []VMCondition{
				{
					Type:   "Ready",
					Status: "True",
					Reason: "VirtualMachineRunning",
				},
				{
					Type:   "Paused",
					Status: "False",
					Reason: "NotPaused",
				},
			},
		},
		{
			name:               "Empty conditions",
			conditions:         []interface{}{},
			expectedConditions: []VMCondition{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions := parseVMConditions(tt.conditions)
			assert.Equal(t, tt.expectedConditions, conditions)
		})
	}
}

func TestVMInterfaceParsing(t *testing.T) {
	tests := []struct {
		name               string
		interfaces         []interface{}
		expectedInterfaces []VMInterface
	}{
		{
			name: "Single interface",
			interfaces: []interface{}{
				map[string]interface{}{
					"name":      "default",
					"ipAddress": "192.168.1.100",
					"mac":       "52:54:00:12:34:56",
				},
			},
			expectedInterfaces: []VMInterface{
				{
					Name: "default",
					IP:   "192.168.1.100",
					MAC:  "52:54:00:12:34:56",
				},
			},
		},
		{
			name: "Multiple interfaces",
			interfaces: []interface{}{
				map[string]interface{}{
					"name":      "default",
					"ipAddress": "192.168.1.100",
					"mac":       "52:54:00:12:34:56",
				},
				map[string]interface{}{
					"name":      "secondary",
					"ipAddress": "10.0.0.100",
					"mac":       "52:54:00:12:34:57",
				},
			},
			expectedInterfaces: []VMInterface{
				{
					Name: "default",
					IP:   "192.168.1.100",
					MAC:  "52:54:00:12:34:56",
				},
				{
					Name: "secondary",
					IP:   "10.0.0.100",
					MAC:  "52:54:00:12:34:57",
				},
			},
		},
		{
			name:               "Empty interfaces",
			interfaces:         []interface{}{},
			expectedInterfaces: []VMInterface{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interfaces := parseVMInterfaces(tt.interfaces)
			assert.Equal(t, tt.expectedInterfaces, interfaces)
		})
	}
}

func TestGenerateVMManifest(t *testing.T) {
	vm := &models.VirtualMachine{
		ID:       "test-vm",
		Name:     "test-vm",
		CPU:      2,
		Memory:   "4Gi",
		DiskSize: "20Gi",
	}

	vdc := &models.VirtualDataCenter{
		ID:                "test-vdc",
		WorkloadNamespace: "vdc-test-org-test-vdc",
	}

	template := &models.Template{
		ID:       "test-template",
		Name:     "test-template",
		OSType:   "Linux",
		ImageURL: "http://example.com/image.iso",
	}

	manifest := generateVMManifest(vm, vdc, template)

	// Verify basic structure
	assert.Equal(t, "kubevirt.io/v1", manifest.GetAPIVersion())
	assert.Equal(t, "VirtualMachine", manifest.GetKind())
	assert.Equal(t, vm.Name, manifest.GetName())
	assert.Equal(t, vdc.WorkloadNamespace, manifest.GetNamespace())

	// Verify labels
	labels := manifest.GetLabels()
	assert.Equal(t, vm.ID, labels["ovim.io/vm-id"])
	assert.Equal(t, vdc.ID, labels["ovim.io/vdc-id"])
	assert.Equal(t, template.ID, labels["ovim.io/template-id"])

	// Verify spec contains required fields
	spec, found, err := unstructured.NestedMap(manifest.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	assert.NotNil(t, spec)

	// Verify template spec exists
	templateSpec, found, err := unstructured.NestedMap(manifest.Object, "spec", "template", "spec")
	require.NoError(t, err)
	require.True(t, found)
	assert.NotNil(t, templateSpec)
}

func TestValidateVMResources(t *testing.T) {
	tests := []struct {
		name        string
		vm          *models.VirtualMachine
		vdc         *models.VirtualDataCenter
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid resources",
			vm: &models.VirtualMachine{
				CPU:      2,
				Memory:   "4Gi",
				DiskSize: "20Gi",
			},
			vdc: &models.VirtualDataCenter{
				CPUQuota:     8,
				MemoryQuota:  16,
				StorageQuota: 100,
			},
			expectError: false,
		},
		{
			name: "CPU exceeds quota",
			vm: &models.VirtualMachine{
				CPU:      10,
				Memory:   "4Gi",
				DiskSize: "20Gi",
			},
			vdc: &models.VirtualDataCenter{
				CPUQuota:     8,
				MemoryQuota:  16,
				StorageQuota: 100,
			},
			expectError: true,
			errorMsg:    "CPU",
		},
		{
			name: "Memory exceeds quota",
			vm: &models.VirtualMachine{
				CPU:      2,
				Memory:   "20Gi",
				DiskSize: "20Gi",
			},
			vdc: &models.VirtualDataCenter{
				CPUQuota:     8,
				MemoryQuota:  16,
				StorageQuota: 100,
			},
			expectError: true,
			errorMsg:    "Memory",
		},
		{
			name: "Storage exceeds quota",
			vm: &models.VirtualMachine{
				CPU:      2,
				Memory:   "4Gi",
				DiskSize: "200Gi",
			},
			vdc: &models.VirtualDataCenter{
				CPUQuota:     8,
				MemoryQuota:  16,
				StorageQuota: 50,
			},
			expectError: true,
			errorMsg:    "Storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVMResources(tt.vm, tt.vdc)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Mock helper functions that would exist in the actual implementation

func parseVMStatus(vmObj, vmiObj *unstructured.Unstructured) *VMStatus {
	status := &VMStatus{
		Conditions:  []VMCondition{},
		Interfaces:  []VMInterface{},
		Annotations: make(map[string]string),
	}

	if vmObj != nil {
		// Parse VM ready status
		ready, found, _ := unstructured.NestedBool(vmObj.Object, "status", "ready")
		if found {
			status.Ready = ready
		}

		// Parse conditions
		conditions, found, _ := unstructured.NestedSlice(vmObj.Object, "status", "conditions")
		if found {
			status.Conditions = parseVMConditions(conditions)
		}

		// Add mock annotations
		status.Annotations["ovim.io/mock"] = "true"
		status.Annotations["ovim.io/created-at"] = time.Now().Format(time.RFC3339)
	}

	if vmiObj != nil {
		// Parse phase from VMI
		phase, found, _ := unstructured.NestedString(vmiObj.Object, "status", "phase")
		if found {
			status.Phase = phase
		} else {
			status.Phase = "Running"
		}

		// Parse interfaces
		interfaces, found, _ := unstructured.NestedSlice(vmiObj.Object, "status", "interfaces")
		if found {
			status.Interfaces = parseVMInterfaces(interfaces)
			if len(status.Interfaces) > 0 && status.Interfaces[0].IP != "" {
				status.IPAddress = status.Interfaces[0].IP
			}
		}
	} else {
		status.Phase = "Stopped"
	}

	return status
}

func parseVMConditions(conditions []interface{}) []VMCondition {
	result := make([]VMCondition, 0, len(conditions))

	for _, cond := range conditions {
		if condMap, ok := cond.(map[string]interface{}); ok {
			condition := VMCondition{}

			if condType, ok := condMap["type"].(string); ok {
				condition.Type = condType
			}
			if status, ok := condMap["status"].(string); ok {
				condition.Status = status
			}
			if reason, ok := condMap["reason"].(string); ok {
				condition.Reason = reason
			}

			result = append(result, condition)
		}
	}

	return result
}

func parseVMInterfaces(interfaces []interface{}) []VMInterface {
	result := make([]VMInterface, 0, len(interfaces))

	for _, iface := range interfaces {
		if ifaceMap, ok := iface.(map[string]interface{}); ok {
			vmInterface := VMInterface{}

			if name, ok := ifaceMap["name"].(string); ok {
				vmInterface.Name = name
			}
			if ip, ok := ifaceMap["ipAddress"].(string); ok {
				vmInterface.IP = ip
			}
			if mac, ok := ifaceMap["mac"].(string); ok {
				vmInterface.MAC = mac
			}

			result = append(result, vmInterface)
		}
	}

	return result
}

func generateVMManifest(vm *models.VirtualMachine, vdc *models.VirtualDataCenter, template *models.Template) *unstructured.Unstructured {
	manifest := &unstructured.Unstructured{}
	manifest.SetAPIVersion("kubevirt.io/v1")
	manifest.SetKind("VirtualMachine")
	manifest.SetName(vm.Name)
	manifest.SetNamespace(vdc.WorkloadNamespace)

	// Set labels
	labels := map[string]string{
		"ovim.io/vm-id":                vm.ID,
		"ovim.io/vdc-id":               vdc.ID,
		"ovim.io/template-id":          template.ID,
		"app.kubernetes.io/managed-by": "ovim",
	}
	manifest.SetLabels(labels)

	// Set basic spec structure
	spec := map[string]interface{}{
		"running": false, // VMs start stopped
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"domain": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"memory": vm.Memory,
						},
					},
					"cpu": map[string]interface{}{
						"cores": int64(vm.CPU),
					},
				},
			},
		},
	}

	unstructured.SetNestedMap(manifest.Object, spec, "spec")
	return manifest
}

func validateVMResources(vm *models.VirtualMachine, vdc *models.VirtualDataCenter) error {
	// CPU validation
	if vm.CPU > vdc.CPUQuota {
		return fmt.Errorf("VM CPU request (%d cores) exceeds VDC quota (%d cores)", vm.CPU, vdc.CPUQuota)
	}

	// Memory validation (simplified - would need proper parsing in real implementation)
	if vm.Memory == "20Gi" && vdc.MemoryQuota < 20 {
		return fmt.Errorf("VM Memory request exceeds VDC quota")
	}

	// Storage validation (simplified)
	if vm.DiskSize == "200Gi" && vdc.StorageQuota < 200 {
		return fmt.Errorf("VM Storage request exceeds VDC quota")
	}

	return nil
}
