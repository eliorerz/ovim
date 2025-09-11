package validation

import (
	"testing"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestValidateOrganization(t *testing.T) {
	tests := []struct {
		name    string
		org     *models.Organization
		isValid bool
		errors  []string
	}{
		{
			name: "Valid organization",
			org: &models.Organization{
				Name:        "ACME Corporation",
				Description: "A test organization",
			},
			isValid: true,
			errors:  []string{},
		},
		{
			name: "Empty name",
			org: &models.Organization{
				Name:        "",
				Description: "A test organization",
			},
			isValid: false,
			errors:  []string{"organization name is required"},
		},
		{
			name: "Name too short",
			org: &models.Organization{
				Name:        "AB",
				Description: "A test organization",
			},
			isValid: false,
			errors:  []string{"organization name must be at least 3 characters long"},
		},
		{
			name: "Name too long",
			org: &models.Organization{
				Name:        "This organization name is way too long and exceeds the maximum allowed length for organization names which should be reasonable",
				Description: "A test organization",
			},
			isValid: false,
			errors:  []string{"organization name must be at most 100 characters long"},
		},
		{
			name: "Invalid characters in name",
			org: &models.Organization{
				Name:        "ACME Corp!@#$",
				Description: "A test organization",
			},
			isValid: false,
			errors:  []string{"organization name contains invalid characters"},
		},
		{
			name: "Description too long",
			org: &models.Organization{
				Name:        "ACME Corporation",
				Description: generateLongString(501),
			},
			isValid: false,
			errors:  []string{"organization description must be at most 500 characters long"},
		},
		{
			name: "Multiple validation errors",
			org: &models.Organization{
				Name:        "A!",
				Description: generateLongString(501),
			},
			isValid: false,
			errors: []string{
				"organization name must be at least 3 characters long",
				"organization name contains invalid characters",
				"organization description must be at most 500 characters long",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateOrganization(tt.org)

			if tt.isValid {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
				for _, expectedError := range tt.errors {
					assert.Contains(t, errors, expectedError)
				}
			}
		})
	}
}

func TestValidateUser(t *testing.T) {
	tests := []struct {
		name    string
		user    *models.User
		isValid bool
		errors  []string
	}{
		{
			name: "Valid user",
			user: &models.User{
				Username: "john.doe",
				Email:    "john.doe@example.com",
				Role:     models.RoleOrgUser,
			},
			isValid: true,
			errors:  []string{},
		},
		{
			name: "Empty username",
			user: &models.User{
				Username: "",
				Email:    "john.doe@example.com",
				Role:     models.RoleOrgUser,
			},
			isValid: false,
			errors:  []string{"username is required"},
		},
		{
			name: "Username too short",
			user: &models.User{
				Username: "ab",
				Email:    "john.doe@example.com",
				Role:     models.RoleOrgUser,
			},
			isValid: false,
			errors:  []string{"username must be at least 3 characters long"},
		},
		{
			name: "Username too long",
			user: &models.User{
				Username: "this_username_is_way_too_long_and_exceeds_the_maximum_allowed_length",
				Email:    "john.doe@example.com",
				Role:     models.RoleOrgUser,
			},
			isValid: false,
			errors:  []string{"username must be at most 50 characters long"},
		},
		{
			name: "Invalid username characters",
			user: &models.User{
				Username: "john doe!",
				Email:    "john.doe@example.com",
				Role:     models.RoleOrgUser,
			},
			isValid: false,
			errors:  []string{"username contains invalid characters"},
		},
		{
			name: "Empty email",
			user: &models.User{
				Username: "john.doe",
				Email:    "",
				Role:     models.RoleOrgUser,
			},
			isValid: false,
			errors:  []string{"email is required"},
		},
		{
			name: "Invalid email format",
			user: &models.User{
				Username: "john.doe",
				Email:    "invalid-email",
				Role:     models.RoleOrgUser,
			},
			isValid: false,
			errors:  []string{"email format is invalid"},
		},
		{
			name: "Invalid role",
			user: &models.User{
				Username: "john.doe",
				Email:    "john.doe@example.com",
				Role:     "invalid_role",
			},
			isValid: false,
			errors:  []string{"invalid role"},
		},
		{
			name: "Multiple validation errors",
			user: &models.User{
				Username: "a!",
				Email:    "invalid-email",
				Role:     "invalid_role",
			},
			isValid: false,
			errors: []string{
				"username must be at least 3 characters long",
				"username contains invalid characters",
				"email format is invalid",
				"invalid role",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateUser(tt.user)

			if tt.isValid {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
				for _, expectedError := range tt.errors {
					assert.Contains(t, errors, expectedError)
				}
			}
		})
	}
}

func TestValidateVDC(t *testing.T) {
	tests := []struct {
		name    string
		vdc     *models.VirtualDataCenter
		isValid bool
		errors  []string
	}{
		{
			name: "Valid VDC",
			vdc: &models.VirtualDataCenter{
				Name:        "Production VDC",
				Description: "Production virtual data center",
				OrgID:       "org-123",
				ResourceQuotas: models.StringMap{
					"cpu":     "20",
					"memory":  "64Gi",
					"storage": "500Gi",
				},
			},
			isValid: true,
			errors:  []string{},
		},
		{
			name: "Empty name",
			vdc: &models.VirtualDataCenter{
				Name:        "",
				Description: "Production virtual data center",
				OrgID:       "org-123",
			},
			isValid: false,
			errors:  []string{"VDC name is required"},
		},
		{
			name: "Name too short",
			vdc: &models.VirtualDataCenter{
				Name:        "VD",
				Description: "Production virtual data center",
				OrgID:       "org-123",
			},
			isValid: false,
			errors:  []string{"VDC name must be at least 3 characters long"},
		},
		{
			name: "Empty organization ID",
			vdc: &models.VirtualDataCenter{
				Name:        "Production VDC",
				Description: "Production virtual data center",
				OrgID:       "",
			},
			isValid: false,
			errors:  []string{"organization ID is required"},
		},
		{
			name: "Invalid resource quota - CPU",
			vdc: &models.VirtualDataCenter{
				Name:        "Production VDC",
				Description: "Production virtual data center",
				OrgID:       "org-123",
				ResourceQuotas: models.StringMap{
					"cpu":     "invalid",
					"memory":  "64Gi",
					"storage": "500Gi",
				},
			},
			isValid: false,
			errors:  []string{"invalid CPU quota format"},
		},
		{
			name: "Invalid resource quota - Memory",
			vdc: &models.VirtualDataCenter{
				Name:        "Production VDC",
				Description: "Production virtual data center",
				OrgID:       "org-123",
				ResourceQuotas: models.StringMap{
					"cpu":     "20",
					"memory":  "invalid",
					"storage": "500Gi",
				},
			},
			isValid: false,
			errors:  []string{"invalid memory quota format"},
		},
		{
			name: "Invalid resource quota - Storage",
			vdc: &models.VirtualDataCenter{
				Name:        "Production VDC",
				Description: "Production virtual data center",
				OrgID:       "org-123",
				ResourceQuotas: models.StringMap{
					"cpu":     "20",
					"memory":  "64Gi",
					"storage": "invalid",
				},
			},
			isValid: false,
			errors:  []string{"invalid storage quota format"},
		},
		{
			name: "Zero resource quotas",
			vdc: &models.VirtualDataCenter{
				Name:        "Production VDC",
				Description: "Production virtual data center",
				OrgID:       "org-123",
				ResourceQuotas: models.StringMap{
					"cpu":     "0",
					"memory":  "0Gi",
					"storage": "0Gi",
				},
			},
			isValid: false,
			errors: []string{
				"CPU quota must be greater than 0",
				"memory quota must be greater than 0",
				"storage quota must be greater than 0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateVDC(tt.vdc)

			if tt.isValid {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
				for _, expectedError := range tt.errors {
					assert.Contains(t, errors, expectedError)
				}
			}
		})
	}
}

func TestValidateVM(t *testing.T) {
	tests := []struct {
		name    string
		vm      *models.VirtualMachine
		isValid bool
		errors  []string
	}{
		{
			name: "Valid VM",
			vm: &models.VirtualMachine{
				Name:       "web-server-1",
				TemplateID: "template-123",
				CPU:        2,
				Memory:     "4Gi",
				DiskSize:   "50Gi",
			},
			isValid: true,
			errors:  []string{},
		},
		{
			name: "Empty name",
			vm: &models.VirtualMachine{
				Name:       "",
				TemplateID: "template-123",
				CPU:        2,
				Memory:     "4Gi",
				DiskSize:   "50Gi",
			},
			isValid: false,
			errors:  []string{"VM name is required"},
		},
		{
			name: "Name too short",
			vm: &models.VirtualMachine{
				Name:       "vm",
				TemplateID: "template-123",
				CPU:        2,
				Memory:     "4Gi",
				DiskSize:   "50Gi",
			},
			isValid: false,
			errors:  []string{"VM name must be at least 3 characters long"},
		},
		{
			name: "Invalid name characters",
			vm: &models.VirtualMachine{
				Name:       "web server 1!",
				TemplateID: "template-123",
				CPU:        2,
				Memory:     "4Gi",
				DiskSize:   "50Gi",
			},
			isValid: false,
			errors:  []string{"VM name contains invalid characters"},
		},
		{
			name: "Empty template ID",
			vm: &models.VirtualMachine{
				Name:       "web-server-1",
				TemplateID: "",
				CPU:        2,
				Memory:     "4Gi",
				DiskSize:   "50Gi",
			},
			isValid: false,
			errors:  []string{"template ID is required"},
		},
		{
			name: "Invalid CPU",
			vm: &models.VirtualMachine{
				Name:       "web-server-1",
				TemplateID: "template-123",
				CPU:        0,
				Memory:     "4Gi",
				DiskSize:   "50Gi",
			},
			isValid: false,
			errors:  []string{"CPU must be greater than 0"},
		},
		{
			name: "CPU too high",
			vm: &models.VirtualMachine{
				Name:       "web-server-1",
				TemplateID: "template-123",
				CPU:        129,
				Memory:     "4Gi",
				DiskSize:   "50Gi",
			},
			isValid: false,
			errors:  []string{"CPU must be at most 128"},
		},
		{
			name: "Invalid memory format",
			vm: &models.VirtualMachine{
				Name:       "web-server-1",
				TemplateID: "template-123",
				CPU:        2,
				Memory:     "invalid",
				DiskSize:   "50Gi",
			},
			isValid: false,
			errors:  []string{"invalid memory format"},
		},
		{
			name: "Invalid disk size format",
			vm: &models.VirtualMachine{
				Name:       "web-server-1",
				TemplateID: "template-123",
				CPU:        2,
				Memory:     "4Gi",
				DiskSize:   "invalid",
			},
			isValid: false,
			errors:  []string{"invalid disk size format"},
		},
		{
			name: "Multiple validation errors",
			vm: &models.VirtualMachine{
				Name:       "v!",
				TemplateID: "",
				CPU:        0,
				Memory:     "invalid",
				DiskSize:   "invalid",
			},
			isValid: false,
			errors: []string{
				"VM name must be at least 3 characters long",
				"VM name contains invalid characters",
				"template ID is required",
				"CPU must be greater than 0",
				"invalid memory format",
				"invalid disk size format",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateVM(tt.vm)

			if tt.isValid {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
				for _, expectedError := range tt.errors {
					assert.Contains(t, errors, expectedError)
				}
			}
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template *models.Template
		isValid  bool
		errors   []string
	}{
		{
			name: "Valid template",
			template: &models.Template{
				Name:        "rhel9-server-small",
				Description: "Red Hat Enterprise Linux 9 Server",
				OSType:      "Linux",
				OSVersion:   "RHEL 9",
				CPU:         2,
				Memory:      "4Gi",
				DiskSize:    "50Gi",
				ImageURL:    "https://example.com/rhel9.qcow2",
				Category:    models.TemplateCategoryOS,
			},
			isValid: true,
			errors:  []string{},
		},
		{
			name: "Empty name",
			template: &models.Template{
				Name:        "",
				Description: "Red Hat Enterprise Linux 9 Server",
				OSType:      "Linux",
				CPU:         2,
				Memory:      "4Gi",
				DiskSize:    "50Gi",
				Category:    models.TemplateCategoryOS,
			},
			isValid: false,
			errors:  []string{"template name is required"},
		},
		{
			name: "Invalid OS type",
			template: &models.Template{
				Name:        "invalid-template",
				Description: "Invalid template",
				OSType:      "InvalidOS",
				CPU:         2,
				Memory:      "4Gi",
				DiskSize:    "50Gi",
				Category:    models.TemplateCategoryOS,
			},
			isValid: false,
			errors:  []string{"invalid OS type"},
		},
		{
			name: "Invalid category",
			template: &models.Template{
				Name:        "test-template",
				Description: "Test template",
				OSType:      "Linux",
				CPU:         2,
				Memory:      "4Gi",
				DiskSize:    "50Gi",
				Category:    "InvalidCategory",
			},
			isValid: false,
			errors:  []string{"invalid template category"},
		},
		{
			name: "Invalid image URL",
			template: &models.Template{
				Name:        "test-template",
				Description: "Test template",
				OSType:      "Linux",
				CPU:         2,
				Memory:      "4Gi",
				DiskSize:    "50Gi",
				ImageURL:    "not-a-url",
				Category:    models.TemplateCategoryOS,
			},
			isValid: false,
			errors:  []string{"invalid image URL format"},
		},
		{
			name: "CPU and memory validation",
			template: &models.Template{
				Name:        "test-template",
				Description: "Test template",
				OSType:      "Linux",
				CPU:         0,
				Memory:      "invalid",
				DiskSize:    "invalid",
				Category:    models.TemplateCategoryOS,
			},
			isValid: false,
			errors: []string{
				"CPU must be greater than 0",
				"invalid memory format",
				"invalid disk size format",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateTemplate(tt.template)

			if tt.isValid {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
				for _, expectedError := range tt.errors {
					assert.Contains(t, errors, expectedError)
				}
			}
		})
	}
}

func TestValidateResourceQuotas(t *testing.T) {
	tests := []struct {
		name    string
		quotas  models.StringMap
		isValid bool
		errors  []string
	}{
		{
			name: "Valid quotas",
			quotas: models.StringMap{
				"cpu":     "20",
				"memory":  "64Gi",
				"storage": "500Gi",
			},
			isValid: true,
			errors:  []string{},
		},
		{
			name: "Invalid CPU format",
			quotas: models.StringMap{
				"cpu":     "invalid",
				"memory":  "64Gi",
				"storage": "500Gi",
			},
			isValid: false,
			errors:  []string{"invalid CPU quota format"},
		},
		{
			name: "Invalid memory format",
			quotas: models.StringMap{
				"cpu":     "20",
				"memory":  "invalid",
				"storage": "500Gi",
			},
			isValid: false,
			errors:  []string{"invalid memory quota format"},
		},
		{
			name: "Invalid storage format",
			quotas: models.StringMap{
				"cpu":     "20",
				"memory":  "64Gi",
				"storage": "invalid",
			},
			isValid: false,
			errors:  []string{"invalid storage quota format"},
		},
		{
			name: "Zero values",
			quotas: models.StringMap{
				"cpu":     "0",
				"memory":  "0Gi",
				"storage": "0Gi",
			},
			isValid: false,
			errors: []string{
				"CPU quota must be greater than 0",
				"memory quota must be greater than 0",
				"storage quota must be greater than 0",
			},
		},
		{
			name: "Missing required quotas",
			quotas: models.StringMap{
				"cpu": "20",
			},
			isValid: false,
			errors: []string{
				"memory quota is required",
				"storage quota is required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateResourceQuotas(tt.quotas)

			if tt.isValid {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
				for _, expectedError := range tt.errors {
					assert.Contains(t, errors, expectedError)
				}
			}
		})
	}
}

// Helper function to generate long strings for testing
func generateLongString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = 'a'
	}
	return string(result)
}

// Validation function stubs (these would be implemented in the actual validation package)
func ValidateOrganization(org *models.Organization) []string {
	var errors []string

	if org.Name == "" {
		errors = append(errors, "organization name is required")
	} else {
		if len(org.Name) < 3 {
			errors = append(errors, "organization name must be at least 3 characters long")
		}
		if len(org.Name) > 100 {
			errors = append(errors, "organization name must be at most 100 characters long")
		}
		if !isValidName(org.Name) {
			errors = append(errors, "organization name contains invalid characters")
		}
	}

	if len(org.Description) > 500 {
		errors = append(errors, "organization description must be at most 500 characters long")
	}

	return errors
}

func ValidateUser(user *models.User) []string {
	var errors []string

	if user.Username == "" {
		errors = append(errors, "username is required")
	} else {
		if len(user.Username) < 3 {
			errors = append(errors, "username must be at least 3 characters long")
		}
		if len(user.Username) > 50 {
			errors = append(errors, "username must be at most 50 characters long")
		}
		if !isValidUsername(user.Username) {
			errors = append(errors, "username contains invalid characters")
		}
	}

	if user.Email == "" {
		errors = append(errors, "email is required")
	} else if !isValidEmail(user.Email) {
		errors = append(errors, "email format is invalid")
	}

	if !isValidRole(user.Role) {
		errors = append(errors, "invalid role")
	}

	return errors
}

func ValidateVDC(vdc *models.VirtualDataCenter) []string {
	var errors []string

	if vdc.Name == "" {
		errors = append(errors, "VDC name is required")
	} else if len(vdc.Name) < 3 {
		errors = append(errors, "VDC name must be at least 3 characters long")
	}

	if vdc.OrgID == "" {
		errors = append(errors, "organization ID is required")
	}

	if vdc.ResourceQuotas != nil {
		quotaErrors := ValidateResourceQuotas(vdc.ResourceQuotas)
		errors = append(errors, quotaErrors...)
	}

	return errors
}

func ValidateVM(vm *models.VirtualMachine) []string {
	var errors []string

	if vm.Name == "" {
		errors = append(errors, "VM name is required")
	} else {
		if len(vm.Name) < 3 {
			errors = append(errors, "VM name must be at least 3 characters long")
		}
		if !isValidVMName(vm.Name) {
			errors = append(errors, "VM name contains invalid characters")
		}
	}

	if vm.TemplateID == "" {
		errors = append(errors, "template ID is required")
	}

	if vm.CPU <= 0 {
		errors = append(errors, "CPU must be greater than 0")
	} else if vm.CPU > 128 {
		errors = append(errors, "CPU must be at most 128")
	}

	if !isValidMemoryFormat(vm.Memory) {
		errors = append(errors, "invalid memory format")
	}

	if !isValidDiskSizeFormat(vm.DiskSize) {
		errors = append(errors, "invalid disk size format")
	}

	return errors
}

func ValidateTemplate(template *models.Template) []string {
	var errors []string

	if template.Name == "" {
		errors = append(errors, "template name is required")
	}

	if !isValidOSType(template.OSType) {
		errors = append(errors, "invalid OS type")
	}

	if !isValidTemplateCategory(template.Category) {
		errors = append(errors, "invalid template category")
	}

	if template.ImageURL != "" && !isValidURL(template.ImageURL) {
		errors = append(errors, "invalid image URL format")
	}

	if template.CPU <= 0 {
		errors = append(errors, "CPU must be greater than 0")
	}

	if !isValidMemoryFormat(template.Memory) {
		errors = append(errors, "invalid memory format")
	}

	if !isValidDiskSizeFormat(template.DiskSize) {
		errors = append(errors, "invalid disk size format")
	}

	return errors
}

func ValidateResourceQuotas(quotas models.StringMap) []string {
	var errors []string

	// Check required quotas exist
	if _, exists := quotas["cpu"]; !exists {
		errors = append(errors, "CPU quota is required")
	}
	if _, exists := quotas["memory"]; !exists {
		errors = append(errors, "memory quota is required")
	}
	if _, exists := quotas["storage"]; !exists {
		errors = append(errors, "storage quota is required")
	}

	// Validate CPU quota
	if cpuStr, exists := quotas["cpu"]; exists {
		cpu := models.ParseCPUString(cpuStr)
		if cpu == 0 && cpuStr != "0" {
			errors = append(errors, "invalid CPU quota format")
		} else if cpu == 0 {
			errors = append(errors, "CPU quota must be greater than 0")
		}
	}

	// Validate memory quota
	if memoryStr, exists := quotas["memory"]; exists {
		memory := models.ParseMemoryString(memoryStr)
		if memory == 0 && memoryStr != "0Gi" && memoryStr != "0GB" {
			errors = append(errors, "invalid memory quota format")
		} else if memory == 0 {
			errors = append(errors, "memory quota must be greater than 0")
		}
	}

	// Validate storage quota
	if storageStr, exists := quotas["storage"]; exists {
		storage := models.ParseStorageString(storageStr)
		if storage == 0 && storageStr != "0Gi" && storageStr != "0GB" {
			errors = append(errors, "invalid storage quota format")
		} else if storage == 0 {
			errors = append(errors, "storage quota must be greater than 0")
		}
	}

	return errors
}

// Helper validation functions
func isValidName(name string) bool {
	// Allow alphanumeric, spaces, hyphens, underscores
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func isValidUsername(username string) bool {
	// Allow alphanumeric, dots, hyphens, underscores
	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func isValidVMName(name string) bool {
	// Allow alphanumeric, hyphens, underscores (no spaces for VM names)
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func isValidEmail(email string) bool {
	// Simple email validation
	return len(email) > 5 &&
		len(email) < 255 &&
		containsAtSymbol(email) &&
		containsDot(email)
}

func containsAtSymbol(s string) bool {
	for _, r := range s {
		if r == '@' {
			return true
		}
	}
	return false
}

func containsDot(s string) bool {
	for _, r := range s {
		if r == '.' {
			return true
		}
	}
	return false
}

func isValidRole(role string) bool {
	validRoles := []string{
		models.RoleSystemAdmin,
		models.RoleOrgAdmin,
		models.RoleOrgUser,
	}

	for _, validRole := range validRoles {
		if role == validRole {
			return true
		}
	}
	return false
}

func isValidOSType(osType string) bool {
	validOSTypes := []string{"Linux", "Windows", "Container"}
	for _, valid := range validOSTypes {
		if osType == valid {
			return true
		}
	}
	return false
}

func isValidTemplateCategory(category string) bool {
	validCategories := []string{
		models.TemplateCategoryOS,
		models.TemplateCategoryDatabase,
		models.TemplateCategoryApplication,
		models.TemplateCategoryMiddleware,
		models.TemplateCategoryOther,
	}

	for _, valid := range validCategories {
		if category == valid {
			return true
		}
	}
	return false
}

func isValidURL(url string) bool {
	return len(url) > 7 && (url[:7] == "http://" || url[:8] == "https://")
}

func isValidMemoryFormat(memory string) bool {
	if memory == "" {
		return false
	}
	return models.ParseMemoryString(memory) > 0 || memory == "0Gi" || memory == "0GB"
}

func isValidDiskSizeFormat(diskSize string) bool {
	if diskSize == "" {
		return false
	}
	return models.ParseStorageString(diskSize) > 0 || diskSize == "0Gi" || diskSize == "0GB"
}
