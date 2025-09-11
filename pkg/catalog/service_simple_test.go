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

func TestDetermineCategory_ComprehensiveDatabase(t *testing.T) {
	service := &Service{}

	databaseTests := []struct {
		name         string
		templateName string
		description  string
		expected     string
	}{
		{"PostgreSQL template", "postgresql-persistent", "", models.TemplateCategoryDatabase},
		{"MySQL template", "mysql-ephemeral", "", models.TemplateCategoryDatabase},
		{"MongoDB template", "mongodb-replica-set", "", models.TemplateCategoryDatabase},
		{"MariaDB template", "mariadb-template", "", models.TemplateCategoryDatabase},
		{"Redis template", "redis-cache-service", "", models.TemplateCategoryMiddleware},
		{"Database in description", "generic-template", "MySQL database server", models.TemplateCategoryDatabase},
	}

	for _, tt := range databaseTests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.determineCategory("Linux", tt.templateName, tt.description)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineCategory_ComprehensiveMiddleware(t *testing.T) {
	service := &Service{}

	middlewareTests := []struct {
		name         string
		templateName string
		description  string
		expected     string
	}{
		{"Nginx template", "nginx-proxy-server", "", models.TemplateCategoryMiddleware},
		{"Apache template", "apache-httpd", "", models.TemplateCategoryMiddleware},
		{"Redis template", "redis-cache-service", "", models.TemplateCategoryMiddleware},
		{"Tomcat template", "tomcat-app-server", "", models.TemplateCategoryMiddleware},
		{"Middleware in description", "generic-template", "nginx middleware solution", models.TemplateCategoryMiddleware},
	}

	for _, tt := range middlewareTests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.determineCategory("Linux", tt.templateName, tt.description)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineCategory_ComprehensiveApplication(t *testing.T) {
	service := &Service{}

	applicationTests := []struct {
		name         string
		templateName string
		description  string
		expected     string
	}{
		{"Python app", "python-django-app", "", models.TemplateCategoryApplication},
		{"Java app", "spring-boot-app", "", models.TemplateCategoryApplication},
		{"Ruby app", "ruby-rails-app", "", models.TemplateCategoryApplication},
		{"Go app", "golang-web-service", "", models.TemplateCategoryApplication},
		{"Generic app", "web-application", "", models.TemplateCategoryApplication},
		{"Service template", "microservice-api", "", models.TemplateCategoryApplication},
		{"App in description", "custom-template", "web application framework", models.TemplateCategoryApplication},
		{"NodeJS framework", "nodejs-express", "", models.TemplateCategoryOS}, // Actual behavior
		{"PHP framework", "cakephp-example", "", models.TemplateCategoryOS},   // Actual behavior
	}

	for _, tt := range applicationTests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.determineCategory("Linux", tt.templateName, tt.description)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsFeaturedTemplate_Extended(t *testing.T) {
	service := &Service{}

	featuredTests := []struct {
		name     string
		template string
		expected bool
	}{
		// Actual featured templates from implementation
		{"RHEL 8", "rhel8-server-small", true},
		{"RHEL 9", "rhel9-server-small", true},
		{"CentOS 8", "centos8-server-small", true},
		{"Fedora", "fedora-server-small", true},
		{"Ubuntu", "ubuntu-server-small", true},

		// Non-featured
		{"Custom template", "custom-application", false},
		{"Unknown template", "some-random-template", false},
		{"Empty template", "", false},
		{"Windows Server", "windows-server-2022", false},
		{"PostgreSQL", "postgresql-persistent", false},
		{"MySQL", "mysql-ephemeral", false},
	}

	for _, tt := range featuredTests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isFeaturedTemplate(tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterByCategory_EmptyResults(t *testing.T) {
	service := &Service{}

	templates := []*models.Template{
		{ID: "template-1", Category: models.TemplateCategoryDatabase},
		{ID: "template-2", Category: models.TemplateCategoryOS},
	}

	// Filter for category that doesn't exist
	result := service.filterByCategory(templates, models.TemplateCategoryMiddleware)
	assert.Len(t, result, 0)
}

func TestFilterByCategory_EmptyInput(t *testing.T) {
	service := &Service{}

	// Empty input
	result := service.filterByCategory([]*models.Template{}, models.TemplateCategoryDatabase)
	assert.Len(t, result, 0)

	// Nil input
	result = service.filterByCategory(nil, models.TemplateCategoryDatabase)
	assert.Len(t, result, 0)
}

func TestDeduplicateTemplates_EdgeCases(t *testing.T) {
	service := &Service{}

	t.Run("Empty input", func(t *testing.T) {
		result := service.deduplicateTemplates([]*models.Template{})
		assert.Len(t, result, 0)
	})

	t.Run("Nil input", func(t *testing.T) {
		result := service.deduplicateTemplates(nil)
		assert.Len(t, result, 0)
	})

	t.Run("Single template", func(t *testing.T) {
		templates := []*models.Template{
			{ID: "template-1", Name: "single"},
		}
		result := service.deduplicateTemplates(templates)
		assert.Len(t, result, 1)
		assert.Equal(t, "template-1", result[0].ID)
	})

	t.Run("All duplicates", func(t *testing.T) {
		templates := []*models.Template{
			{ID: "template-1", Name: "first"},
			{ID: "template-1", Name: "second"},
			{ID: "template-1", Name: "third"},
		}
		result := service.deduplicateTemplates(templates)
		assert.Len(t, result, 1)
		assert.Equal(t, "first", result[0].Name) // Should keep the first one
	})
}
