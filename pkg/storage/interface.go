package storage

import (
	"github.com/eliorerz/ovim-updated/pkg/models"
)

// Storage defines the interface for data storage operations
type Storage interface {
	// User operations
	ListUsers() ([]*models.User, error)
	ListUsersByOrg(orgID string) ([]*models.User, error)
	GetUserByUsername(username string) (*models.User, error)
	GetUserByID(id string) (*models.User, error)
	CreateUser(user *models.User) error
	UpdateUser(user *models.User) error
	DeleteUser(id string) error

	// Organization operations
	ListOrganizations() ([]*models.Organization, error)
	GetOrganization(id string) (*models.Organization, error)
	CreateOrganization(org *models.Organization) error
	UpdateOrganization(org *models.Organization) error
	DeleteOrganization(id string) error

	// VDC operations
	ListVDCs(orgID string) ([]*models.VirtualDataCenter, error)
	GetVDC(id string) (*models.VirtualDataCenter, error)
	CreateVDC(vdc *models.VirtualDataCenter) error
	UpdateVDC(vdc *models.VirtualDataCenter) error
	DeleteVDC(id string) error

	// Template operations
	ListTemplates() ([]*models.Template, error)
	ListTemplatesByOrg(orgID string) ([]*models.Template, error)
	GetTemplate(id string) (*models.Template, error)
	CreateTemplate(template *models.Template) error
	UpdateTemplate(template *models.Template) error
	DeleteTemplate(id string) error

	// VM operations
	ListVMs(orgID string) ([]*models.VirtualMachine, error)
	GetVM(id string) (*models.VirtualMachine, error)
	CreateVM(vm *models.VirtualMachine) error
	UpdateVM(vm *models.VirtualMachine) error
	DeleteVM(id string) error

	// Organization Catalog Source operations
	ListOrganizationCatalogSources(orgID string) ([]*models.OrganizationCatalogSource, error)
	GetOrganizationCatalogSource(id string) (*models.OrganizationCatalogSource, error)
	CreateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error
	UpdateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error
	DeleteOrganizationCatalogSource(id string) error

	// Event operations
	ListEvents(filter *models.EventFilter) (*models.EventsResponse, error)
	GetEvent(id string) (*models.Event, error)
	CreateEvent(event *models.Event) error
	CreateEvents(events []*models.Event) error
	UpdateEvent(event *models.Event) error
	DeleteEvent(id string) error
	CleanupOldEvents() (int, error)

	// Event category operations
	ListEventCategories() ([]*models.EventCategory, error)
	GetEventCategory(name string) (*models.EventCategory, error)

	// Event retention policy operations
	ListEventRetentionPolicies() ([]*models.EventRetentionPolicy, error)
	GetEventRetentionPolicy(category, eventType string) (*models.EventRetentionPolicy, error)
	UpdateEventRetentionPolicy(policy *models.EventRetentionPolicy) error

	// Zone operations
	ListZones() ([]*models.Zone, error)
	GetZone(id string) (*models.Zone, error)
	CreateZone(zone *models.Zone) error
	UpdateZone(zone *models.Zone) error
	DeleteZone(id string) error
	GetZoneUtilization() ([]*models.ZoneUtilization, error)

	// Organization Zone Quota operations
	ListOrganizationZoneQuotas(orgID string) ([]*models.OrganizationZoneQuota, error)
	GetOrganizationZoneQuota(orgID, zoneID string) (*models.OrganizationZoneQuota, error)
	CreateOrganizationZoneQuota(quota *models.OrganizationZoneQuota) error
	UpdateOrganizationZoneQuota(quota *models.OrganizationZoneQuota) error
	DeleteOrganizationZoneQuota(orgID, zoneID string) error
	GetOrganizationZoneAccess(orgID string) ([]*models.OrganizationZoneAccess, error)

	// Health check
	Ping() error
	Close() error
}
