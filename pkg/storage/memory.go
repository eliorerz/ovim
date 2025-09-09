package storage

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/util"
)

var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidInput  = errors.New("invalid input")
)

// MemoryStorage implements the Storage interface using in-memory storage
type MemoryStorage struct {
	users         map[string]*models.User
	organizations map[string]*models.Organization
	vdcs          map[string]*models.VirtualDataCenter
	templates     map[string]*models.Template
	vms           map[string]*models.VirtualMachine
	mutex         sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage instance
func NewMemoryStorage() (Storage, error) {
	storage := &MemoryStorage{
		users:         make(map[string]*models.User),
		organizations: make(map[string]*models.Organization),
		vdcs:          make(map[string]*models.VirtualDataCenter),
		templates:     make(map[string]*models.Template),
		vms:           make(map[string]*models.VirtualMachine),
	}

	if err := storage.seedData(); err != nil {
		return nil, fmt.Errorf("failed to seed data: %w", err)
	}

	klog.Info("Initialized in-memory storage with seed data")
	return storage, nil
}

// seedData populates the storage with initial test data
func (s *MemoryStorage) seedData() error {
	adminHash, err := auth.HashPassword("adminpassword")
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	userHash, err := auth.HashPassword("userpassword")
	if err != nil {
		return fmt.Errorf("failed to hash user password: %w", err)
	}

	now := time.Now()

	// Seed users
	users := []*models.User{
		{
			ID:           "user-admin",
			Username:     "admin",
			Email:        "admin@ovim.local",
			PasswordHash: adminHash,
			Role:         models.RoleSystemAdmin,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "user-orgadmin",
			Username:     "orgadmin",
			Email:        "orgadmin@acme.com",
			PasswordHash: adminHash,
			Role:         models.RoleOrgAdmin,
			OrgID:        util.StringPtr("org-acme"),
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "user-regular",
			Username:     "user",
			Email:        "user@acme.com",
			PasswordHash: userHash,
			Role:         models.RoleOrgUser,
			OrgID:        util.StringPtr("org-acme"),
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	for _, user := range users {
		s.users[user.ID] = user
	}

	// Seed organizations
	orgs := []*models.Organization{
		{
			ID:          "org-acme",
			Name:        "Acme Corporation",
			Description: "Main corporate organization",
			Namespace:   "acme-corp",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "org-dev",
			Name:        "Development Team",
			Description: "Development and testing environment",
			Namespace:   "dev-team",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	for _, org := range orgs {
		s.organizations[org.ID] = org
	}

	// Seed VDCs
	vdcs := []*models.VirtualDataCenter{
		{
			ID:             "vdc-acme-main",
			Name:           "Acme Main VDC",
			Description:    "Main virtual data center for Acme Corp",
			OrgID:          "org-acme",
			Namespace:      "acme-corp",
			ResourceQuotas: models.StringMap{"cpu": "20", "memory": "64Gi", "storage": "1Ti"},
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             "vdc-dev-main",
			Name:           "Development VDC",
			Description:    "Development virtual data center",
			OrgID:          "org-dev",
			Namespace:      "dev-team",
			ResourceQuotas: models.StringMap{"cpu": "10", "memory": "32Gi", "storage": "500Gi"},
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	for _, vdc := range vdcs {
		s.vdcs[vdc.ID] = vdc
	}

	// Seed templates
	templates := []*models.Template{
		{
			ID:          "tmpl-rhel9",
			Name:        "Red Hat Enterprise Linux 9.2",
			Description: "Latest RHEL 9.2 with security updates",
			OSType:      "Linux",
			OSVersion:   "RHEL 9.2",
			CPU:         2,
			Memory:      "4Gi",
			DiskSize:    "20Gi",
			ImageURL:    "registry.redhat.io/rhel9/rhel:latest",
			Metadata:    models.StringMap{"vendor": "Red Hat", "certified": "true"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "tmpl-ubuntu22",
			Name:        "Ubuntu Server 22.04 LTS",
			Description: "Ubuntu Server 22.04 LTS with cloud-init",
			OSType:      "Linux",
			OSVersion:   "Ubuntu 22.04",
			CPU:         2,
			Memory:      "2Gi",
			DiskSize:    "20Gi",
			ImageURL:    "registry.ubuntu.com/ubuntu:22.04",
			Metadata:    models.StringMap{"vendor": "Canonical", "lts": "true"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "tmpl-centos9",
			Name:        "CentOS Stream 9",
			Description: "CentOS Stream 9 development environment",
			OSType:      "Linux",
			OSVersion:   "CentOS Stream 9",
			CPU:         1,
			Memory:      "2Gi",
			DiskSize:    "20Gi",
			ImageURL:    "quay.io/centos/centos:stream9",
			Metadata:    models.StringMap{"vendor": "CentOS", "stream": "true"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	for _, template := range templates {
		s.templates[template.ID] = template
	}

	klog.Infof("Seeded storage with %d users, %d organizations, %d VDCs, %d templates",
		len(users), len(orgs), len(vdcs), len(templates))

	return nil
}

// User operations
func (s *MemoryStorage) GetUserByUsername(username string) (*models.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, user := range s.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStorage) GetUserByID(id string) (*models.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStorage) CreateUser(user *models.User) error {
	if user == nil || user.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.users[user.ID]; exists {
		return ErrAlreadyExists
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = user.CreatedAt
	s.users[user.ID] = user
	return nil
}

func (s *MemoryStorage) UpdateUser(user *models.User) error {
	if user == nil || user.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.users[user.ID]; !exists {
		return ErrNotFound
	}

	user.UpdatedAt = time.Now()
	s.users[user.ID] = user
	return nil
}

func (s *MemoryStorage) DeleteUser(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.users[id]; !exists {
		return ErrNotFound
	}

	delete(s.users, id)
	return nil
}

// Organization operations
func (s *MemoryStorage) ListOrganizations() ([]*models.Organization, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	orgs := make([]*models.Organization, 0, len(s.organizations))
	for _, org := range s.organizations {
		orgs = append(orgs, org)
	}
	return orgs, nil
}

func (s *MemoryStorage) GetOrganization(id string) (*models.Organization, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	org, exists := s.organizations[id]
	if !exists {
		return nil, ErrNotFound
	}
	return org, nil
}

func (s *MemoryStorage) CreateOrganization(org *models.Organization) error {
	if org == nil || org.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.organizations[org.ID]; exists {
		return ErrAlreadyExists
	}

	org.CreatedAt = time.Now()
	org.UpdatedAt = org.CreatedAt
	s.organizations[org.ID] = org
	return nil
}

func (s *MemoryStorage) UpdateOrganization(org *models.Organization) error {
	if org == nil || org.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.organizations[org.ID]; !exists {
		return ErrNotFound
	}

	org.UpdatedAt = time.Now()
	s.organizations[org.ID] = org
	return nil
}

func (s *MemoryStorage) DeleteOrganization(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.organizations[id]; !exists {
		return ErrNotFound
	}

	delete(s.organizations, id)
	return nil
}

// VDC operations
func (s *MemoryStorage) ListVDCs(orgID string) ([]*models.VirtualDataCenter, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vdcs := make([]*models.VirtualDataCenter, 0)
	for _, vdc := range s.vdcs {
		if orgID == "" || vdc.OrgID == orgID {
			vdcs = append(vdcs, vdc)
		}
	}
	return vdcs, nil
}

func (s *MemoryStorage) GetVDC(id string) (*models.VirtualDataCenter, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vdc, exists := s.vdcs[id]
	if !exists {
		return nil, ErrNotFound
	}
	return vdc, nil
}

func (s *MemoryStorage) CreateVDC(vdc *models.VirtualDataCenter) error {
	if vdc == nil || vdc.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.vdcs[vdc.ID]; exists {
		return ErrAlreadyExists
	}

	vdc.CreatedAt = time.Now()
	vdc.UpdatedAt = vdc.CreatedAt
	s.vdcs[vdc.ID] = vdc
	return nil
}

func (s *MemoryStorage) UpdateVDC(vdc *models.VirtualDataCenter) error {
	if vdc == nil || vdc.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.vdcs[vdc.ID]; !exists {
		return ErrNotFound
	}

	vdc.UpdatedAt = time.Now()
	s.vdcs[vdc.ID] = vdc
	return nil
}

func (s *MemoryStorage) DeleteVDC(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.vdcs[id]; !exists {
		return ErrNotFound
	}

	delete(s.vdcs, id)
	return nil
}

// Template operations
func (s *MemoryStorage) ListTemplates() ([]*models.Template, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	templates := make([]*models.Template, 0, len(s.templates))
	for _, tmpl := range s.templates {
		templates = append(templates, tmpl)
	}
	return templates, nil
}

func (s *MemoryStorage) GetTemplate(id string) (*models.Template, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	tmpl, exists := s.templates[id]
	if !exists {
		return nil, ErrNotFound
	}
	return tmpl, nil
}

func (s *MemoryStorage) CreateTemplate(template *models.Template) error {
	if template == nil || template.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.templates[template.ID]; exists {
		return ErrAlreadyExists
	}

	template.CreatedAt = time.Now()
	template.UpdatedAt = template.CreatedAt
	s.templates[template.ID] = template
	return nil
}

func (s *MemoryStorage) UpdateTemplate(template *models.Template) error {
	if template == nil || template.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.templates[template.ID]; !exists {
		return ErrNotFound
	}

	template.UpdatedAt = time.Now()
	s.templates[template.ID] = template
	return nil
}

func (s *MemoryStorage) DeleteTemplate(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.templates[id]; !exists {
		return ErrNotFound
	}

	delete(s.templates, id)
	return nil
}

// VM operations
func (s *MemoryStorage) ListVMs(orgID string) ([]*models.VirtualMachine, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vms := make([]*models.VirtualMachine, 0)
	for _, vm := range s.vms {
		if orgID == "" || vm.OrgID == orgID {
			vms = append(vms, vm)
		}
	}
	return vms, nil
}

func (s *MemoryStorage) GetVM(id string) (*models.VirtualMachine, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vm, exists := s.vms[id]
	if !exists {
		return nil, ErrNotFound
	}
	return vm, nil
}

func (s *MemoryStorage) CreateVM(vm *models.VirtualMachine) error {
	if vm == nil || vm.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.vms[vm.ID]; exists {
		return ErrAlreadyExists
	}

	vm.CreatedAt = time.Now()
	vm.UpdatedAt = vm.CreatedAt
	s.vms[vm.ID] = vm
	return nil
}

func (s *MemoryStorage) UpdateVM(vm *models.VirtualMachine) error {
	if vm == nil || vm.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.vms[vm.ID]; !exists {
		return ErrNotFound
	}

	vm.UpdatedAt = time.Now()
	s.vms[vm.ID] = vm
	return nil
}

func (s *MemoryStorage) DeleteVM(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.vms[id]; !exists {
		return ErrNotFound
	}

	delete(s.vms, id)
	return nil
}

// Health operations
func (s *MemoryStorage) Ping() error {
	return nil
}

func (s *MemoryStorage) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Clear all data
	s.users = nil
	s.organizations = nil
	s.vdcs = nil
	s.templates = nil
	s.vms = nil

	klog.Info("Memory storage closed")
	return nil
}
