package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/openshift"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"k8s.io/klog/v2"
)

// CatalogSource represents a source of templates
type CatalogSource struct {
	Type      string // global, organization, external
	Name      string
	Namespace string
	OrgID     string // for organization-specific catalogs
	Enabled   bool
}

// Service provides catalog management with multiple sources
type Service struct {
	storage           storage.Storage
	osClient          *openshift.Client
	globalNS          string
	orgTemplateSuffix string
}

// NewService creates a new catalog service
func NewService(storage storage.Storage, osClient *openshift.Client, globalNS string) *Service {
	return &Service{
		storage:           storage,
		osClient:          osClient,
		globalNS:          globalNS,
		orgTemplateSuffix: "-templates",
	}
}

// GetTemplates retrieves templates from multiple sources
func (s *Service) GetTemplates(ctx context.Context, userOrgID string, source string, category string) ([]*models.Template, error) {
	var allTemplates []*models.Template

	// Get global templates from OpenShift if source is empty or "global"
	if source == "" || source == models.TemplateSourceGlobal {
		globalTemplates, err := s.getGlobalTemplates(ctx)
		if err != nil {
			klog.Errorf("Failed to get global templates: %v", err)
		} else {
			allTemplates = append(allTemplates, globalTemplates...)
		}
	}

	// Get organization templates if source is empty or "organization"
	if (source == "" || source == models.TemplateSourceOrganization) && userOrgID != "" {
		orgTemplates, err := s.getOrganizationTemplates(ctx, userOrgID)
		if err != nil {
			klog.Errorf("Failed to get organization templates for org %s: %v", userOrgID, err)
		} else {
			allTemplates = append(allTemplates, orgTemplates...)
		}
	}

	// Get stored templates from database
	if source == "" {
		dbTemplates, err := s.storage.ListTemplates()
		if err != nil {
			klog.Errorf("Failed to get database templates: %v", err)
		} else {
			allTemplates = append(allTemplates, dbTemplates...)
		}
	} else if source == models.TemplateSourceOrganization && userOrgID != "" {
		dbTemplates, err := s.storage.ListTemplatesByOrg(userOrgID)
		if err != nil {
			klog.Errorf("Failed to get organization templates from database: %v", err)
		} else {
			allTemplates = append(allTemplates, dbTemplates...)
		}
	}

	// Filter by category if specified
	if category != "" {
		allTemplates = s.filterByCategory(allTemplates, category)
	}

	// Deduplicate templates by ID
	return s.deduplicateTemplates(allTemplates), nil
}

// getGlobalTemplates retrieves templates from the global OpenShift namespace
func (s *Service) getGlobalTemplates(ctx context.Context) ([]*models.Template, error) {
	if s.osClient == nil {
		return nil, fmt.Errorf("OpenShift client not available")
	}

	osTemplates, err := s.osClient.GetTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenShift templates: %w", err)
	}

	var templates []*models.Template
	for _, osTemplate := range osTemplates {
		template := s.convertOpenShiftTemplate(osTemplate, models.TemplateSourceGlobal, "Red Hat")
		templates = append(templates, template)
	}

	return templates, nil
}

// getOrganizationTemplates retrieves templates from organization-specific namespace
func (s *Service) getOrganizationTemplates(ctx context.Context, orgID string) ([]*models.Template, error) {
	if s.osClient == nil {
		return nil, fmt.Errorf("OpenShift client not available")
	}

	// Get organization info to determine namespace
	org, err := s.storage.GetOrganization(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Try to get templates from organization's template namespace
	templateNamespace := org.Namespace + s.orgTemplateSuffix

	osTemplates, err := s.getTemplatesFromNamespace(ctx, templateNamespace)
	if err != nil {
		klog.V(4).Infof("No templates found in organization namespace %s: %v", templateNamespace, err)
		return []*models.Template{}, nil
	}

	var templates []*models.Template
	for _, osTemplate := range osTemplates {
		template := s.convertOpenShiftTemplate(osTemplate, models.TemplateSourceOrganization, org.Name)
		template.OrgID = orgID
		template.Namespace = templateNamespace
		templates = append(templates, template)
	}

	return templates, nil
}

// getTemplatesFromNamespace gets templates from a specific namespace
func (s *Service) getTemplatesFromNamespace(ctx context.Context, namespace string) ([]openshift.Template, error) {
	if s.osClient == nil {
		return nil, fmt.Errorf("OpenShift client not available")
	}

	klog.V(4).Infof("Getting templates from namespace: %s", namespace)
	return s.osClient.GetTemplatesFromNamespace(ctx, namespace)
}

// convertOpenShiftTemplate converts an OpenShift template to our model
func (s *Service) convertOpenShiftTemplate(osTemplate openshift.Template, source, sourceVendor string) *models.Template {
	category := s.determineCategory(osTemplate.OSType, osTemplate.Name, osTemplate.Description)

	return &models.Template{
		ID:           osTemplate.ID,
		Name:         osTemplate.Name,
		TemplateName: osTemplate.TemplateName, // Actual OpenShift template name
		Description:  osTemplate.Description,
		OSType:       osTemplate.OSType,
		OSVersion:    osTemplate.OSVersion,
		CPU:          osTemplate.CPU,
		Memory:       osTemplate.Memory,
		DiskSize:     osTemplate.DiskSize,
		ImageURL:     osTemplate.ImageURL,
		IconClass:    osTemplate.IconClass,
		Source:       source,
		SourceVendor: sourceVendor,
		Category:     category,
		Namespace:    osTemplate.Namespace,
		Featured:     s.isFeaturedTemplate(osTemplate.Name),
		Metadata: models.StringMap{
			"openshift_template": "true",
			"source_namespace":   osTemplate.Namespace,
		},
	}
}

// determineCategory determines the template category based on OS type and name
func (s *Service) determineCategory(osType, name, description string) string {
	name = strings.ToLower(name)
	description = strings.ToLower(description)

	// Database templates
	if strings.Contains(name, "postgres") || strings.Contains(name, "mysql") ||
		strings.Contains(name, "mariadb") || strings.Contains(name, "mongodb") ||
		strings.Contains(description, "database") {
		return models.TemplateCategoryDatabase
	}

	// Middleware templates
	if strings.Contains(name, "redis") || strings.Contains(name, "nginx") ||
		strings.Contains(name, "apache") || strings.Contains(name, "tomcat") ||
		strings.Contains(description, "middleware") {
		return models.TemplateCategoryMiddleware
	}

	// Application templates
	if strings.Contains(name, "app") || strings.Contains(name, "service") ||
		strings.Contains(description, "application") {
		return models.TemplateCategoryApplication
	}

	// Default to OS category
	return models.TemplateCategoryOS
}

// isFeaturedTemplate determines if a template should be featured
func (s *Service) isFeaturedTemplate(name string) bool {
	featuredTemplates := []string{
		"rhel9-server-small",
		"rhel8-server-small",
		"centos8-server-small",
		"fedora-server-small",
		"ubuntu-server-small",
	}

	for _, featured := range featuredTemplates {
		if strings.Contains(strings.ToLower(name), featured) {
			return true
		}
	}
	return false
}

// filterByCategory filters templates by category
func (s *Service) filterByCategory(templates []*models.Template, category string) []*models.Template {
	var filtered []*models.Template
	for _, template := range templates {
		if template.Category == category {
			filtered = append(filtered, template)
		}
	}
	return filtered
}

// deduplicateTemplates removes duplicate templates by ID
func (s *Service) deduplicateTemplates(templates []*models.Template) []*models.Template {
	seen := make(map[string]bool)
	var unique []*models.Template

	for _, template := range templates {
		if !seen[template.ID] {
			seen[template.ID] = true
			unique = append(unique, template)
		}
	}

	return unique
}

// GetCatalogSources returns available catalog sources for an organization
func (s *Service) GetCatalogSources(ctx context.Context, userOrgID string) ([]CatalogSource, error) {
	sources := []CatalogSource{
		{
			Type:      models.TemplateSourceGlobal,
			Name:      "Red Hat Catalog",
			Namespace: s.globalNS,
			Enabled:   true,
		},
	}

	// Add organization catalog if user belongs to an organization
	if userOrgID != "" {
		org, err := s.storage.GetOrganization(userOrgID)
		if err == nil {
			sources = append(sources, CatalogSource{
				Type:      models.TemplateSourceOrganization,
				Name:      fmt.Sprintf("%s Templates", org.Name),
				Namespace: org.Namespace + s.orgTemplateSuffix,
				OrgID:     userOrgID,
				Enabled:   true,
			})
		}
	}

	return sources, nil
}
