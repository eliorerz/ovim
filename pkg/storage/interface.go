package storage

import (
	"github.com/eliorerz/ovim-updated/pkg/models"
)

// Storage defines the interface for data storage operations
type Storage interface {
	// User operations
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

	// Health check
	Ping() error
	Close() error
}
