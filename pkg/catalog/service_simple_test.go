package catalog

import (
	"testing"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestDetermineCategory(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name         string
		osType       string
		templateName string
		description  string
		expected     string
	}{
		{
			name:         "Database template by name",
			osType:       "Linux",
			templateName: "postgres-server-template",
			description:  "",
			expected:     models.TemplateCategoryDatabase,
		},
		{
			name:         "Database template by description",
			osType:       "Linux",
			templateName: "db-template",
			description:  "A powerful database solution",
			expected:     models.TemplateCategoryDatabase,
		},
		{
			name:         "Middleware template",
			osType:       "Linux",
			templateName: "nginx-proxy",
			description:  "",
			expected:     models.TemplateCategoryMiddleware,
		},
		{
			name:         "Application template",
			osType:       "Linux",
			templateName: "webapp-service",
			description:  "",
			expected:     models.TemplateCategoryApplication,
		},
		{
			name:         "OS template (default)",
			osType:       "Linux",
			templateName: "rhel9-server",
			description:  "",
			expected:     models.TemplateCategoryOS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.determineCategory(tt.osType, tt.templateName, tt.description)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsFeaturedTemplate(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name     string
		template string
		expected bool
	}{
		{
			name:     "Featured RHEL template",
			template: "rhel9-server-small",
			expected: true,
		},
		{
			name:     "Featured Ubuntu template",
			template: "ubuntu-server-small",
			expected: true,
		},
		{
			name:     "Non-featured template",
			template: "custom-application",
			expected: false,
		},
		{
			name:     "Case insensitive match",
			template: "RHEL9-SERVER-SMALL",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isFeaturedTemplate(tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterByCategory(t *testing.T) {
	service := &Service{}

	templates := []*models.Template{
		{
			ID:       "template-1",
			Name:     "postgres-template",
			Category: models.TemplateCategoryDatabase,
		},
		{
			ID:       "template-2",
			Name:     "rhel9-server",
			Category: models.TemplateCategoryOS,
		},
		{
			ID:       "template-3",
			Name:     "nginx-proxy",
			Category: models.TemplateCategoryMiddleware,
		},
	}

	result := service.filterByCategory(templates, models.TemplateCategoryDatabase)

	assert.Len(t, result, 1)
	assert.Equal(t, "template-1", result[0].ID)
	assert.Equal(t, models.TemplateCategoryDatabase, result[0].Category)
}

func TestDeduplicateTemplates(t *testing.T) {
	service := &Service{}

	templates := []*models.Template{
		{
			ID:   "template-1",
			Name: "first-template",
		},
		{
			ID:   "template-2",
			Name: "second-template",
		},
		{
			ID:   "template-1", // Duplicate ID
			Name: "duplicate-template",
		},
	}

	result := service.deduplicateTemplates(templates)

	assert.Len(t, result, 2)

	// Should keep the first occurrence
	ids := make(map[string]bool)
	for _, template := range result {
		assert.False(t, ids[template.ID], "Duplicate ID found: %s", template.ID)
		ids[template.ID] = true
	}
}

func TestNewService_Structure(t *testing.T) {
	// Test with nil client (database-only mode)
	service := NewService(nil, nil, "openshift")

	assert.NotNil(t, service)
	assert.Nil(t, service.storage)
	assert.Nil(t, service.osClient)
	assert.Equal(t, "openshift", service.globalNS)
	assert.Equal(t, "-templates", service.orgTemplateSuffix)
}
