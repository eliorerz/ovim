package storage

import (
	"errors"
	"sync"
	"time"

	"github.com/eliorerz/ovim-updated/auth"
	"github.com/eliorerz/ovim-updated/models"
)

type MemoryStorage struct {
	users         map[string]*models.User
	organizations map[string]*models.Organization
	vdcs          map[string]*models.VirtualDataCenter
	templates     map[string]*models.Template
	vms           map[string]*models.VirtualMachine
	mutex         sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	storage := &MemoryStorage{
		users:         make(map[string]*models.User),
		organizations: make(map[string]*models.Organization),
		vdcs:          make(map[string]*models.VirtualDataCenter),
		templates:     make(map[string]*models.Template),
		vms:           make(map[string]*models.VirtualMachine),
	}

	storage.seedData()
	return storage
}

func (s *MemoryStorage) seedData() {
	adminHash, _ := auth.HashPassword("admin")
	userHash, _ := auth.HashPassword("user")

	now := time.Now()

	s.users["1"] = &models.User{
		ID:           "1",
		Username:     "admin",
		Email:        "admin@ovim.local",
		PasswordHash: adminHash,
		Role:         models.RoleSystemAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.users["2"] = &models.User{
		ID:           "2",
		Username:     "orgadmin",
		Email:        "orgadmin@acme.com",
		PasswordHash: adminHash,
		Role:         models.RoleOrgAdmin,
		OrgID:        stringPtr("org-1"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.users["3"] = &models.User{
		ID:           "3",
		Username:     "user",
		Email:        "user@acme.com",
		PasswordHash: userHash,
		Role:         models.RoleOrgUser,
		OrgID:        stringPtr("org-1"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.organizations["org-1"] = &models.Organization{
		ID:          "org-1",
		Name:        "Acme Corporation",
		Description: "Main corporate organization",
		Namespace:   "acme-corp",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.vdcs["vdc-1"] = &models.VirtualDataCenter{
		ID:             "vdc-1",
		Name:           "Acme VDC",
		Description:    "Main virtual data center for Acme Corp",
		OrgID:          "org-1",
		Namespace:      "acme-corp",
		ResourceQuotas: map[string]string{"cpu": "20", "memory": "64Gi", "storage": "1Ti"},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.templates["tmpl-1"] = &models.Template{
		ID:          "tmpl-1",
		Name:        "Red Hat Enterprise Linux 9.2",
		Description: "Latest RHEL 9.2 with security updates",
		OSType:      "Linux",
		OSVersion:   "RHEL 9.2",
		CPU:         2,
		Memory:      "4Gi",
		DiskSize:    "20Gi",
		ImageURL:    "registry.redhat.io/rhel9/rhel:latest",
		Metadata:    map[string]string{"vendor": "Red Hat", "certified": "true"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.templates["tmpl-2"] = &models.Template{
		ID:          "tmpl-2",
		Name:        "Ubuntu Server 22.04 LTS",
		Description: "Ubuntu Server 22.04 LTS with cloud-init",
		OSType:      "Linux",
		OSVersion:   "Ubuntu 22.04",
		CPU:         2,
		Memory:      "2Gi",
		DiskSize:    "20Gi",
		ImageURL:    "registry.ubuntu.com/ubuntu:22.04",
		Metadata:    map[string]string{"vendor": "Canonical", "lts": "true"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func stringPtr(s string) *string {
	return &s
}

func (s *MemoryStorage) GetUserByUsername(username string) (*models.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, user := range s.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

func (s *MemoryStorage) GetUserByID(id string) (*models.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

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
		return nil, errors.New("organization not found")
	}
	return org, nil
}

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
		return nil, errors.New("template not found")
	}
	return tmpl, nil
}

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
		return nil, errors.New("virtual machine not found")
	}
	return vm, nil
}
