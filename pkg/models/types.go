package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Namespace   string `json:"namespace" gorm:"uniqueIndex"`
	IsEnabled   bool   `json:"is_enabled" gorm:"default:true"`

	// Resource Quotas - organization-level resource limits
	CPUQuota     int `json:"cpu_quota" gorm:"default:0"`     // Total CPU cores allocated to organization
	MemoryQuota  int `json:"memory_quota" gorm:"default:0"`  // Total memory in GB allocated to organization
	StorageQuota int `json:"storage_quota" gorm:"default:0"` // Total storage in GB allocated to organization

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OrganizationResourceUsage represents current resource usage for an organization
type OrganizationResourceUsage struct {
	CPUUsed     int `json:"cpu_used"`
	MemoryUsed  int `json:"memory_used"`
	StorageUsed int `json:"storage_used"`

	CPUQuota     int `json:"cpu_quota"`
	MemoryQuota  int `json:"memory_quota"`
	StorageQuota int `json:"storage_quota"`

	CPUAvailable     int `json:"cpu_available"`
	MemoryAvailable  int `json:"memory_available"`
	StorageAvailable int `json:"storage_available"`
}

// GetResourceUsage calculates current resource usage for the organization
func (o *Organization) GetResourceUsage(vdcs []*VirtualDataCenter) OrganizationResourceUsage {
	var cpuUsed, memoryUsed, storageUsed int

	for _, vdc := range vdcs {
		if vdc.ResourceQuotas != nil {
			if cpuStr, ok := vdc.ResourceQuotas["cpu"]; ok {
				// Parse CPU string (e.g., "4" or "4 cores")
				cpuParsed := ParseCPUString(cpuStr)
				cpuUsed += cpuParsed
			}
			if memStr, ok := vdc.ResourceQuotas["memory"]; ok {
				// Parse memory string (e.g., "8Gi", "8GB")
				memParsed := ParseMemoryString(memStr)
				memoryUsed += memParsed
			}
			if storStr, ok := vdc.ResourceQuotas["storage"]; ok {
				// Parse storage string (e.g., "100Gi", "100GB")
				storParsed := ParseStorageString(storStr)
				storageUsed += storParsed
			}
		}
	}

	return OrganizationResourceUsage{
		CPUUsed:     cpuUsed,
		MemoryUsed:  memoryUsed,
		StorageUsed: storageUsed,

		CPUQuota:     o.CPUQuota,
		MemoryQuota:  o.MemoryQuota,
		StorageQuota: o.StorageQuota,

		CPUAvailable:     max(0, o.CPUQuota-cpuUsed),
		MemoryAvailable:  max(0, o.MemoryQuota-memoryUsed),
		StorageAvailable: max(0, o.StorageQuota-storageUsed),
	}
}

// CanAllocateResources checks if the organization can allocate the requested resources
func (o *Organization) CanAllocateResources(cpuReq, memoryReq, storageReq int, vdcs []*VirtualDataCenter) bool {
	usage := o.GetResourceUsage(vdcs)
	return usage.CPUAvailable >= cpuReq &&
		usage.MemoryAvailable >= memoryReq &&
		usage.StorageAvailable >= storageReq
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

// Template source types
const (
	TemplateSourceGlobal       = "global"
	TemplateSourceOrganization = "organization"
	TemplateSourceExternal     = "external"
)

// Template categories
const (
	TemplateCategoryOS          = "Operating System"
	TemplateCategoryDatabase    = "Database"
	TemplateCategoryApplication = "Application"
	TemplateCategoryMiddleware  = "Middleware"
	TemplateCategoryOther       = "Other"
)

// Template represents a VM template available in the catalog
type Template struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	OSType       string    `json:"os_type"`
	OSVersion    string    `json:"os_version"`
	CPU          int       `json:"cpu"`
	Memory       string    `json:"memory"`
	DiskSize     string    `json:"disk_size"`
	ImageURL     string    `json:"image_url"`
	IconClass    string    `json:"icon_class"`
	OrgID        string    `json:"org_id" gorm:"index"`
	Source       string    `json:"source" gorm:"default:'global'"`         // global, organization, external
	SourceVendor string    `json:"source_vendor" gorm:"default:'Red Hat'"` // Red Hat, Organization, Community, etc.
	Category     string    `json:"category" gorm:"default:'Operating System'"`
	Namespace    string    `json:"namespace"` // OpenShift namespace where template resides
	Featured     bool      `json:"featured"`  // Whether this template is featured/recommended
	Metadata     StringMap `json:"metadata" gorm:"type:jsonb"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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

	// Resource Quotas (required)
	CPUQuota     *int `json:"cpu_quota" binding:"required"`     // CPU cores allocated to organization
	MemoryQuota  *int `json:"memory_quota" binding:"required"`  // Memory in GB allocated to organization
	StorageQuota *int `json:"storage_quota" binding:"required"` // Storage in GB allocated to organization
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

// Resource parsing helper functions

// ParseCPUString parses CPU strings like "4", "4 cores", "4c"
func ParseCPUString(cpuStr string) int {
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindStringSubmatch(cpuStr)
	if len(matches) > 1 {
		if val, err := strconv.Atoi(matches[1]); err == nil {
			return val
		}
	}
	return 0
}

// ParseMemoryString parses memory strings like "8Gi", "8GB", "8000Mi" and returns GB
func ParseMemoryString(memStr string) int {
	memStr = strings.ToUpper(strings.TrimSpace(memStr))
	re := regexp.MustCompile(`(\d+)\s*(GI?B?|MI?B?|KI?B?|TI?B?)`)
	matches := re.FindStringSubmatch(memStr)
	if len(matches) < 2 {
		return 0
	}

	val, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	unit := matches[2]
	switch {
	case strings.HasPrefix(unit, "T"): // TB, TiB
		return val * 1024 // Convert TB to GB
	case strings.HasPrefix(unit, "G"): // GB, GiB, Gi
		return val
	case strings.HasPrefix(unit, "M"): // MB, MiB, Mi
		return val / 1024 // Convert MB to GB
	case strings.HasPrefix(unit, "K"): // KB, KiB, Ki
		return val / (1024 * 1024) // Convert KB to GB
	default:
		return val // Assume GB if no unit
	}
}

// ParseStorageString parses storage strings like "100Gi", "100GB" and returns GB
func ParseStorageString(storStr string) int {
	// Storage parsing is same as memory parsing
	return ParseMemoryString(storStr)
}
