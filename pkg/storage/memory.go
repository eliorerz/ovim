package storage

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
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

	// Create a test organization for org users
	testOrg := &models.Organization{
		ID:          "org-test",
		Name:        "Test Organization",
		Description: "Test organization for development and testing",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Seed organizations
	s.organizations[testOrg.ID] = testOrg

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
			Email:        "orgadmin@ovim.local",
			PasswordHash: adminHash,
			Role:         models.RoleOrgAdmin,
			OrgID:        &testOrg.ID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "user-user",
			Username:     "user",
			Email:        "user@ovim.local",
			PasswordHash: userHash,
			Role:         models.RoleOrgUser,
			OrgID:        &testOrg.ID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	for _, user := range users {
		s.users[user.ID] = user
	}

	// Organization seeded above

	// No seed VDCs - start with empty list

	// No seed templates - start with empty list

	klog.Infof("Seeded storage with %d users, 1 organizations, 0 VDCs, 0 templates", len(users))

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

func (s *MemoryStorage) ListUsers() ([]*models.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	users := make([]*models.User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users, nil
}

func (s *MemoryStorage) ListUsersByOrg(orgID string) ([]*models.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	users := make([]*models.User, 0)
	for _, user := range s.users {
		if user.OrgID != nil && *user.OrgID == orgID {
			users = append(users, user)
		}
	}
	return users, nil
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

func (s *MemoryStorage) ListTemplatesByOrg(orgID string) ([]*models.Template, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	templates := make([]*models.Template, 0)
	for _, tmpl := range s.templates {
		if tmpl.OrgID == orgID {
			templates = append(templates, tmpl)
		}
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

// NewMemoryStorageForTest creates a new in-memory storage instance for testing
// This version doesn't seed any initial data, providing a clean slate for tests
func NewMemoryStorageForTest() (Storage, error) {
	storage := &MemoryStorage{
		users:         make(map[string]*models.User),
		organizations: make(map[string]*models.Organization),
		vdcs:          make(map[string]*models.VirtualDataCenter),
		templates:     make(map[string]*models.Template),
		vms:           make(map[string]*models.VirtualMachine),
	}

	klog.Info("Initialized in-memory storage for testing with clean state")
	return storage, nil
}
