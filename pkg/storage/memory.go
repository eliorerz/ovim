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
	users             map[string]*models.User
	organizations     map[string]*models.Organization
	vdcs              map[string]*models.VirtualDataCenter
	templates         map[string]*models.Template
	vms               map[string]*models.VirtualMachine
	catalogSources    map[string]*models.OrganizationCatalogSource
	events            map[string]*models.Event
	eventCategories   map[string]*models.EventCategory
	retentionPolicies map[string]*models.EventRetentionPolicy
	zones             map[string]*models.Zone
	orgZoneQuotas     map[string]*models.OrganizationZoneQuota // key: orgID-zoneID
	mutex             sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage instance
func NewMemoryStorage() (Storage, error) {
	storage := &MemoryStorage{
		users:             make(map[string]*models.User),
		organizations:     make(map[string]*models.Organization),
		vdcs:              make(map[string]*models.VirtualDataCenter),
		templates:         make(map[string]*models.Template),
		vms:               make(map[string]*models.VirtualMachine),
		catalogSources:    make(map[string]*models.OrganizationCatalogSource),
		events:            make(map[string]*models.Event),
		eventCategories:   make(map[string]*models.EventCategory),
		retentionPolicies: make(map[string]*models.EventRetentionPolicy),
		zones:             make(map[string]*models.Zone),
		orgZoneQuotas:     make(map[string]*models.OrganizationZoneQuota),
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
	}

	for _, user := range users {
		s.users[user.ID] = user
	}

	// No seed organizations - start with empty list

	// No seed VDCs - start with empty list

	// No seed templates - start with empty list

	// No seed zones - zones will be dynamically created by ACM sync
	klog.Infof("Seeded storage with %d users, 0 organizations, 0 VDCs, 0 templates, 0 zones (zones will be synced from ACM)", len(users))

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

// Organization Catalog Source operations
func (s *MemoryStorage) ListOrganizationCatalogSources(orgID string) ([]*models.OrganizationCatalogSource, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sources := make([]*models.OrganizationCatalogSource, 0)
	for _, source := range s.catalogSources {
		if source.OrgID == orgID {
			sources = append(sources, source)
		}
	}
	return sources, nil
}

func (s *MemoryStorage) GetOrganizationCatalogSource(id string) (*models.OrganizationCatalogSource, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	source, exists := s.catalogSources[id]
	if !exists {
		return nil, ErrNotFound
	}
	return source, nil
}

func (s *MemoryStorage) CreateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error {
	if source == nil || source.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.catalogSources[source.ID]; exists {
		return ErrAlreadyExists
	}

	s.catalogSources[source.ID] = source
	return nil
}

func (s *MemoryStorage) UpdateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error {
	if source == nil || source.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.catalogSources[source.ID]; !exists {
		return ErrNotFound
	}

	s.catalogSources[source.ID] = source
	return nil
}

func (s *MemoryStorage) DeleteOrganizationCatalogSource(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.catalogSources[id]; !exists {
		return ErrNotFound
	}

	delete(s.catalogSources, id)
	return nil
}

// Event operations (in-memory implementation)

func (s *MemoryStorage) ListEvents(filter *models.EventFilter) (*models.EventsResponse, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var events []models.Event
	for _, event := range s.events {
		// Apply basic filters (simplified for in-memory storage)
		if filter != nil {
			if len(filter.Type) > 0 {
				found := false
				for _, t := range filter.Type {
					if event.Type == t {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			if filter.OrgID != "" && (event.OrgID == nil || *event.OrgID != filter.OrgID) {
				continue
			}

			if filter.VDCID != "" && (event.VDCID == nil || *event.VDCID != filter.VDCID) {
				continue
			}
		}

		events = append(events, *event)
	}

	// Apply pagination
	limit := 50
	page := 1
	if filter != nil {
		if filter.Limit > 0 {
			limit = filter.Limit
		}
		if filter.Page > 0 {
			page = filter.Page
		}
	}

	start := (page - 1) * limit
	end := start + limit
	if start >= len(events) {
		events = []models.Event{}
	} else if end > len(events) {
		events = events[start:]
	} else {
		events = events[start:end]
	}

	return &models.EventsResponse{
		Events:     events,
		TotalCount: int64(len(s.events)),
		Page:       page,
		PageSize:   limit,
		TotalPages: (len(s.events) + limit - 1) / limit,
	}, nil
}

func (s *MemoryStorage) GetEvent(id string) (*models.Event, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	event, exists := s.events[id]
	if !exists {
		return nil, ErrNotFound
	}
	return event, nil
}

func (s *MemoryStorage) CreateEvent(event *models.Event) error {
	if event == nil {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("event-%d", len(s.events)+1)
	}

	if _, exists := s.events[event.ID]; exists {
		return ErrAlreadyExists
	}

	event.CreatedAt = time.Now()
	event.UpdatedAt = event.CreatedAt
	s.events[event.ID] = event
	return nil
}

func (s *MemoryStorage) CreateEvents(events []*models.Event) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, event := range events {
		if event == nil {
			continue
		}

		if event.ID == "" {
			event.ID = fmt.Sprintf("event-%d", len(s.events)+1)
		}

		event.CreatedAt = time.Now()
		event.UpdatedAt = event.CreatedAt
		s.events[event.ID] = event
	}
	return nil
}

func (s *MemoryStorage) UpdateEvent(event *models.Event) error {
	if event == nil || event.ID == "" {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.events[event.ID]; !exists {
		return ErrNotFound
	}

	event.UpdatedAt = time.Now()
	s.events[event.ID] = event
	return nil
}

func (s *MemoryStorage) DeleteEvent(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.events[id]; !exists {
		return ErrNotFound
	}

	delete(s.events, id)
	return nil
}

func (s *MemoryStorage) CleanupOldEvents() (int, error) {
	// Simple cleanup for in-memory storage - remove events older than 30 days
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cutoff := time.Now().AddDate(0, 0, -30)
	deletedCount := 0

	for id, event := range s.events {
		if event.LastTimestamp.Before(cutoff) {
			delete(s.events, id)
			deletedCount++
		}
	}

	return deletedCount, nil
}

func (s *MemoryStorage) ListEventCategories() ([]*models.EventCategory, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var categories []*models.EventCategory
	for _, category := range s.eventCategories {
		categories = append(categories, category)
	}
	return categories, nil
}

func (s *MemoryStorage) GetEventCategory(name string) (*models.EventCategory, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	category, exists := s.eventCategories[name]
	if !exists {
		return nil, ErrNotFound
	}
	return category, nil
}

func (s *MemoryStorage) ListEventRetentionPolicies() ([]*models.EventRetentionPolicy, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var policies []*models.EventRetentionPolicy
	for _, policy := range s.retentionPolicies {
		policies = append(policies, policy)
	}
	return policies, nil
}

func (s *MemoryStorage) GetEventRetentionPolicy(category, eventType string) (*models.EventRetentionPolicy, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := fmt.Sprintf("%s-%s", category, eventType)
	policy, exists := s.retentionPolicies[key]
	if !exists {
		return nil, ErrNotFound
	}
	return policy, nil
}

func (s *MemoryStorage) UpdateEventRetentionPolicy(policy *models.EventRetentionPolicy) error {
	if policy == nil {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := fmt.Sprintf("%s-%s", policy.Category, policy.Type)
	policy.UpdatedAt = time.Now()
	s.retentionPolicies[key] = policy
	return nil
}

// NewMemoryStorageForTest creates a new in-memory storage instance for testing
// This version doesn't seed any initial data, providing a clean slate for tests
func NewMemoryStorageForTest() (Storage, error) {
	storage := &MemoryStorage{
		users:             make(map[string]*models.User),
		organizations:     make(map[string]*models.Organization),
		vdcs:              make(map[string]*models.VirtualDataCenter),
		templates:         make(map[string]*models.Template),
		vms:               make(map[string]*models.VirtualMachine),
		catalogSources:    make(map[string]*models.OrganizationCatalogSource),
		events:            make(map[string]*models.Event),
		eventCategories:   make(map[string]*models.EventCategory),
		retentionPolicies: make(map[string]*models.EventRetentionPolicy),
		zones:             make(map[string]*models.Zone),
		orgZoneQuotas:     make(map[string]*models.OrganizationZoneQuota),
	}

	klog.Info("Initialized in-memory storage for testing with clean state")
	return storage, nil
}

// Zone operations

func (s *MemoryStorage) ListZones() ([]*models.Zone, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	zones := make([]*models.Zone, 0, len(s.zones))
	for _, zone := range s.zones {
		zoneCopy := *zone
		zones = append(zones, &zoneCopy)
	}
	return zones, nil
}

func (s *MemoryStorage) GetZone(id string) (*models.Zone, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	zone, exists := s.zones[id]
	if !exists {
		return nil, ErrNotFound
	}
	zoneCopy := *zone
	return &zoneCopy, nil
}

func (s *MemoryStorage) CreateZone(zone *models.Zone) error {
	if zone == nil {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.zones[zone.ID]; exists {
		return ErrAlreadyExists
	}

	zone.CreatedAt = time.Now()
	zone.UpdatedAt = time.Now()
	zone.LastSync = time.Now()

	zoneCopy := *zone
	s.zones[zone.ID] = &zoneCopy
	return nil
}

func (s *MemoryStorage) UpdateZone(zone *models.Zone) error {
	if zone == nil {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.zones[zone.ID]; !exists {
		return ErrNotFound
	}

	zone.UpdatedAt = time.Now()
	zoneCopy := *zone
	s.zones[zone.ID] = &zoneCopy
	return nil
}

func (s *MemoryStorage) DeleteZone(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.zones[id]; !exists {
		return ErrNotFound
	}

	delete(s.zones, id)
	return nil
}

func (s *MemoryStorage) GetZoneUtilization() ([]*models.ZoneUtilization, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	utilization := make([]*models.ZoneUtilization, 0, len(s.zones))
	for _, zone := range s.zones {
		// Calculate basic utilization from VDCs in this zone
		var cpuUsed, memoryUsed, storageUsed int
		var vdcCount, activeVDCCount int

		for _, vdc := range s.vdcs {
			if vdc.ZoneID != nil && *vdc.ZoneID == zone.ID {
				vdcCount++
				if vdc.Phase == "Active" {
					activeVDCCount++
				}
				cpuUsed += vdc.CPUQuota
				memoryUsed += vdc.MemoryQuota
				storageUsed += vdc.StorageQuota
			}
		}

		util := &models.ZoneUtilization{
			ID:              zone.ID,
			Name:            zone.Name,
			Status:          zone.Status,
			CPUCapacity:     zone.CPUCapacity,
			MemoryCapacity:  zone.MemoryCapacity,
			StorageCapacity: zone.StorageCapacity,
			CPUQuota:        zone.CPUQuota,
			MemoryQuota:     zone.MemoryQuota,
			StorageQuota:    zone.StorageQuota,
			CPUUsed:         cpuUsed,
			MemoryUsed:      memoryUsed,
			StorageUsed:     storageUsed,
			VDCCount:        vdcCount,
			ActiveVDCCount:  activeVDCCount,
			LastSync:        zone.LastSync,
			UpdatedAt:       zone.UpdatedAt,
		}
		utilization = append(utilization, util)
	}
	return utilization, nil
}

// Organization Zone Quota operations

func (s *MemoryStorage) ListOrganizationZoneQuotas(orgID string) ([]*models.OrganizationZoneQuota, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	quotas := make([]*models.OrganizationZoneQuota, 0)
	for _, quota := range s.orgZoneQuotas {
		if orgID == "" || quota.OrganizationID == orgID {
			quotaCopy := *quota
			// Load the zone relationship
			if zone, exists := s.zones[quota.ZoneID]; exists {
				zoneCopy := *zone
				quotaCopy.Zone = &zoneCopy
			}
			quotas = append(quotas, &quotaCopy)
		}
	}
	return quotas, nil
}

func (s *MemoryStorage) GetOrganizationZoneQuota(orgID, zoneID string) (*models.OrganizationZoneQuota, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := fmt.Sprintf("%s-%s", orgID, zoneID)
	quota, exists := s.orgZoneQuotas[key]
	if !exists {
		return nil, ErrNotFound
	}

	quotaCopy := *quota
	// Load the zone relationship
	if zone, exists := s.zones[zoneID]; exists {
		zoneCopy := *zone
		quotaCopy.Zone = &zoneCopy
	}
	return &quotaCopy, nil
}

func (s *MemoryStorage) CreateOrganizationZoneQuota(quota *models.OrganizationZoneQuota) error {
	if quota == nil {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := fmt.Sprintf("%s-%s", quota.OrganizationID, quota.ZoneID)
	if _, exists := s.orgZoneQuotas[key]; exists {
		return ErrAlreadyExists
	}

	quota.CreatedAt = time.Now()
	quota.UpdatedAt = time.Now()

	quotaCopy := *quota
	s.orgZoneQuotas[key] = &quotaCopy
	return nil
}

func (s *MemoryStorage) UpdateOrganizationZoneQuota(quota *models.OrganizationZoneQuota) error {
	if quota == nil {
		return ErrInvalidInput
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := fmt.Sprintf("%s-%s", quota.OrganizationID, quota.ZoneID)
	if _, exists := s.orgZoneQuotas[key]; !exists {
		return ErrNotFound
	}

	quota.UpdatedAt = time.Now()
	quotaCopy := *quota
	s.orgZoneQuotas[key] = &quotaCopy
	return nil
}

func (s *MemoryStorage) DeleteOrganizationZoneQuota(orgID, zoneID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := fmt.Sprintf("%s-%s", orgID, zoneID)
	if _, exists := s.orgZoneQuotas[key]; !exists {
		return ErrNotFound
	}

	delete(s.orgZoneQuotas, key)
	return nil
}

func (s *MemoryStorage) GetOrganizationZoneAccess(orgID string) ([]*models.OrganizationZoneAccess, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	access := make([]*models.OrganizationZoneAccess, 0)
	for _, quota := range s.orgZoneQuotas {
		if orgID == "" || quota.OrganizationID == orgID {
			zone, exists := s.zones[quota.ZoneID]
			if !exists {
				continue
			}

			// Calculate usage for this org in this zone
			var cpuUsed, memoryUsed, storageUsed, vdcCount int
			for _, vdc := range s.vdcs {
				if vdc.ZoneID != nil && *vdc.ZoneID == quota.ZoneID && vdc.OrgID == quota.OrganizationID {
					vdcCount++
					cpuUsed += vdc.CPUQuota
					memoryUsed += vdc.MemoryQuota
					storageUsed += vdc.StorageQuota
				}
			}

			accessItem := &models.OrganizationZoneAccess{
				OrganizationID: quota.OrganizationID,
				ZoneID:         quota.ZoneID,
				ZoneName:       zone.Name,
				ZoneStatus:     zone.Status,
				CPUQuota:       quota.CPUQuota,
				MemoryQuota:    quota.MemoryQuota,
				StorageQuota:   quota.StorageQuota,
				IsAllowed:      quota.IsAllowed,
				CPUUsed:        cpuUsed,
				MemoryUsed:     memoryUsed,
				StorageUsed:    storageUsed,
				VDCCount:       vdcCount,
			}
			access = append(access, accessItem)
		}
	}
	return access, nil
}
