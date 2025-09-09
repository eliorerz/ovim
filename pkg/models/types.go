package models

import (
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

// User represents a user in the system
type User struct {
	ID           string    `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"`
	OrgID        *string   `json:"org_id,omitempty" db:"org_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Organization represents a tenant organization
type Organization struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Namespace   string    `json:"namespace" db:"namespace"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// VirtualDataCenter represents a virtual data center within an organization
type VirtualDataCenter struct {
	ID             string            `json:"id" db:"id"`
	Name           string            `json:"name" db:"name"`
	Description    string            `json:"description" db:"description"`
	OrgID          string            `json:"org_id" db:"org_id"`
	Namespace      string            `json:"namespace" db:"namespace"`
	ResourceQuotas map[string]string `json:"resource_quotas" db:"resource_quotas"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" db:"updated_at"`
}

// Template represents a VM template available in the catalog
type Template struct {
	ID          string            `json:"id" db:"id"`
	Name        string            `json:"name" db:"name"`
	Description string            `json:"description" db:"description"`
	OSType      string            `json:"os_type" db:"os_type"`
	OSVersion   string            `json:"os_version" db:"os_version"`
	CPU         int               `json:"cpu" db:"cpu"`
	Memory      string            `json:"memory" db:"memory"`
	DiskSize    string            `json:"disk_size" db:"disk_size"`
	ImageURL    string            `json:"image_url" db:"image_url"`
	Metadata    map[string]string `json:"metadata" db:"metadata"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" db:"updated_at"`
}

// VirtualMachine represents a deployed virtual machine
type VirtualMachine struct {
	ID         string            `json:"id" db:"id"`
	Name       string            `json:"name" db:"name"`
	OrgID      string            `json:"org_id" db:"org_id"`
	VDCID      string            `json:"vdc_id" db:"vdc_id"`
	TemplateID string            `json:"template_id" db:"template_id"`
	OwnerID    string            `json:"owner_id" db:"owner_id"`
	Status     string            `json:"status" db:"status"`
	CPU        int               `json:"cpu" db:"cpu"`
	Memory     string            `json:"memory" db:"memory"`
	DiskSize   string            `json:"disk_size" db:"disk_size"`
	IPAddress  string            `json:"ip_address" db:"ip_address"`
	Metadata   map[string]string `json:"metadata" db:"metadata"`
	CreatedAt  time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at" db:"updated_at"`
}

// CreateOrganizationRequest represents a request to create an organization
type CreateOrganizationRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
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
