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
	assert.Equal(t, "Rhel9 Server Small VM", result.Name) // Cleaned up template name
	assert.Equal(t, "openshift", result.Namespace)
	assert.Equal(t, "Red Hat Enterprise Linux 9 Server Template", result.Description)
	assert.Equal(t, "Rhel9", result.OSType) // From OS label
	assert.Equal(t, "", result.OSVersion)   // No specific version annotation
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

func TestConvertTemplate_ProperAnnotations(t *testing.T) {
	client := &Client{}

	// Test with proper OpenShift annotations (this is the ideal case)
	osTemplate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("proper-uid"),
			Name:      "rhel9-highperformance-medium",
			Namespace: "openshift",
			Annotations: map[string]string{
				"openshift.io/display-name":       "Red Hat Enterprise Linux 9 High Performance VM",
				"openshift.io/description":        "RHEL 9 optimized for high performance workloads",
				"os.template.kubevirt.io/name":    "Red Hat Enterprise Linux",
				"os.template.kubevirt.io/version": "9.2",
			},
			Labels: map[string]string{
				"flavor.template.kubevirt.io/medium": "true",
			},
		},
	}

	result := client.convertTemplate(osTemplate)

	assert.Equal(t, "proper-uid", result.ID)
	assert.Equal(t, "Red Hat Enterprise Linux 9 High Performance VM", result.Name) // Uses display-name annotation
	assert.Equal(t, "openshift", result.Namespace)
	assert.Equal(t, "RHEL 9 optimized for high performance workloads", result.Description)
	assert.Equal(t, "Red Hat Enterprise Linux", result.OSType) // From OS annotation
	assert.Equal(t, "9.2", result.OSVersion)                   // From OS version annotation
	assert.Equal(t, 1, result.CPU)
	assert.Equal(t, "4Gi", result.Memory)
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
			expected: "Linux", // Falls back to Linux when no OS info found
		},
		{
			name: "Multiple OS labels (first match wins)",
			labels: map[string]string{
				"os.template.kubevirt.io/rhel9":  "true",
				"os.template.kubevirt.io/ubuntu": "true",
			},
			expected: "Rhel9", // Simple label-based extraction
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

			assert.Equal(t, tt.expected, result.OSType)
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
	assert.Equal(t, "Minimal Template VM", result.Name) // Cleaned up template name (dashes become spaces)
	assert.Equal(t, "minimal-ns", result.Namespace)
	assert.Equal(t, "Virtual Machine template", result.Description) // Now provides default description
	assert.Equal(t, "Linux", result.OSType)                         // Now defaults to Linux
	assert.Equal(t, 1, result.CPU)                                  // Default values
	assert.Equal(t, "2Gi", result.Memory)                           // Default values
	assert.Equal(t, "20Gi", result.DiskSize)                        // Default values
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
	assert.Equal(t, "", result.OSVersion) // No specific version for fedora without version number
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

	assert.Equal(t, "Linux", result.OSType) // Now defaults to Linux
	assert.Equal(t, 1, result.CPU)          // Should use defaults
	assert.Equal(t, "2Gi", result.Memory)   // Should use defaults
}

// Comprehensive unit tests for template display name extraction
func TestExtractDisplayName(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name        string
		template    *templatev1.Template
		expected    string
		description string
	}{
		{
			name: "Primary: openshift.io/display-name annotation",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rhel9-server-small",
					Annotations: map[string]string{
						"openshift.io/display-name": "Red Hat Enterprise Linux 9 VM",
					},
				},
			},
			expected:    "Red Hat Enterprise Linux 9 VM",
			description: "Should use the primary display-name annotation",
		},
		{
			name: "Secondary: name.os.template.kubevirt.io annotation",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ubuntu-server-medium",
					Annotations: map[string]string{
						"name.os.template.kubevirt.io": "Ubuntu 22.04 LTS Server VM",
					},
				},
			},
			expected:    "Ubuntu 22.04 LTS Server VM",
			description: "Should use KubeVirt name annotation when display-name is missing",
		},
		{
			name: "Tertiary: short long-description annotation",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "centos-workstation",
					Annotations: map[string]string{
						"template.openshift.io/long-description": "CentOS Stream 9 Workstation",
					},
				},
			},
			expected:    "CentOS Stream 9 Workstation",
			description: "Should use short long-description when other annotations are missing",
		},
		{
			name: "Skip long long-description annotation",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fedora-desktop",
					Annotations: map[string]string{
						"template.openshift.io/long-description": "This is a very long description that exceeds the 80 character limit and should be ignored in favor of template name cleanup",
					},
				},
			},
			expected:    "Fedora Desktop VM",
			description: "Should skip long descriptions and fallback to name cleanup",
		},
		{
			name: "Complex template name: high performance variant",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rhel9-highperformance-medium",
				},
			},
			expected:    "Rhel9 Highperformance Medium VM",
			description: "Should handle complex template names with multiple components",
		},
		{
			name: "Template name with numbers and versions",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "windows2k22-datacenter-large",
				},
			},
			expected:    "Windows2k22 Datacenter Large VM",
			description: "Should preserve version numbers and capitalize appropriately",
		},
		{
			name: "Template name with special prefixes",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "custom-v2-app-server",
				},
			},
			expected:    "Custom V2 APP Server VM",
			description: "Should handle version prefixes correctly",
		},
		{
			name: "Simple template name",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "database",
				},
			},
			expected:    "Database VM",
			description: "Should handle simple names and add VM suffix",
		},
		{
			name: "Template name already containing VM",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "custom-vm-template",
				},
			},
			expected:    "Custom VM Template",
			description: "Should not add VM suffix if already present",
		},
		{
			name: "Empty template name",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
			expected:    "VM",
			description: "Should handle empty names gracefully",
		},
		{
			name: "Priority test: display-name overrides others",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-template",
					Annotations: map[string]string{
						"openshift.io/display-name":              "Primary Display Name",
						"name.os.template.kubevirt.io":           "Secondary Name",
						"template.openshift.io/long-description": "Tertiary Description",
					},
				},
			},
			expected:    "Primary Display Name",
			description: "Should always prefer display-name annotation over others",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.extractDisplayName(tt.template)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// Unit tests for OS information extraction
func TestExtractOSInfo(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name            string
		template        *templatev1.Template
		expectedOS      string
		expectedVersion string
		description     string
	}{
		{
			name: "Primary: OS annotations with name and version",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"os.template.kubevirt.io/name":    "Red Hat Enterprise Linux",
						"os.template.kubevirt.io/version": "9.2",
					},
				},
			},
			expectedOS:      "Red Hat Enterprise Linux",
			expectedVersion: "9.2",
			description:     "Should extract OS name and version from dedicated annotations",
		},
		{
			name: "Primary: OS annotation with name only",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"os.template.kubevirt.io/name": "Ubuntu",
					},
				},
			},
			expectedOS:      "Ubuntu",
			expectedVersion: "",
			description:     "Should extract OS name even without version annotation",
		},
		{
			name: "Secondary: template operating-system annotation",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"template.kubevirt.io/operating-system": "Microsoft Windows Server",
					},
				},
			},
			expectedOS:      "Microsoft Windows Server",
			expectedVersion: "",
			description:     "Should use operating-system annotation as fallback",
		},
		{
			name: "Tertiary: OS labels",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"os.template.kubevirt.io/fedora39": "true",
					},
				},
			},
			expectedOS:      "Fedora39",
			expectedVersion: "",
			description:     "Should extract OS from labels when annotations are missing",
		},
		{
			name: "Label with underscores",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"os.template.kubevirt.io/centos_stream_9": "true",
					},
				},
			},
			expectedOS:      "Centos Stream 9",
			expectedVersion: "",
			description:     "Should replace underscores with spaces in OS labels",
		},
		{
			name: "Template name fallback: RHEL",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rhel9-server-small",
				},
			},
			expectedOS:      "Red Hat Enterprise Linux",
			expectedVersion: "",
			description:     "Should detect RHEL from template name as last resort",
		},
		{
			name: "Template name fallback: CentOS",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "centos-stream-medium",
				},
			},
			expectedOS:      "CentOS Stream",
			expectedVersion: "",
			description:     "Should detect CentOS from template name",
		},
		{
			name: "Template name fallback: Windows",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "windows2k22-datacenter",
				},
			},
			expectedOS:      "Microsoft Windows",
			expectedVersion: "",
			description:     "Should detect Windows from template name",
		},
		{
			name: "Default fallback",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unknown-custom-template",
				},
			},
			expectedOS:      "Linux",
			expectedVersion: "",
			description:     "Should default to Linux when no OS information is found",
		},
		{
			name: "Priority test: annotations override labels",
			template: &templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-template",
					Annotations: map[string]string{
						"os.template.kubevirt.io/name": "Annotation OS",
					},
					Labels: map[string]string{
						"os.template.kubevirt.io/different": "true",
					},
				},
			},
			expectedOS:      "Annotation OS",
			expectedVersion: "",
			description:     "Should prefer annotations over labels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osType, osVersion := client.extractOSInfo(tt.template)
			assert.Equal(t, tt.expectedOS, osType, tt.description+" (OS type)")
			assert.Equal(t, tt.expectedVersion, osVersion, tt.description+" (OS version)")
		})
	}
}

// Unit tests for template name cleanup
func TestCleanupTemplateName(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name        string
		input       string
		expected    string
		description string
	}{
		{
			name:        "Simple dash replacement",
			input:       "rhel9-server-small",
			expected:    "Rhel9 Server Small VM",
			description: "Should replace dashes with spaces and title case",
		},
		{
			name:        "Complex high performance template",
			input:       "rhel9-highperformance-gpu-large",
			expected:    "Rhel9 Highperformance GPU Large VM",
			description: "Should handle complex multi-component names",
		},
		{
			name:        "Version preservation",
			input:       "windows2k22-datacenter-v2",
			expected:    "Windows2k22 Datacenter V2 VM",
			description: "Should preserve version numbers and version prefixes",
		},
		{
			name:        "Short words capitalization",
			input:       "app-db-api-v3",
			expected:    "APP DB API V3 VM",
			description: "Should capitalize common acronyms",
		},
		{
			name:        "Already contains VM",
			input:       "custom-vm-template",
			expected:    "Custom VM Template",
			description: "Should not add VM suffix if already present",
		},
		{
			name:        "Single word",
			input:       "database",
			expected:    "Database VM",
			description: "Should handle single words and add VM suffix",
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "VM",
			description: "Should handle empty input gracefully",
		},
		{
			name:        "Multiple consecutive dashes",
			input:       "test--template---name",
			expected:    "Test Template Name VM",
			description: "Should handle multiple consecutive dashes",
		},
		{
			name:        "Mixed case preservation",
			input:       "mySQL-db-server",
			expected:    "MySQL DB Server VM",
			description: "Should title case each word appropriately",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.cleanupTemplateName(tt.input)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}
