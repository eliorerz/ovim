package storage

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
)

// PostgresStorage implements the Storage interface using PostgreSQL with GORM
type PostgresStorage struct {
	db *gorm.DB
}

// NewPostgresStorage creates a new PostgreSQL storage instance
func NewPostgresStorage(dsn string) (Storage, error) {
	// Configure GORM logger to use klog
	gormLogger := logger.New(
		&klogWriter{},
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	storage := &PostgresStorage{db: db}

	// Run migrations
	if err := storage.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Seed initial data
	if err := storage.seedData(); err != nil {
		return nil, fmt.Errorf("failed to seed data: %w", err)
	}

	klog.Info("Initialized PostgreSQL storage with migrations and seed data")
	return storage, nil
}

// klogWriter implements the writer interface for GORM logger
type klogWriter struct{}

func (w *klogWriter) Printf(format string, args ...interface{}) {
	klog.V(4).Infof(format, args...)
}

// migrate runs database migrations
func (s *PostgresStorage) migrate() error {
	return s.db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.VirtualDataCenter{},
		&models.Template{},
		&models.VirtualMachine{},
		&models.OrganizationCatalogSource{},
		&models.Event{},
		&models.EventCategory{},
		&models.EventRetentionPolicy{},
		&models.Zone{},
		&models.OrganizationZoneQuota{},
	)
}

// seedData populates the database with initial test data if it's empty
func (s *PostgresStorage) seedData() error {
	now := time.Now()

	// Check if users already exist - seed users only if none exist
	var userCount int64
	if err := s.db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	var usersSeeded int
	if userCount == 0 {
		adminHash, err := auth.HashPassword("adminpassword")
		if err != nil {
			return fmt.Errorf("failed to hash admin password: %w", err)
		}

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
			if err := s.db.Create(user).Error; err != nil {
				return fmt.Errorf("failed to create user %s: %w", user.Username, err)
			}
		}
		usersSeeded = len(users)
	}

	// Check if zones already exist - seed zones if none exist
	var zoneCount int64
	if err := s.db.Model(&models.Zone{}).Count(&zoneCount).Error; err != nil {
		return fmt.Errorf("failed to count zones: %w", err)
	}

	// No seed zones - zones will be dynamically created by ACM sync
	var zonesSeeded int

	if usersSeeded > 0 || zonesSeeded > 0 {
		klog.Infof("Seeded database with %d users, %d zones", usersSeeded, zonesSeeded)
	} else {
		klog.Info("Database already contains data, skipping seeding")
	}

	return nil
}

// User operations
func (s *PostgresStorage) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := s.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStorage) GetUserByID(id string) (*models.User, error) {
	var user models.User
	err := s.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStorage) CreateUser(user *models.User) error {
	if user == nil || user.ID == "" {
		return ErrInvalidInput
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = user.CreatedAt

	err := s.db.Create(user).Error
	if err != nil {
		// Check for unique constraint violation
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStorage) UpdateUser(user *models.User) error {
	if user == nil || user.ID == "" {
		return ErrInvalidInput
	}

	user.UpdatedAt = time.Now()
	result := s.db.Save(user)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) DeleteUser(id string) error {
	result := s.db.Delete(&models.User{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) ListUsers() ([]*models.User, error) {
	var users []*models.User
	err := s.db.Find(&users).Error
	return users, err
}

func (s *PostgresStorage) ListUsersByOrg(orgID string) ([]*models.User, error) {
	var users []*models.User
	err := s.db.Where("org_id = ?", orgID).Find(&users).Error
	return users, err
}

// Organization operations
func (s *PostgresStorage) ListOrganizations() ([]*models.Organization, error) {
	var orgs []*models.Organization
	err := s.db.Find(&orgs).Error
	return orgs, err
}

func (s *PostgresStorage) GetOrganization(id string) (*models.Organization, error) {
	var org models.Organization
	err := s.db.Where("id = ?", id).First(&org).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &org, nil
}

func (s *PostgresStorage) CreateOrganization(org *models.Organization) error {
	if org == nil || org.ID == "" {
		return ErrInvalidInput
	}

	org.CreatedAt = time.Now()
	org.UpdatedAt = org.CreatedAt

	err := s.db.Create(org).Error
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return err
	}

	return nil
}

func (s *PostgresStorage) UpdateOrganization(org *models.Organization) error {
	if org == nil || org.ID == "" {
		return ErrInvalidInput
	}

	org.UpdatedAt = time.Now()
	result := s.db.Save(org)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) DeleteOrganization(id string) error {
	result := s.db.Delete(&models.Organization{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// VDC operations
func (s *PostgresStorage) ListVDCs(orgID string) ([]*models.VirtualDataCenter, error) {
	var vdcs []*models.VirtualDataCenter
	query := s.db
	if orgID != "" {
		query = query.Where("org_id = ?", orgID)
	}
	err := query.Find(&vdcs).Error
	return vdcs, err
}

func (s *PostgresStorage) GetVDC(id string) (*models.VirtualDataCenter, error) {
	var vdc models.VirtualDataCenter
	err := s.db.Where("id = ?", id).First(&vdc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &vdc, nil
}

func (s *PostgresStorage) CreateVDC(vdc *models.VirtualDataCenter) error {
	if vdc == nil || vdc.ID == "" {
		return ErrInvalidInput
	}

	vdc.CreatedAt = time.Now()
	vdc.UpdatedAt = vdc.CreatedAt

	err := s.db.Create(vdc).Error
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStorage) UpdateVDC(vdc *models.VirtualDataCenter) error {
	if vdc == nil || vdc.ID == "" {
		return ErrInvalidInput
	}

	vdc.UpdatedAt = time.Now()
	result := s.db.Save(vdc)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) DeleteVDC(id string) error {
	result := s.db.Delete(&models.VirtualDataCenter{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Template operations
func (s *PostgresStorage) ListTemplates() ([]*models.Template, error) {
	var templates []*models.Template
	err := s.db.Find(&templates).Error
	return templates, err
}

func (s *PostgresStorage) ListTemplatesByOrg(orgID string) ([]*models.Template, error) {
	var templates []*models.Template
	err := s.db.Where("org_id = ?", orgID).Find(&templates).Error
	return templates, err
}

func (s *PostgresStorage) GetTemplate(id string) (*models.Template, error) {
	var template models.Template
	err := s.db.Where("id = ?", id).First(&template).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &template, nil
}

func (s *PostgresStorage) CreateTemplate(template *models.Template) error {
	if template == nil || template.ID == "" {
		return ErrInvalidInput
	}

	template.CreatedAt = time.Now()
	template.UpdatedAt = template.CreatedAt

	err := s.db.Create(template).Error
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStorage) UpdateTemplate(template *models.Template) error {
	if template == nil || template.ID == "" {
		return ErrInvalidInput
	}

	template.UpdatedAt = time.Now()
	result := s.db.Save(template)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) DeleteTemplate(id string) error {
	result := s.db.Delete(&models.Template{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// VM operations
func (s *PostgresStorage) ListVMs(orgID string) ([]*models.VirtualMachine, error) {
	var vms []*models.VirtualMachine
	query := s.db
	if orgID != "" {
		query = query.Where("org_id = ?", orgID)
	}
	err := query.Find(&vms).Error
	return vms, err
}

func (s *PostgresStorage) GetVM(id string) (*models.VirtualMachine, error) {
	var vm models.VirtualMachine
	err := s.db.Where("id = ?", id).First(&vm).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &vm, nil
}

func (s *PostgresStorage) CreateVM(vm *models.VirtualMachine) error {
	if vm == nil || vm.ID == "" {
		return ErrInvalidInput
	}

	vm.CreatedAt = time.Now()
	vm.UpdatedAt = vm.CreatedAt

	err := s.db.Create(vm).Error
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStorage) UpdateVM(vm *models.VirtualMachine) error {
	if vm == nil || vm.ID == "" {
		return ErrInvalidInput
	}

	vm.UpdatedAt = time.Now()
	result := s.db.Save(vm)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) DeleteVM(id string) error {
	result := s.db.Delete(&models.VirtualMachine{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Health operations
func (s *PostgresStorage) Ping() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (s *PostgresStorage) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	klog.Info("Closing PostgreSQL storage connection")
	return sqlDB.Close()
}

// Helper function to check for duplicate key errors
func isDuplicateKeyError(err error) bool {
	// PostgreSQL error codes for unique violation
	return err != nil && (
	// Check for common PostgreSQL unique constraint violation patterns
	contains(err.Error(), "duplicate key") ||
		contains(err.Error(), "unique constraint") ||
		contains(err.Error(), "UNIQUE constraint"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			indexOfSubstring(s, substr) >= 0))
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// NewPostgresStorageForTest creates a PostgreSQL storage instance for testing
// This version clears all data and doesn't seed initial data, providing a clean slate for tests
func NewPostgresStorageForTest(dsn string) (Storage, error) {
	// Configure GORM logger to use klog
	gormLogger := logger.New(
		&klogWriter{},
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	storage := &PostgresStorage{db: db}

	// Clear all existing data for a clean test environment
	if err := storage.clearAllData(); err != nil {
		return nil, fmt.Errorf("failed to clear test data: %w", err)
	}

	// Run migrations
	if err := storage.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	klog.Info("Initialized PostgreSQL storage for testing with clean database")
	return storage, nil
}

// clearAllData removes all data from the database for testing
func (s *PostgresStorage) clearAllData() error {
	// Delete all data in reverse order to respect foreign key constraints
	tables := []string{
		"events",
		"virtual_machines",
		"templates",
		"organization_catalog_sources",
		"virtual_data_centers",
		"organizations",
		"users",
		"event_retention_policies",
		"event_categories",
	}

	for _, table := range tables {
		if err := s.db.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
	}

	klog.Info("Cleared all test data from PostgreSQL database")
	return nil
}

// Organization Catalog Source operations

func (s *PostgresStorage) ListOrganizationCatalogSources(orgID string) ([]*models.OrganizationCatalogSource, error) {
	var sources []*models.OrganizationCatalogSource
	if err := s.db.Where("org_id = ?", orgID).Find(&sources).Error; err != nil {
		return nil, fmt.Errorf("failed to list organization catalog sources: %w", err)
	}
	return sources, nil
}

func (s *PostgresStorage) GetOrganizationCatalogSource(id string) (*models.OrganizationCatalogSource, error) {
	var source models.OrganizationCatalogSource
	if err := s.db.First(&source, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get organization catalog source: %w", err)
	}
	return &source, nil
}

func (s *PostgresStorage) CreateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error {
	if err := s.db.Create(source).Error; err != nil {
		return fmt.Errorf("failed to create organization catalog source: %w", err)
	}
	return nil
}

func (s *PostgresStorage) UpdateOrganizationCatalogSource(source *models.OrganizationCatalogSource) error {
	if err := s.db.Save(source).Error; err != nil {
		return fmt.Errorf("failed to update organization catalog source: %w", err)
	}
	return nil
}

func (s *PostgresStorage) DeleteOrganizationCatalogSource(id string) error {
	if err := s.db.Delete(&models.OrganizationCatalogSource{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete organization catalog source: %w", err)
	}
	return nil
}

// Event operations

func (s *PostgresStorage) ListEvents(filter *models.EventFilter) (*models.EventsResponse, error) {
	query := s.db.Model(&models.Event{})

	// Apply filters
	if filter != nil {
		// Soft delete filter
		if !filter.IncludeDeleted {
			query = query.Where("deleted_at IS NULL")
		}

		// Type filter
		if len(filter.Type) > 0 {
			query = query.Where("type IN ?", filter.Type)
		}

		// Category filter
		if len(filter.Category) > 0 {
			query = query.Where("category IN ?", filter.Category)
		}

		// Reason filter
		if len(filter.Reason) > 0 {
			query = query.Where("reason IN ?", filter.Reason)
		}

		// Component filter
		if len(filter.Component) > 0 {
			query = query.Where("component IN ?", filter.Component)
		}

		// Namespace filter
		if len(filter.Namespace) > 0 {
			query = query.Where("namespace IN ?", filter.Namespace)
		}

		// Context filters
		if filter.OrgID != "" {
			query = query.Where("org_id = ?", filter.OrgID)
		}
		if filter.VDCID != "" {
			query = query.Where("vdc_id = ?", filter.VDCID)
		}
		if filter.VMID != "" {
			query = query.Where("vm_id = ?", filter.VMID)
		}
		if filter.UserID != "" {
			query = query.Where("user_id = ?", filter.UserID)
		}
		if filter.Username != "" {
			query = query.Where("username ILIKE ?", "%"+filter.Username+"%")
		}

		// Full-text search
		if filter.Search != "" {
			query = query.Where("to_tsvector('english', message) @@ plainto_tsquery('english', ?)", filter.Search)
		}

		// Time range filters
		if filter.Since != "" {
			query = query.Where("last_timestamp >= ?", filter.Since)
		}
		if filter.Until != "" {
			query = query.Where("last_timestamp <= ?", filter.Until)
		}

		// Sorting
		sortBy := "last_timestamp"
		if filter.SortBy != "" {
			// Validate sort field
			validSortFields := map[string]bool{
				"last_timestamp": true, "first_timestamp": true, "event_time": true,
				"created_at": true, "type": true, "category": true, "reason": true,
				"component": true, "count": true,
			}
			if validSortFields[filter.SortBy] {
				sortBy = filter.SortBy
			}
		}

		sortOrder := "DESC"
		if filter.SortOrder == "asc" {
			sortOrder = "ASC"
		}

		query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))
	} else {
		// Default: exclude deleted, sort by last_timestamp DESC
		query = query.Where("deleted_at IS NULL").Order("last_timestamp DESC")
	}

	// Count total records
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}

	// Apply pagination
	limit := 50 // default
	page := 1   // default
	if filter != nil {
		if filter.Limit > 0 && filter.Limit <= 200 {
			limit = filter.Limit
		}
		if filter.Page > 0 {
			page = filter.Page
		}
	}

	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)

	// Execute query
	var events []models.Event
	if err := query.Find(&events).Error; err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	// Calculate total pages
	totalPages := int((totalCount + int64(limit) - 1) / int64(limit))

	return &models.EventsResponse{
		Events:     events,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	}, nil
}

func (s *PostgresStorage) GetEvent(id string) (*models.Event, error) {
	var event models.Event
	err := s.db.Where("id = ?", id).First(&event).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	return &event, nil
}

func (s *PostgresStorage) CreateEvent(event *models.Event) error {
	if event == nil {
		return ErrInvalidInput
	}

	// Set defaults
	if event.Type == "" {
		event.Type = models.EventTypeNormal
	}
	if event.Category == "" {
		event.Category = "General"
	}
	if event.Count == 0 {
		event.Count = 1
	}
	if event.FirstTimestamp.IsZero() {
		event.FirstTimestamp = time.Now()
	}
	if event.LastTimestamp.IsZero() {
		event.LastTimestamp = event.FirstTimestamp
	}
	if event.EventTime.IsZero() {
		event.EventTime = event.FirstTimestamp
	}

	// Try to find existing event with same name and deduplicate
	if event.Name != "" {
		var existingEvent models.Event
		err := s.db.Where("name = ? AND deleted_at IS NULL", event.Name).First(&existingEvent).Error
		if err == nil {
			// Event exists, increment count and update timestamp
			existingEvent.Count++
			existingEvent.LastTimestamp = time.Now()
			existingEvent.Message = event.Message // Update message to latest
			existingEvent.UpdatedAt = time.Now()

			// Update metadata if provided
			if len(event.Metadata) > 0 {
				existingEvent.Metadata = event.Metadata
			}

			return s.db.Save(&existingEvent).Error
		} else if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to check for existing event: %w", err)
		}
	}

	// Create new event
	event.CreatedAt = time.Now()
	event.UpdatedAt = event.CreatedAt

	err := s.db.Create(event).Error
	if err != nil {
		if isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create event: %w", err)
	}
	return nil
}

func (s *PostgresStorage) CreateEvents(events []*models.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Process events in batch
	now := time.Now()
	for _, event := range events {
		if event == nil {
			continue
		}

		// Set defaults
		if event.Type == "" {
			event.Type = models.EventTypeNormal
		}
		if event.Category == "" {
			event.Category = "General"
		}
		if event.Count == 0 {
			event.Count = 1
		}
		if event.FirstTimestamp.IsZero() {
			event.FirstTimestamp = now
		}
		if event.LastTimestamp.IsZero() {
			event.LastTimestamp = event.FirstTimestamp
		}
		if event.EventTime.IsZero() {
			event.EventTime = event.FirstTimestamp
		}

		event.CreatedAt = now
		event.UpdatedAt = now
	}

	// Use transaction for batch insert
	return s.db.Transaction(func(tx *gorm.DB) error {
		return tx.CreateInBatches(events, 100).Error
	})
}

func (s *PostgresStorage) UpdateEvent(event *models.Event) error {
	if event == nil || event.ID == "" {
		return ErrInvalidInput
	}

	event.UpdatedAt = time.Now()
	result := s.db.Save(event)
	if result.Error != nil {
		return fmt.Errorf("failed to update event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) DeleteEvent(id string) error {
	// Soft delete
	result := s.db.Model(&models.Event{}).Where("id = ?", id).Update("deleted_at", time.Now())
	if result.Error != nil {
		return fmt.Errorf("failed to delete event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) CleanupOldEvents() (int, error) {
	// Call the PostgreSQL cleanup function
	var deletedCount int
	err := s.db.Raw("SELECT cleanup_old_events()").Scan(&deletedCount).Error
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old events: %w", err)
	}
	return deletedCount, nil
}

// Event category operations

func (s *PostgresStorage) ListEventCategories() ([]*models.EventCategory, error) {
	var categories []*models.EventCategory
	err := s.db.Find(&categories).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list event categories: %w", err)
	}
	return categories, nil
}

func (s *PostgresStorage) GetEventCategory(name string) (*models.EventCategory, error) {
	var category models.EventCategory
	err := s.db.Where("name = ?", name).First(&category).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get event category: %w", err)
	}
	return &category, nil
}

// Event retention policy operations

func (s *PostgresStorage) ListEventRetentionPolicies() ([]*models.EventRetentionPolicy, error) {
	var policies []*models.EventRetentionPolicy
	err := s.db.Find(&policies).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list event retention policies: %w", err)
	}
	return policies, nil
}

func (s *PostgresStorage) GetEventRetentionPolicy(category, eventType string) (*models.EventRetentionPolicy, error) {
	var policy models.EventRetentionPolicy
	err := s.db.Where("category = ? AND type = ?", category, eventType).First(&policy).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get event retention policy: %w", err)
	}
	return &policy, nil
}

func (s *PostgresStorage) UpdateEventRetentionPolicy(policy *models.EventRetentionPolicy) error {
	if policy == nil {
		return ErrInvalidInput
	}

	policy.UpdatedAt = time.Now()
	result := s.db.Save(policy)
	if result.Error != nil {
		return fmt.Errorf("failed to update event retention policy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Zone operations

func (s *PostgresStorage) ListZones() ([]*models.Zone, error) {
	var zones []*models.Zone
	err := s.db.Find(&zones).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}
	return zones, nil
}

func (s *PostgresStorage) GetZone(id string) (*models.Zone, error) {
	var zone models.Zone
	err := s.db.Where("id = ?", id).First(&zone).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get zone: %w", err)
	}
	return &zone, nil
}

func (s *PostgresStorage) CreateZone(zone *models.Zone) error {
	if zone == nil {
		return ErrInvalidInput
	}

	zone.CreatedAt = time.Now()
	zone.UpdatedAt = time.Now()
	zone.LastSync = time.Now()

	err := s.db.Create(zone).Error
	if err != nil {
		return fmt.Errorf("failed to create zone: %w", err)
	}
	return nil
}

func (s *PostgresStorage) UpdateZone(zone *models.Zone) error {
	if zone == nil {
		return ErrInvalidInput
	}

	zone.UpdatedAt = time.Now()
	result := s.db.Save(zone)
	if result.Error != nil {
		return fmt.Errorf("failed to update zone: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) DeleteZone(id string) error {
	result := s.db.Delete(&models.Zone{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete zone: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) GetZoneUtilization() ([]*models.ZoneUtilization, error) {
	var utilization []*models.ZoneUtilization
	err := s.db.Table("zone_utilization").Find(&utilization).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get zone utilization: %w", err)
	}
	return utilization, nil
}

// Organization Zone Quota operations

func (s *PostgresStorage) ListOrganizationZoneQuotas(orgID string) ([]*models.OrganizationZoneQuota, error) {
	var quotas []*models.OrganizationZoneQuota
	query := s.db.Preload("Zone")
	if orgID != "" {
		query = query.Where("organization_id = ?", orgID)
	}
	err := query.Find(&quotas).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list organization zone quotas: %w", err)
	}
	return quotas, nil
}

func (s *PostgresStorage) GetOrganizationZoneQuota(orgID, zoneID string) (*models.OrganizationZoneQuota, error) {
	var quota models.OrganizationZoneQuota
	err := s.db.Preload("Zone").Where("organization_id = ? AND zone_id = ?", orgID, zoneID).First(&quota).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get organization zone quota: %w", err)
	}
	return &quota, nil
}

func (s *PostgresStorage) CreateOrganizationZoneQuota(quota *models.OrganizationZoneQuota) error {
	if quota == nil {
		return ErrInvalidInput
	}

	quota.CreatedAt = time.Now()
	quota.UpdatedAt = time.Now()

	err := s.db.Create(quota).Error
	if err != nil {
		return fmt.Errorf("failed to create organization zone quota: %w", err)
	}
	return nil
}

func (s *PostgresStorage) UpdateOrganizationZoneQuota(quota *models.OrganizationZoneQuota) error {
	if quota == nil {
		return ErrInvalidInput
	}

	quota.UpdatedAt = time.Now()
	result := s.db.Save(quota)
	if result.Error != nil {
		return fmt.Errorf("failed to update organization zone quota: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) DeleteOrganizationZoneQuota(orgID, zoneID string) error {
	result := s.db.Delete(&models.OrganizationZoneQuota{}, "organization_id = ? AND zone_id = ?", orgID, zoneID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete organization zone quota: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStorage) GetOrganizationZoneAccess(orgID string) ([]*models.OrganizationZoneAccess, error) {
	var access []*models.OrganizationZoneAccess
	query := s.db.Table("organization_zone_access")
	if orgID != "" {
		query = query.Where("organization_id = ?", orgID)
	}
	err := query.Find(&access).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get organization zone access: %w", err)
	}
	return access, nil
}
