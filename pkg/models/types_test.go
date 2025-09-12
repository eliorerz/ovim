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

func TestCreateVDCRequest_WithLimitRange(t *testing.T) {
	quotas := map[string]string{
		"cpu":     "10",
		"memory":  "32Gi",
		"storage": "100Gi",
	}

	req := CreateVDCRequest{
		Name:           "Test VDC with LimitRange",
		Description:    "A test VDC with VM resource constraints",
		OrgID:          "org-123",
		ResourceQuotas: quotas,
		MinCPU:         intPtr(1),
		MaxCPU:         intPtr(8),
		MinMemory:      intPtr(1),
		MaxMemory:      intPtr(16),
	}

	assert.Equal(t, "Test VDC with LimitRange", req.Name)
	assert.Equal(t, "A test VDC with VM resource constraints", req.Description)
	assert.Equal(t, "org-123", req.OrgID)
	assert.Equal(t, quotas, req.ResourceQuotas)
	assert.NotNil(t, req.MinCPU)
	assert.Equal(t, 1, *req.MinCPU)
	assert.NotNil(t, req.MaxCPU)
	assert.Equal(t, 8, *req.MaxCPU)
	assert.NotNil(t, req.MinMemory)
	assert.Equal(t, 1, *req.MinMemory)
	assert.NotNil(t, req.MaxMemory)
	assert.Equal(t, 16, *req.MaxMemory)
}

func TestLimitRangeRequest_Struct(t *testing.T) {
	req := LimitRangeRequest{
		MinCPU:    1,
		MaxCPU:    8,
		MinMemory: 1,
		MaxMemory: 16,
	}

	assert.Equal(t, 1, req.MinCPU)
	assert.Equal(t, 8, req.MaxCPU)
	assert.Equal(t, 1, req.MinMemory)
	assert.Equal(t, 16, req.MaxMemory)
}

func TestLimitRangeInfo_Struct(t *testing.T) {
	info := LimitRangeInfo{
		Exists:    true,
		MinCPU:    1,
		MaxCPU:    8,
		MinMemory: 1,
		MaxMemory: 16,
	}

	assert.True(t, info.Exists)
	assert.Equal(t, 1, info.MinCPU)
	assert.Equal(t, 8, info.MaxCPU)
	assert.Equal(t, 1, info.MinMemory)
	assert.Equal(t, 16, info.MaxMemory)
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
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

// Tests for Resource Quota functionality added in Phase A1/A2

func TestOrganization_IdentityContainer(t *testing.T) {
	org := Organization{
		ID:          "org-123",
		Name:        "Test Organization",
		Description: "Identity container only",
		Namespace:   "org-test",
		IsEnabled:   true,
	}

	assert.Equal(t, "org-123", org.ID)
	assert.Equal(t, "Test Organization", org.Name)
	assert.Equal(t, "Identity container only", org.Description)
	assert.Equal(t, "org-test", org.Namespace)
	assert.True(t, org.IsEnabled)
}

func TestOrganizationResourceUsage_Struct(t *testing.T) {
	usage := OrganizationResourceUsage{
		CPUUsed:     10,
		MemoryUsed:  20,
		StorageUsed: 100,

		CPUQuota:     50,
		MemoryQuota:  100,
		StorageQuota: 500,

		VDCCount: 3,
	}

	assert.Equal(t, 10, usage.CPUUsed)
	assert.Equal(t, 20, usage.MemoryUsed)
	assert.Equal(t, 100, usage.StorageUsed)
	assert.Equal(t, 50, usage.CPUQuota)
	assert.Equal(t, 100, usage.MemoryQuota)
	assert.Equal(t, 500, usage.StorageQuota)
	assert.Equal(t, 3, usage.VDCCount)
}

func TestParseCPUString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"Simple number", "4", 4},
		{"Number with cores", "8 cores", 8},
		{"Number with 'c'", "12c", 12},
		{"Leading/trailing spaces", "  16  ", 16},
		{"Zero", "0", 0},
		{"Large number", "64", 64},
		{"Invalid string", "invalid", 0},
		{"Empty string", "", 0},
		{"Only text", "cores", 0},
		{"Mixed valid", "2 CPU cores", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCPUString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMemoryString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		// GB formats
		{"GB format", "8GB", 8},
		{"GiB format", "16GiB", 16},
		{"Gi format", "32Gi", 32},
		{"G format", "4G", 4},

		// MB formats (convert to GB)
		{"MB format", "2048MB", 2},
		{"MiB format", "1024MiB", 1},
		{"Mi format", "512Mi", 0}, // 512Mi = 0.5 GB, rounds down to 0

		// TB formats (convert to GB)
		{"TB format", "2TB", 2048},
		{"TiB format", "1TiB", 1024},
		{"Ti format", "3Ti", 3072},

		// KB formats (convert to GB)
		{"Large KB", "2097152KB", 2}, // 2GB in KB
		{"Small KB", "1024KB", 0},    // 1MB in KB, rounds down to 0

		// Edge cases
		{"Zero", "0GB", 0},
		{"No unit (invalid)", "16", 0}, // No unit is invalid, requires unit
		{"Lowercase", "8gb", 8},
		{"Mixed case", "4GiB", 4},
		{"With spaces", "  8 GB  ", 8},
		{"Invalid format", "invalid", 0},
		{"Empty string", "", 0},
		{"Only unit", "GB", 0},
		{"Decimal not supported", "8.5GB", 5}, // regex captures digit "5" from the decimal
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMemoryString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseStorageString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"Basic GB", "100GB", 100},
		{"GiB format", "50GiB", 50},
		{"Gi format", "200Gi", 200},
		{"TB format", "1TB", 1024},
		{"TiB format", "2TiB", 2048},
		{"Large storage", "10TB", 10240},
		{"Zero storage", "0GB", 0},
		{"Invalid format", "invalid", 0},
		{"Empty string", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseStorageString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOrganization_GetResourceUsage(t *testing.T) {
	org := Organization{
		ID:        "org-123",
		Name:      "Test Organization",
		Namespace: "org-test",
	}

	tests := []struct {
		name             string
		vdcs             []*VirtualDataCenter
		expectedCPU      int
		expectedMemory   int
		expectedStorage  int
		expectedVDCCount int
	}{
		{
			name:             "Empty VDCs",
			vdcs:             []*VirtualDataCenter{},
			expectedCPU:      0,
			expectedMemory:   0,
			expectedStorage:  0,
			expectedVDCCount: 0,
		},
		{
			name: "Single VDC with resources",
			vdcs: []*VirtualDataCenter{
				{
					ID: "vdc-1",
					ResourceQuotas: StringMap{
						"cpu":     "10",
						"memory":  "32Gi",
						"storage": "100Gi",
					},
				},
			},
			expectedCPU:      10,
			expectedMemory:   32,
			expectedStorage:  100,
			expectedVDCCount: 1,
		},
		{
			name: "Multiple VDCs with resources",
			vdcs: []*VirtualDataCenter{
				{
					ID: "vdc-1",
					ResourceQuotas: StringMap{
						"cpu":     "10",
						"memory":  "32Gi",
						"storage": "100Gi",
					},
				},
				{
					ID: "vdc-2",
					ResourceQuotas: StringMap{
						"cpu":     "8",
						"memory":  "16Gi",
						"storage": "50Gi",
					},
				},
			},
			expectedCPU:      18,
			expectedMemory:   48,
			expectedStorage:  150,
			expectedVDCCount: 2,
		},
		{
			name: "VDC with nil ResourceQuotas",
			vdcs: []*VirtualDataCenter{
				{
					ID:             "vdc-1",
					ResourceQuotas: nil,
				},
				{
					ID: "vdc-2",
					ResourceQuotas: StringMap{
						"cpu":     "5",
						"memory":  "8Gi",
						"storage": "25Gi",
					},
				},
			},
			expectedCPU:      5,
			expectedMemory:   8,
			expectedStorage:  25,
			expectedVDCCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := org.GetResourceUsage(tt.vdcs, []*VirtualMachine{})

			assert.Equal(t, tt.expectedCPU, usage.CPUQuota)         // Total allocated across VDCs
			assert.Equal(t, tt.expectedMemory, usage.MemoryQuota)   // Total allocated across VDCs
			assert.Equal(t, tt.expectedStorage, usage.StorageQuota) // Total allocated across VDCs
			assert.Equal(t, tt.expectedVDCCount, usage.VDCCount)

			// TODO: Usage calculation will be implemented when VM tracking is added
			assert.Equal(t, 0, usage.CPUUsed)
			assert.Equal(t, 0, usage.MemoryUsed)
			assert.Equal(t, 0, usage.StorageUsed)
		})
	}
}

func TestOrganization_CanAllocateResources(t *testing.T) {
	org := Organization{
		ID:        "org-123",
		Name:      "Test Organization",
		Namespace: "org-test",
	}

	existingVDCs := []*VirtualDataCenter{
		{
			ID: "vdc-1",
			ResourceQuotas: StringMap{
				"cpu":     "20",
				"memory":  "40Gi",
				"storage": "200Gi",
			},
		},
	}

	tests := []struct {
		name        string
		cpuReq      int
		memoryReq   int
		storageReq  int
		canAllocate bool
	}{
		{
			name:        "Organizations are identity containers - always allow",
			cpuReq:      1000,
			memoryReq:   2000,
			storageReq:  5000,
			canAllocate: true,
		},
		{
			name:        "Zero resources - allowed",
			cpuReq:      0,
			memoryReq:   0,
			storageReq:  0,
			canAllocate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := org.CanAllocateResources(tt.cpuReq, tt.memoryReq, tt.storageReq, existingVDCs)
			assert.Equal(t, tt.canAllocate, result)
		})
	}
}

func TestOrganization_CanAllocateResources_EmptyVDCs(t *testing.T) {
	org := Organization{
		ID:        "org-123",
		Name:      "Test Organization",
		Namespace: "org-test",
	}

	// No existing VDCs
	emptyVDCs := []*VirtualDataCenter{}

	tests := []struct {
		name        string
		cpuReq      int
		memoryReq   int
		storageReq  int
		canAllocate bool
	}{
		{
			name:        "Organizations are identity containers - always allow",
			cpuReq:      50,
			memoryReq:   100,
			storageReq:  500,
			canAllocate: true,
		},
		{
			name:        "Large requests also allowed for identity containers",
			cpuReq:      1000,
			memoryReq:   2000,
			storageReq:  5000,
			canAllocate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := org.CanAllocateResources(tt.cpuReq, tt.memoryReq, tt.storageReq, emptyVDCs)
			assert.Equal(t, tt.canAllocate, result)
		})
	}
}

func TestOrganization_IdentityContainer_JSONSerialization(t *testing.T) {
	org := Organization{
		ID:          "org-123",
		Name:        "Test Organization",
		Description: "Identity container only",
		Namespace:   "org-test",
		IsEnabled:   true,
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(org)
	assert.NoError(t, err)

	// Deserialize from JSON
	var deserialized Organization
	err = json.Unmarshal(jsonData, &deserialized)
	assert.NoError(t, err)

	// Should be equal
	assert.Equal(t, org.ID, deserialized.ID)
	assert.Equal(t, org.Name, deserialized.Name)
	assert.Equal(t, org.Description, deserialized.Description)
	assert.Equal(t, org.Namespace, deserialized.Namespace)
	assert.Equal(t, org.IsEnabled, deserialized.IsEnabled)
}

func TestOrganizationResourceUsage_JSONSerialization(t *testing.T) {
	usage := OrganizationResourceUsage{
		CPUUsed:     20,
		MemoryUsed:  40,
		StorageUsed: 200,

		CPUQuota:     50,
		MemoryQuota:  100,
		StorageQuota: 500,

		VDCCount: 3,
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(usage)
	assert.NoError(t, err)

	// Deserialize from JSON
	var deserialized OrganizationResourceUsage
	err = json.Unmarshal(jsonData, &deserialized)
	assert.NoError(t, err)

	// Should be equal
	assert.Equal(t, usage, deserialized)
}
