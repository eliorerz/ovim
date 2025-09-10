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
	)
}

// seedData populates the database with initial test data if it's empty
func (s *PostgresStorage) seedData() error {
	// Check if users already exist
	var userCount int64
	if err := s.db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if userCount > 0 {
		klog.Info("Database already contains data, skipping seeding")
		return nil
	}

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
		if err := s.db.Create(user).Error; err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Username, err)
		}
	}

	// No seed organizations - start with empty list

	// No seed VDCs - start with empty list

	// No seed templates - start with empty list

	klog.Infof("Seeded database with %d users, 0 organizations, 0 VDCs, 0 templates", len(users))

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
		"virtual_machines",
		"templates",
		"virtual_data_centers",
		"organizations",
		"users",
	}

	for _, table := range tables {
		if err := s.db.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
	}

	klog.Info("Cleared all test data from PostgreSQL database")
	return nil
}
