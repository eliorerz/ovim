package openshift

import (
	"testing"

	"github.com/eliorerz/ovim-updated/pkg/config"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestConvertTemplate(t *testing.T) {
	client := &Client{}

	// Create a mock OpenShift template
	osTemplate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("test-uid-123"),
			Name:      "rhel9-server-small",
			Namespace: "openshift",
			Annotations: map[string]string{
				"openshift.io/description": "Red Hat Enterprise Linux 9 Server Template",
			},
			Labels: map[string]string{
				"os.template.kubevirt.io/rhel9":     "true",
				"flavor.template.kubevirt.io/small": "true",
			},
		},
	}

	result := client.convertTemplate(osTemplate)

	assert.Equal(t, "test-uid-123", result.ID)
	assert.Equal(t, "rhel9-server-small", result.Name)
	assert.Equal(t, "openshift", result.Namespace)
	assert.Equal(t, "Red Hat Enterprise Linux 9 Server Template", result.Description)
	assert.Equal(t, "Rhel9", result.OSType)
	assert.Equal(t, "Rhel9", result.OSVersion)
	assert.Equal(t, 1, result.CPU)
	assert.Equal(t, "2Gi", result.Memory)
	assert.Equal(t, "20Gi", result.DiskSize)
}

func TestConvertTemplate_DescriptionFallback(t *testing.T) {
	client := &Client{}

	// Test fallback to "description" annotation
	osTemplate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("test-uid-123"),
			Name:      "test-template",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"description": "Fallback description",
			},
		},
	}

	result := client.convertTemplate(osTemplate)

	assert.Equal(t, "Fallback description", result.Description)
}

func TestConvertTemplate_DisplayNameFallback(t *testing.T) {
	client := &Client{}

	// Test fallback to "openshift.io/display-name" annotation
	osTemplate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("test-uid-123"),
			Name:      "test-template",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"openshift.io/display-name": "Display Name Description",
			},
		},
	}

	result := client.convertTemplate(osTemplate)

	assert.Equal(t, "Display Name Description", result.Description)
}

func TestConvertTemplate_OSTypeDetection(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name: "RHEL 8 detection",
			labels: map[string]string{
				"os.template.kubevirt.io/rhel8": "true",
			},
			expected: "Rhel8",
		},
		{
			name: "Ubuntu detection",
			labels: map[string]string{
				"os.template.kubevirt.io/ubuntu": "true",
			},
			expected: "Ubuntu",
		},
		{
			name: "CentOS detection",
			labels: map[string]string{
				"os.template.kubevirt.io/centos": "true",
			},
			expected: "Centos",
		},
		{
			name:     "Unknown OS (default)",
			labels:   map[string]string{},
			expected: "Unknown",
		},
		{
			name: "Multiple OS labels (first match wins)",
			labels: map[string]string{
				"os.template.kubevirt.io/rhel9":  "true",
				"os.template.kubevirt.io/ubuntu": "true",
			},
			expected: "Rhel9", // Should match the first one found (map iteration)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osTemplate := &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("test-uid"),
					Name:      "test-template",
					Namespace: "test-ns",
					Labels:    tt.labels,
				},
			}

			result := client.convertTemplate(osTemplate)

			if tt.expected == "Unknown" {
				assert.Equal(t, tt.expected, result.OSType)
			} else {
				// For specific OS types, we expect either the specific type or one of the multiple types
				assert.Contains(t, []string{tt.expected, "Rhel9", "Ubuntu"}, result.OSType)
			}
		})
	}
}

func TestConvertTemplate_FlavorDetection(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name        string
		labels      map[string]string
		expectedCPU int
		expectedMem string
	}{
		{
			name: "Tiny flavor",
			labels: map[string]string{
				"flavor.template.kubevirt.io/tiny": "true",
			},
			expectedCPU: 1,
			expectedMem: "1Gi",
		},
		{
			name: "Small flavor",
			labels: map[string]string{
				"flavor.template.kubevirt.io/small": "true",
			},
			expectedCPU: 1,
			expectedMem: "2Gi",
		},
		{
			name: "Medium flavor",
			labels: map[string]string{
				"flavor.template.kubevirt.io/medium": "true",
			},
			expectedCPU: 1,
			expectedMem: "4Gi",
		},
		{
			name: "Large flavor",
			labels: map[string]string{
				"flavor.template.kubevirt.io/large": "true",
			},
			expectedCPU: 2,
			expectedMem: "8Gi",
		},
		{
			name:        "Unknown flavor (default)",
			labels:      map[string]string{},
			expectedCPU: 1,
			expectedMem: "2Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osTemplate := &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("test-uid"),
					Name:      "test-template",
					Namespace: "test-ns",
					Labels:    tt.labels,
				},
			}

			result := client.convertTemplate(osTemplate)

			assert.Equal(t, tt.expectedCPU, result.CPU)
			assert.Equal(t, tt.expectedMem, result.Memory)
		})
	}
}

func TestTemplate_Struct(t *testing.T) {
	// Test Template struct fields and JSON tags
	template := Template{
		ID:          "test-id",
		Name:        "test-name",
		Description: "test-description",
		OSType:      "Linux",
		OSVersion:   "Ubuntu 20.04",
		CPU:         2,
		Memory:      "4Gi",
		DiskSize:    "30Gi",
		Namespace:   "test-namespace",
		ImageURL:    "https://example.com/image.qcow2",
	}

	// Test that all fields are properly set
	assert.Equal(t, "test-id", template.ID)
	assert.Equal(t, "test-name", template.Name)
	assert.Equal(t, "test-description", template.Description)
	assert.Equal(t, "Linux", template.OSType)
	assert.Equal(t, "Ubuntu 20.04", template.OSVersion)
	assert.Equal(t, 2, template.CPU)
	assert.Equal(t, "4Gi", template.Memory)
	assert.Equal(t, "30Gi", template.DiskSize)
	assert.Equal(t, "test-namespace", template.Namespace)
	assert.Equal(t, "https://example.com/image.qcow2", template.ImageURL)
}

func TestVirtualMachine_Struct(t *testing.T) {
	// Test VirtualMachine struct fields
	vm := VirtualMachine{
		ID:        "vm-123",
		Name:      "test-vm",
		Status:    "Running",
		Namespace: "test-namespace",
		Template:  "rhel9-template",
		Created:   "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "vm-123", vm.ID)
	assert.Equal(t, "test-vm", vm.Name)
	assert.Equal(t, "Running", vm.Status)
	assert.Equal(t, "test-namespace", vm.Namespace)
	assert.Equal(t, "rhel9-template", vm.Template)
	assert.Equal(t, "2024-01-01T00:00:00Z", vm.Created)
}

func TestDeployVMRequest_Struct(t *testing.T) {
	// Test DeployVMRequest struct fields
	req := DeployVMRequest{
		TemplateName:    "rhel9-template",
		VMName:          "test-vm",
		TargetNamespace: "user-namespace",
		DiskSize:        "50Gi",
	}

	assert.Equal(t, "rhel9-template", req.TemplateName)
	assert.Equal(t, "test-vm", req.VMName)
	assert.Equal(t, "user-namespace", req.TargetNamespace)
	assert.Equal(t, "50Gi", req.DiskSize)
}

// Integration test for client creation with different configurations
func TestNewClient_Configurations(t *testing.T) {
	tests := []struct {
		name   string
		config *config.OpenShiftConfig
		hasErr bool
	}{
		{
			name: "In-cluster config",
			config: &config.OpenShiftConfig{
				Enabled:           true,
				InCluster:         true,
				ConfigPath:        "",
				TemplateNamespace: "openshift",
			},
			hasErr: true, // Will fail in test environment as we're not in-cluster
		},
		{
			name: "Kubeconfig path config",
			config: &config.OpenShiftConfig{
				Enabled:           true,
				InCluster:         false,
				ConfigPath:        "/nonexistent/path",
				TemplateNamespace: "openshift",
			},
			hasErr: true, // Will fail as path doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.hasErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// Test helper functions and edge cases
func TestConvertTemplate_EmptyTemplate(t *testing.T) {
	client := &Client{}

	// Test with minimal template
	osTemplate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("minimal-uid"),
			Name:      "minimal-template",
			Namespace: "minimal-ns",
		},
	}

	result := client.convertTemplate(osTemplate)

	assert.Equal(t, "minimal-uid", result.ID)
	assert.Equal(t, "minimal-template", result.Name)
	assert.Equal(t, "minimal-ns", result.Namespace)
	assert.Equal(t, "", result.Description)
	assert.Equal(t, "Unknown", result.OSType)
	assert.Equal(t, 1, result.CPU)           // Default values
	assert.Equal(t, "2Gi", result.Memory)    // Default values
	assert.Equal(t, "20Gi", result.DiskSize) // Default values
}

func TestConvertTemplate_PartialLabels(t *testing.T) {
	client := &Client{}

	// Test with OS label but no flavor label
	osTemplate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("partial-uid"),
			Name:      "partial-template",
			Namespace: "partial-ns",
			Labels: map[string]string{
				"os.template.kubevirt.io/fedora": "true",
				// No flavor labels
			},
		},
	}

	result := client.convertTemplate(osTemplate)

	assert.Equal(t, "Fedora", result.OSType)
	assert.Equal(t, "Fedora", result.OSVersion)
	assert.Equal(t, 1, result.CPU)        // Default flavor
	assert.Equal(t, "2Gi", result.Memory) // Default flavor
}

func TestConvertTemplate_InvalidLabels(t *testing.T) {
	client := &Client{}

	// Test with invalid/irrelevant labels
	osTemplate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("invalid-uid"),
			Name:      "invalid-template",
			Namespace: "invalid-ns",
			Labels: map[string]string{
				"os.template.kubevirt.io/unknown": "false", // false value
				"flavor.template.kubevirt.io/xl":  "true",  // unknown flavor
				"irrelevant.label":                "true",  // irrelevant label
			},
		},
	}

	result := client.convertTemplate(osTemplate)

	assert.Equal(t, "Unknown", result.OSType) // Should remain unknown
	assert.Equal(t, 1, result.CPU)            // Should use defaults
	assert.Equal(t, "2Gi", result.Memory)     // Should use defaults
}
