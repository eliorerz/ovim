package models

import (
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringMap_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected StringMap
		hasError bool
	}{
		{
			name:     "Valid JSON bytes",
			input:    []byte(`{"key1":"value1","key2":"value2"}`),
			expected: StringMap{"key1": "value1", "key2": "value2"},
			hasError: false,
		},
		{
			name:     "Empty JSON bytes",
			input:    []byte(`{}`),
			expected: StringMap{},
			hasError: false,
		},
		{
			name:     "Nil input",
			input:    nil,
			expected: StringMap{},
			hasError: false,
		},
		{
			name:     "Empty byte slice",
			input:    []byte{},
			expected: StringMap{},
			hasError: false,
		},
		{
			name:     "Invalid JSON",
			input:    []byte(`{"invalid":json}`),
			expected: nil,
			hasError: true,
		},
		{
			name:     "Invalid type (string)",
			input:    "not a byte slice",
			expected: nil,
			hasError: true,
		},
		{
			name:     "Invalid type (int)",
			input:    123,
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sm StringMap
			err := sm.Scan(tt.input)

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, sm)
			}
		})
	}
}

func TestStringMap_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    StringMap
		expected string
		hasError bool
	}{
		{
			name:     "Valid StringMap",
			input:    StringMap{"key1": "value1", "key2": "value2"},
			expected: `{"key1":"value1","key2":"value2"}`,
			hasError: false,
		},
		{
			name:     "Empty StringMap",
			input:    StringMap{},
			expected: `{}`,
			hasError: false,
		},
		{
			name:     "Nil StringMap",
			input:    nil,
			expected: "",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.input.Value()

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.input == nil {
					assert.Nil(t, value)
				} else {
					// Parse both as JSON to compare content (order-independent)
					var expectedMap, actualMap map[string]string

					err1 := json.Unmarshal([]byte(tt.expected), &expectedMap)
					assert.NoError(t, err1)

					actualBytes, ok := value.([]byte)
					assert.True(t, ok)

					err2 := json.Unmarshal(actualBytes, &actualMap)
					assert.NoError(t, err2)

					assert.Equal(t, expectedMap, actualMap)
				}
			}
		})
	}
}

func TestStringMap_DriverValuer_Interface(t *testing.T) {
	// Test that StringMap implements driver.Valuer interface
	var sm StringMap = StringMap{"test": "value"}
	var valuer driver.Valuer = sm

	value, err := valuer.Value()
	assert.NoError(t, err)
	assert.NotNil(t, value)
}

func TestUser_Struct(t *testing.T) {
	// Test User struct fields and JSON tags
	orgID := "org-123"
	user := User{
		ID:           "user-123",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		Role:         RoleOrgUser,
		OrgID:        &orgID,
	}

	// Test that all fields are properly set
	assert.Equal(t, "user-123", user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "hashedpassword", user.PasswordHash)
	assert.Equal(t, RoleOrgUser, user.Role)
	assert.NotNil(t, user.OrgID)
	assert.Equal(t, "org-123", *user.OrgID)
}

func TestUser_NilOrgID(t *testing.T) {
	user := User{
		ID:       "user-123",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     RoleSystemAdmin,
		OrgID:    nil,
	}

	assert.Nil(t, user.OrgID)
}

func TestOrganization_Struct(t *testing.T) {
	org := Organization{
		ID:          "org-123",
		Name:        "ACME Corporation",
		Description: "A test organization",
		Namespace:   "org-acme",
		IsEnabled:   true,
	}

	assert.Equal(t, "org-123", org.ID)
	assert.Equal(t, "ACME Corporation", org.Name)
	assert.Equal(t, "A test organization", org.Description)
	assert.Equal(t, "org-acme", org.Namespace)
	assert.True(t, org.IsEnabled)
}

func TestVirtualDataCenter_Struct(t *testing.T) {
	quotas := StringMap{
		"cpu":     "10",
		"memory":  "32Gi",
		"storage": "100Gi",
	}

	vdc := VirtualDataCenter{
		ID:             "vdc-123",
		Name:           "Production VDC",
		Description:    "Production virtual data center",
		OrgID:          "org-123",
		Namespace:      "org-acme-prod",
		ResourceQuotas: quotas,
	}

	assert.Equal(t, "vdc-123", vdc.ID)
	assert.Equal(t, "Production VDC", vdc.Name)
	assert.Equal(t, "Production virtual data center", vdc.Description)
	assert.Equal(t, "org-123", vdc.OrgID)
	assert.Equal(t, "org-acme-prod", vdc.Namespace)
	assert.Equal(t, quotas, vdc.ResourceQuotas)
}

func TestTemplate_Struct(t *testing.T) {
	metadata := StringMap{
		"openshift_template": "true",
		"source_namespace":   "openshift",
	}

	template := Template{
		ID:           "template-123",
		Name:         "rhel9-server-small",
		Description:  "Red Hat Enterprise Linux 9 Server",
		OSType:       "Linux",
		OSVersion:    "RHEL 9",
		CPU:          1,
		Memory:       "2Gi",
		DiskSize:     "20Gi",
		ImageURL:     "https://example.com/image.qcow2",
		OrgID:        "org-123",
		Source:       TemplateSourceGlobal,
		SourceVendor: "Red Hat",
		Category:     TemplateCategoryOS,
		Namespace:    "openshift",
		Featured:     true,
		Metadata:     metadata,
	}

	assert.Equal(t, "template-123", template.ID)
	assert.Equal(t, "rhel9-server-small", template.Name)
	assert.Equal(t, "Red Hat Enterprise Linux 9 Server", template.Description)
	assert.Equal(t, "Linux", template.OSType)
	assert.Equal(t, "RHEL 9", template.OSVersion)
	assert.Equal(t, 1, template.CPU)
	assert.Equal(t, "2Gi", template.Memory)
	assert.Equal(t, "20Gi", template.DiskSize)
	assert.Equal(t, "https://example.com/image.qcow2", template.ImageURL)
	assert.Equal(t, "org-123", template.OrgID)
	assert.Equal(t, TemplateSourceGlobal, template.Source)
	assert.Equal(t, "Red Hat", template.SourceVendor)
	assert.Equal(t, TemplateCategoryOS, template.Category)
	assert.Equal(t, "openshift", template.Namespace)
	assert.True(t, template.Featured)
	assert.Equal(t, metadata, template.Metadata)
}

func TestVirtualMachine_Struct(t *testing.T) {
	metadata := StringMap{
		"node":       "worker-1",
		"zone":       "us-east-1a",
		"created_by": "user-123",
	}

	vm := VirtualMachine{
		ID:         "vm-123",
		Name:       "test-vm",
		OrgID:      "org-123",
		VDCID:      "vdc-123",
		TemplateID: "template-123",
		OwnerID:    "user-123",
		Status:     VMStatusRunning,
		CPU:        2,
		Memory:     "4Gi",
		DiskSize:   "30Gi",
		IPAddress:  "192.168.1.100",
		Metadata:   metadata,
	}

	assert.Equal(t, "vm-123", vm.ID)
	assert.Equal(t, "test-vm", vm.Name)
	assert.Equal(t, "org-123", vm.OrgID)
	assert.Equal(t, "vdc-123", vm.VDCID)
	assert.Equal(t, "template-123", vm.TemplateID)
	assert.Equal(t, "user-123", vm.OwnerID)
	assert.Equal(t, VMStatusRunning, vm.Status)
	assert.Equal(t, 2, vm.CPU)
	assert.Equal(t, "4Gi", vm.Memory)
	assert.Equal(t, "30Gi", vm.DiskSize)
	assert.Equal(t, "192.168.1.100", vm.IPAddress)
	assert.Equal(t, metadata, vm.Metadata)
}

func TestConstants_UserRoles(t *testing.T) {
	assert.Equal(t, "system_admin", RoleSystemAdmin)
	assert.Equal(t, "org_admin", RoleOrgAdmin)
	assert.Equal(t, "org_user", RoleOrgUser)
}

func TestConstants_VMStatuses(t *testing.T) {
	assert.Equal(t, "pending", VMStatusPending)
	assert.Equal(t, "provisioning", VMStatusProvisioning)
	assert.Equal(t, "running", VMStatusRunning)
	assert.Equal(t, "stopped", VMStatusStopped)
	assert.Equal(t, "error", VMStatusError)
	assert.Equal(t, "deleting", VMStatusDeleting)
}

func TestConstants_TemplateSources(t *testing.T) {
	assert.Equal(t, "global", TemplateSourceGlobal)
	assert.Equal(t, "organization", TemplateSourceOrganization)
	assert.Equal(t, "external", TemplateSourceExternal)
}

func TestConstants_TemplateCategories(t *testing.T) {
	assert.Equal(t, "Operating System", TemplateCategoryOS)
	assert.Equal(t, "Database", TemplateCategoryDatabase)
	assert.Equal(t, "Application", TemplateCategoryApplication)
	assert.Equal(t, "Middleware", TemplateCategoryMiddleware)
	assert.Equal(t, "Other", TemplateCategoryOther)
}

func TestCreateOrganizationRequest_Struct(t *testing.T) {
	req := CreateOrganizationRequest{
		Name:        "Test Organization",
		Description: "A test organization",
		IsEnabled:   true,
	}

	assert.Equal(t, "Test Organization", req.Name)
	assert.Equal(t, "A test organization", req.Description)
	assert.True(t, req.IsEnabled)
}

func TestUpdateOrganizationRequest_Struct(t *testing.T) {
	req := UpdateOrganizationRequest{
		Name:        "Updated Organization",
		Description: "Updated description",
		IsEnabled:   false,
	}

	assert.Equal(t, "Updated Organization", req.Name)
	assert.Equal(t, "Updated description", req.Description)
	assert.False(t, req.IsEnabled)
}

func TestCreateVDCRequest_Struct(t *testing.T) {
	quotas := map[string]string{
		"cpu":    "10",
		"memory": "32Gi",
	}

	req := CreateVDCRequest{
		Name:           "Test VDC",
		Description:    "A test VDC",
		OrgID:          "org-123",
		ResourceQuotas: quotas,
	}

	assert.Equal(t, "Test VDC", req.Name)
	assert.Equal(t, "A test VDC", req.Description)
	assert.Equal(t, "org-123", req.OrgID)
	assert.Equal(t, quotas, req.ResourceQuotas)
}

func TestUpdateVDCRequest_Struct(t *testing.T) {
	quotas := map[string]string{
		"storage": "100Gi",
	}

	req := UpdateVDCRequest{
		Name:           "Updated VDC",
		Description:    "Updated description",
		ResourceQuotas: quotas,
	}

	assert.Equal(t, "Updated VDC", req.Name)
	assert.Equal(t, "Updated description", req.Description)
	assert.Equal(t, quotas, req.ResourceQuotas)
}

func TestCreateVMRequest_Struct(t *testing.T) {
	req := CreateVMRequest{
		Name:       "test-vm",
		TemplateID: "template-123",
		CPU:        2,
		Memory:     "4Gi",
		DiskSize:   "30Gi",
	}

	assert.Equal(t, "test-vm", req.Name)
	assert.Equal(t, "template-123", req.TemplateID)
	assert.Equal(t, 2, req.CPU)
	assert.Equal(t, "4Gi", req.Memory)
	assert.Equal(t, "30Gi", req.DiskSize)
}

func TestUpdateVMPowerRequest_Struct(t *testing.T) {
	req := UpdateVMPowerRequest{
		Action: "start",
	}

	assert.Equal(t, "start", req.Action)
}

// Test JSON serialization/deserialization
func TestStringMap_JSONSerialization(t *testing.T) {
	original := StringMap{
		"key1": "value1",
		"key2": "value2",
		"key3": "value with spaces",
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(original)
	assert.NoError(t, err)

	// Deserialize from JSON
	var deserialized StringMap
	err = json.Unmarshal(jsonData, &deserialized)
	assert.NoError(t, err)

	// Should be equal
	assert.Equal(t, original, deserialized)
}

func TestTemplate_JSONSerialization(t *testing.T) {
	template := Template{
		ID:           "template-123",
		Name:         "test-template",
		Source:       TemplateSourceGlobal,
		SourceVendor: "Red Hat",
		Category:     TemplateCategoryOS,
		Featured:     true,
		Metadata: StringMap{
			"test_key": "test_value",
		},
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(template)
	assert.NoError(t, err)

	// Deserialize from JSON
	var deserialized Template
	err = json.Unmarshal(jsonData, &deserialized)
	assert.NoError(t, err)

	// Should be equal
	assert.Equal(t, template.ID, deserialized.ID)
	assert.Equal(t, template.Name, deserialized.Name)
	assert.Equal(t, template.Source, deserialized.Source)
	assert.Equal(t, template.SourceVendor, deserialized.SourceVendor)
	assert.Equal(t, template.Category, deserialized.Category)
	assert.Equal(t, template.Featured, deserialized.Featured)
	assert.Equal(t, template.Metadata, deserialized.Metadata)
}

// Edge case testing
func TestStringMap_ComplexValues(t *testing.T) {
	sm := StringMap{
		"json_string":   `{"nested":"value"}`,
		"special_chars": "áéíóú!@#$%^&*()",
		"empty_string":  "",
		"unicode":       "🚀🌟✨",
		"numbers":       "123456",
		"multiline":     "line1\nline2\nline3",
	}

	// Test Value() method
	value, err := sm.Value()
	assert.NoError(t, err)

	// Test Scan() method
	var scanned StringMap
	err = scanned.Scan(value)
	assert.NoError(t, err)

	assert.Equal(t, sm, scanned)
}
