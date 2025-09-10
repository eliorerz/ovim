package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// User roles
const (
	RoleSystemAdmin = "system_admin"
	RoleOrgAdmin    = "org_admin"
	RoleOrgUser     = "org_user"
)

// VM statuses
const (
	VMStatusPending      = "pending"
	VMStatusProvisioning = "provisioning"
	VMStatusRunning      = "running"
	VMStatusStopped      = "stopped"
	VMStatusError        = "error"
	VMStatusDeleting     = "deleting"
)

// StringMap is a custom type that implements GORM interface for map[string]string
type StringMap map[string]string

// Scan implements the Scanner interface for database deserialization
func (sm *StringMap) Scan(value interface{}) error {
	if value == nil {
		*sm = make(StringMap)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into StringMap", value)
	}

	if len(bytes) == 0 {
		*sm = make(StringMap)
		return nil
	}

	var result map[string]string
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*sm = StringMap(result)
	return nil
}

// Value implements the driver Valuer interface for database serialization
func (sm StringMap) Value() (driver.Value, error) {
	if sm == nil {
		return nil, nil
	}
	return json.Marshal(map[string]string(sm))
}

// User represents a user in the system
type User struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex"`
	Email        string    `json:"email" gorm:"uniqueIndex"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	OrgID        *string   `json:"org_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Organization represents a tenant organization
type Organization struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Namespace   string    `json:"namespace" gorm:"uniqueIndex"`
	IsEnabled   bool      `json:"is_enabled" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VirtualDataCenter represents a virtual data center within an organization
type VirtualDataCenter struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	OrgID          string    `json:"org_id"`
	Namespace      string    `json:"namespace"`
	ResourceQuotas StringMap `json:"resource_quotas" gorm:"type:jsonb"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Template represents a VM template available in the catalog
type Template struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OSType      string    `json:"os_type"`
	OSVersion   string    `json:"os_version"`
	CPU         int       `json:"cpu"`
	Memory      string    `json:"memory"`
	DiskSize    string    `json:"disk_size"`
	ImageURL    string    `json:"image_url"`
	OrgID       string    `json:"org_id" gorm:"index"`
	Metadata    StringMap `json:"metadata" gorm:"type:jsonb"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VirtualMachine represents a deployed virtual machine
type VirtualMachine struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	Name       string    `json:"name"`
	OrgID      string    `json:"org_id"`
	VDCID      string    `json:"vdc_id"`
	TemplateID string    `json:"template_id"`
	OwnerID    string    `json:"owner_id"`
	Status     string    `json:"status"`
	CPU        int       `json:"cpu"`
	Memory     string    `json:"memory"`
	DiskSize   string    `json:"disk_size"`
	IPAddress  string    `json:"ip_address"`
	Metadata   StringMap `json:"metadata" gorm:"type:jsonb"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateOrganizationRequest represents a request to create an organization
type CreateOrganizationRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsEnabled   bool   `json:"is_enabled"`
}

// UpdateOrganizationRequest represents a request to update an organization
type UpdateOrganizationRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsEnabled   bool   `json:"is_enabled"`
}

// CreateVDCRequest represents a request to create a virtual data center
type CreateVDCRequest struct {
	Name           string            `json:"name" binding:"required"`
	Description    string            `json:"description"`
	OrgID          string            `json:"org_id" binding:"required"`
	ResourceQuotas map[string]string `json:"resource_quotas,omitempty"`
}

// UpdateVDCRequest represents a request to update a virtual data center
type UpdateVDCRequest struct {
	Name           string            `json:"name,omitempty"`
	Description    string            `json:"description,omitempty"`
	ResourceQuotas map[string]string `json:"resource_quotas,omitempty"`
}

// CreateVMRequest represents a request to create a virtual machine
type CreateVMRequest struct {
	Name       string `json:"name" binding:"required"`
	TemplateID string `json:"template_id" binding:"required"`
	CPU        int    `json:"cpu,omitempty"`
	Memory     string `json:"memory,omitempty"`
	DiskSize   string `json:"disk_size,omitempty"`
}

// UpdateVMPowerRequest represents a request to change VM power state
type UpdateVMPowerRequest struct {
	Action string `json:"action" binding:"required"` // "start", "stop", "restart"
}
