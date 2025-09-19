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
	RoleOrgMember   = "org_member"
)

// Event types
const (
	EventTypeNormal  = "Normal"
	EventTypeWarning = "Warning"
	EventTypeError   = "Error"
)

// Event categories
const (
	EventCategoryOrganization = "organization"
	EventCategoryVDC          = "vdc"
	EventCategoryVM           = "vm"
	EventCategorySecurity     = "security"
	EventCategoryPerformance  = "performance"
	EventCategoryIntegration  = "integration"
	EventCategorySystem       = "system"
	EventCategoryAudit        = "audit"
	EventCategoryQuota        = "quota"
	EventCategoryNetwork      = "network"
	EventCategoryStorage      = "storage"
	EventCategoryBackup       = "backup"
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

// Legacy types moved to migration_compat.go to avoid duplicates

// OrganizationResourceUsage represents current resource usage across all VDCs in an organization
type OrganizationResourceUsage struct {
	CPUUsed     int `json:"cpu_used"`
	MemoryUsed  int `json:"memory_used"`
	StorageUsed int `json:"storage_used"`

	// Total quota allocated across all VDCs
	CPUQuota     int `json:"cpu_quota"`
	MemoryQuota  int `json:"memory_quota"`
	StorageQuota int `json:"storage_quota"`

	// Available resources (quota - used)
	CPUAvailable     int `json:"cpu_available"`
	MemoryAvailable  int `json:"memory_available"`
	StorageAvailable int `json:"storage_available"`

	VDCCount int `json:"vdc_count"` // Number of VDCs in the organization
}

// VDCResourceUsage represents current resource usage for a specific VDC
type VDCResourceUsage struct {
	CPUUsed     int `json:"cpu_used"`
	MemoryUsed  int `json:"memory_used"`
	StorageUsed int `json:"storage_used"`

	// VDC quota
	CPUQuota     int `json:"cpu_quota"`
	MemoryQuota  int `json:"memory_quota"`
	StorageQuota int `json:"storage_quota"`

	// Available resources (quota - used)
	CPUAvailable     int `json:"cpu_available"`
	MemoryAvailable  int `json:"memory_available"`
	StorageAvailable int `json:"storage_available"`

	VMCount int `json:"vm_count"` // Number of VMs in the VDC
}

// GetResourceUsage calculates current resource usage for a specific VDC
func (vdc *VirtualDataCenter) GetResourceUsage(vms []*VirtualMachine) VDCResourceUsage {
	var cpuUsed, memoryUsed, storageUsed int
	var cpuQuota, memoryQuota, storageQuota int
	var vmCount int

	// Get quota from this VDC's CRD fields
	cpuQuota = vdc.CPUQuota
	memoryQuota = vdc.MemoryQuota
	storageQuota = vdc.StorageQuota

	// Calculate actual usage from VMs in this specific VDC
	for _, vm := range vms {
		if vm.VDCID != nil && *vm.VDCID == vdc.ID {
			// Only count VMs that are deployed (not stopped/failed)
			if vm.Status == "Running" || vm.Status == "Stopped" || vm.Status == "Paused" {
				cpuUsed += vm.CPU
				memoryUsed += ParseMemoryString(vm.Memory)
				storageUsed += ParseStorageString(vm.DiskSize)
				vmCount++
			}
		}
	}

	return VDCResourceUsage{
		CPUUsed:     cpuUsed,
		MemoryUsed:  memoryUsed,
		StorageUsed: storageUsed,

		CPUQuota:     cpuQuota,
		MemoryQuota:  memoryQuota,
		StorageQuota: storageQuota,

		CPUAvailable:     cpuQuota - cpuUsed,
		MemoryAvailable:  memoryQuota - memoryUsed,
		StorageAvailable: storageQuota - storageUsed,

		VMCount: vmCount,
	}
}

// GetResourceUsage calculates current resource usage across all VDCs in the organization
func (o *Organization) GetResourceUsage(vdcs []*VirtualDataCenter, vms []*VirtualMachine) OrganizationResourceUsage {
	var totalCPUUsed, totalMemoryUsed, totalStorageUsed int
	var totalCPUQuota, totalMemoryQuota, totalStorageQuota int

	// Aggregate usage and quotas from all VDCs
	for _, vdc := range vdcs {
		// Add up the quotas
		totalCPUQuota += vdc.CPUQuota
		totalMemoryQuota += vdc.MemoryQuota
		totalStorageQuota += vdc.StorageQuota

		// Calculate usage for this VDC
		vdcUsage := vdc.GetResourceUsage(vms)
		totalCPUUsed += vdcUsage.CPUUsed
		totalMemoryUsed += vdcUsage.MemoryUsed
		totalStorageUsed += vdcUsage.StorageUsed
	}

	return OrganizationResourceUsage{
		CPUUsed:          totalCPUUsed,
		MemoryUsed:       totalMemoryUsed,
		StorageUsed:      totalStorageUsed,
		CPUQuota:         totalCPUQuota,
		MemoryQuota:      totalMemoryQuota,
		StorageQuota:     totalStorageQuota,
		CPUAvailable:     totalCPUQuota - totalCPUUsed,
		MemoryAvailable:  totalMemoryQuota - totalMemoryUsed,
		StorageAvailable: totalStorageQuota - totalStorageUsed,
		VDCCount:         len(vdcs),
	}
}

// CanAllocateResources checks if the organization can allocate the requested resources
// Since organizations no longer have quotas, this always returns true
// Resource allocation is now handled at the VDC level
func (o *Organization) CanAllocateResources(cpuReq, memoryReq, storageReq int, vdcs []*VirtualDataCenter) bool {
	// Organizations are identity containers only - no resource limits
	return true
}

// Legacy VDC methods moved to migration_compat.go

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
	ID           string `json:"id" gorm:"primaryKey"`
	Name         string `json:"name"`
	TemplateName string `json:"template_name"` // Actual OpenShift template name
	Description  string `json:"description"`
	OSType       string `json:"os_type"`
	OSVersion    string `json:"os_version"`
	CPU          int    `json:"cpu"`
	Memory       string `json:"memory"`
	DiskSize     string `json:"disk_size"`
	ImageURL     string `json:"image_url"`
	IconClass    string `json:"icon_class"`
	OrgID        string `json:"org_id" gorm:"index"`

	// CRD catalog integration
	CatalogID   *string `json:"catalog_id,omitempty" gorm:"index"`         // Reference to new Catalog CRD
	ContentType string  `json:"content_type" gorm:"default:'vm-template'"` // vm-template, application-stack

	// Legacy fields (for backward compatibility)
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
	OrgID      string    `json:"org_id" gorm:"index"`
	VDCID      *string   `json:"vdc_id,omitempty" gorm:"index"` // Updated for optional VDC association
	TemplateID string    `json:"template_id" gorm:"index"`
	OwnerID    string    `json:"owner_id" gorm:"index"`
	Status     string    `json:"status" gorm:"index"`
	CPU        int       `json:"cpu"`
	Memory     string    `json:"memory"`
	DiskSize   string    `json:"disk_size"`
	IPAddress  string    `json:"ip_address"`
	Metadata   StringMap `json:"metadata" gorm:"type:jsonb"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LimitRangeRequest represents LimitRange parameters for VM resource constraints
type LimitRangeRequest struct {
	MinCPU    int `json:"min_cpu"`    // Minimum CPU cores per VM
	MaxCPU    int `json:"max_cpu"`    // Maximum CPU cores per VM
	MinMemory int `json:"min_memory"` // Minimum memory in GB per VM
	MaxMemory int `json:"max_memory"` // Maximum memory in GB per VM
}

// Legacy CRD request types moved to crd_types.go to avoid conflicts
// Only VM-specific requests remain here

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
	switch unit {
	// Binary units (base 1024)
	case "GI", "GIB":
		// Binary Gibibytes - convert to decimal GB
		// 1 GiB = 1024³ bytes = 1.073741824 GB
		return int(float64(val) * 1.073741824)
	case "TI", "TIB":
		// Binary Tebibytes - convert to decimal GB
		// 1 TiB = 1024⁴ bytes = 1099.511627776 GB
		return int(float64(val) * 1099.511627776)
	case "MI", "MIB":
		// Binary Mebibytes - convert to decimal GB
		// 1 MiB = 1024² bytes = 0.001048576 GB
		return int(float64(val) * 0.001048576)
	case "KI", "KIB":
		// Binary Kibibytes - convert to decimal GB
		// 1 KiB = 1024 bytes = 0.000001024 GB
		return int(float64(val) * 0.000001024)

	// Decimal units (base 1000)
	case "TB":
		// Decimal Terabytes - convert to GB
		return val * 1000
	case "GB":
		// Decimal Gigabytes - already in GB
		return val
	case "MB":
		// Decimal Megabytes - convert to GB
		return val / 1000
	case "KB":
		// Decimal Kilobytes - convert to GB
		return val / (1000 * 1000)

	// Legacy cases for compatibility
	case "G":
		// Assume binary (legacy case)
		return int(float64(val) * 1.073741824)
	case "M":
		// Assume binary (legacy case)
		return int(float64(val) * 0.001048576)
	case "T":
		// Assume binary (legacy case)
		return int(float64(val) * 1099.511627776)
	case "K":
		// Assume binary (legacy case)
		return int(float64(val) * 0.000001024)

	default:
		return val // Assume GB if no unit
	}
}

// ParseStorageString parses storage strings like "100Gi", "100GB" and returns GB
func ParseStorageString(storStr string) int {
	// Storage parsing is same as memory parsing
	return ParseMemoryString(storStr)
}

// LimitRangeInfo represents current LimitRange information for an organization namespace
type LimitRangeInfo struct {
	Exists    bool `json:"exists"`     // Whether LimitRange exists
	MinCPU    int  `json:"min_cpu"`    // Minimum CPU cores per VM
	MaxCPU    int  `json:"max_cpu"`    // Maximum CPU cores per VM
	MinMemory int  `json:"min_memory"` // Minimum memory in GB per VM
	MaxMemory int  `json:"max_memory"` // Maximum memory in GB per VM
}

// OrganizationCatalogSource represents a catalog source attached to an organization
type OrganizationCatalogSource struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	OrgID           string    `json:"org_id" gorm:"index"`
	SourceType      string    `json:"source_type"`      // Type of catalog source (e.g., "operatorhubio", "redhat-operators")
	SourceName      string    `json:"source_name"`      // Display name for this source in the organization
	SourceNamespace string    `json:"source_namespace"` // OpenShift namespace where the catalog source exists
	Enabled         bool      `json:"enabled" gorm:"default:true"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateOrganizationCatalogSourceRequest represents a request to add a catalog source to an organization
type CreateOrganizationCatalogSourceRequest struct {
	SourceType      string `json:"source_type" binding:"required"`
	SourceName      string `json:"source_name" binding:"required"`
	SourceNamespace string `json:"source_namespace" binding:"required"`
	Enabled         bool   `json:"enabled"`
}

// UpdateOrganizationCatalogSourceRequest represents a request to update an organization catalog source
type UpdateOrganizationCatalogSourceRequest struct {
	SourceName *string `json:"source_name,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
}

// CatalogSource represents a source of templates or applications available in the catalog
type CatalogSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	URL         string `json:"url,omitempty"`
	Path        string `json:"path,omitempty"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// Event represents a persistent event in the database
type Event struct {
	ID       string `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name     string `json:"name" gorm:"not null"`
	EventUID string `json:"event_uid,omitempty" gorm:"uniqueIndex"`

	// Event classification
	Type      string `json:"type" gorm:"not null;default:'Normal'"`
	Reason    string `json:"reason" gorm:"not null"`
	Category  string `json:"category" gorm:"default:'General'"`
	Component string `json:"component" gorm:"not null"`

	// Event content
	Message string `json:"message" gorm:"type:text;not null"`
	Action  string `json:"action,omitempty"`

	// Event context
	Namespace string  `json:"namespace,omitempty"`
	OrgID     *string `json:"org_id,omitempty" gorm:"index"`
	VDCID     *string `json:"vdc_id,omitempty" gorm:"index"`
	VMID      *string `json:"vm_id,omitempty" gorm:"index"`
	UserID    *string `json:"user_id,omitempty" gorm:"index"`
	Username  string  `json:"username,omitempty"`

	// Involved object (Kubernetes resource)
	InvolvedObjectKind            string `json:"involved_object_kind,omitempty"`
	InvolvedObjectName            string `json:"involved_object_name,omitempty"`
	InvolvedObjectNamespace       string `json:"involved_object_namespace,omitempty"`
	InvolvedObjectUID             string `json:"involved_object_uid,omitempty"`
	InvolvedObjectResourceVersion string `json:"involved_object_resource_version,omitempty"`

	// Event metadata
	Metadata    StringMap `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	Annotations StringMap `json:"annotations" gorm:"type:jsonb;default:'{}'"`
	Labels      StringMap `json:"labels" gorm:"type:jsonb;default:'{}'"`

	// Event timing
	FirstTimestamp time.Time `json:"first_timestamp" gorm:"not null;default:CURRENT_TIMESTAMP"`
	LastTimestamp  time.Time `json:"last_timestamp" gorm:"not null;default:CURRENT_TIMESTAMP"`
	EventTime      time.Time `json:"event_time" gorm:"default:CURRENT_TIMESTAMP"`

	// Event counting and aggregation
	Count int `json:"count" gorm:"default:1"`

	// Event source and reporting
	SourceComponent     string `json:"source_component,omitempty"`
	SourceHost          string `json:"source_host,omitempty"`
	ReportingController string `json:"reporting_controller,omitempty"`
	ReportingInstance   string `json:"reporting_instance,omitempty"`

	// Event series (for related events)
	SeriesCount            *int       `json:"series_count,omitempty"`
	SeriesLastObservedTime *time.Time `json:"series_last_observed_time,omitempty"`
	SeriesState            string     `json:"series_state,omitempty"`

	// Event lifecycle
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// EventCategory represents an event category configuration
type EventCategory struct {
	Name        string    `json:"name" gorm:"primaryKey"`
	Description string    `json:"description"`
	Color       string    `json:"color" gorm:"default:'#1f77b4'"`
	Icon        string    `json:"icon"`
	CreatedAt   time.Time `json:"created_at"`
}

// EventRetentionPolicy represents event retention configuration
type EventRetentionPolicy struct {
	ID            int       `json:"id" gorm:"primaryKey"`
	Category      string    `json:"category" gorm:"not null"`
	Type          string    `json:"type" gorm:"not null;default:'all'"`
	RetentionDays int       `json:"retention_days" gorm:"not null;default:30"`
	MaxEvents     int       `json:"max_events" gorm:"default:10000"`
	AutoCleanup   bool      `json:"auto_cleanup" gorm:"default:true"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Event filter and request types

// EventFilter represents filters for event queries
type EventFilter struct {
	Type           []string `form:"type"`
	Category       []string `form:"category"`
	Reason         []string `form:"reason"`
	Component      []string `form:"component"`
	Namespace      []string `form:"namespace"`
	OrgID          string   `form:"org_id"`
	VDCID          string   `form:"vdc_id"`
	VMID           string   `form:"vm_id"`
	UserID         string   `form:"user_id"`
	Username       string   `form:"username"`
	Search         string   `form:"search"`          // Full-text search in message
	Since          string   `form:"since"`           // Time filter (RFC3339)
	Until          string   `form:"until"`           // Time filter (RFC3339)
	Limit          int      `form:"limit"`           // Pagination limit
	Page           int      `form:"page"`            // Pagination page
	SortBy         string   `form:"sort_by"`         // Sort field
	SortOrder      string   `form:"sort_order"`      // asc/desc
	IncludeDeleted bool     `form:"include_deleted"` // Include soft-deleted events
}

// EventsResponse represents a paginated list of events
type EventsResponse struct {
	Events     []Event `json:"events"`
	TotalCount int64   `json:"total_count"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	TotalPages int     `json:"total_pages"`
}

// CreateEventRequest represents a request to create a new event
type CreateEventRequest struct {
	Name      string `json:"name" binding:"required"`
	Type      string `json:"type" binding:"required"`
	Reason    string `json:"reason" binding:"required"`
	Message   string `json:"message" binding:"required"`
	Category  string `json:"category"`
	Component string `json:"component" binding:"required"`
	Action    string `json:"action,omitempty"`

	// Context
	Namespace string `json:"namespace,omitempty"`
	OrgID     string `json:"org_id,omitempty"`
	VDCID     string `json:"vdc_id,omitempty"`
	VMID      string `json:"vm_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`

	// Involved object
	InvolvedObjectKind      string `json:"involved_object_kind,omitempty"`
	InvolvedObjectName      string `json:"involved_object_name,omitempty"`
	InvolvedObjectNamespace string `json:"involved_object_namespace,omitempty"`
	InvolvedObjectUID       string `json:"involved_object_uid,omitempty"`

	// Metadata
	Metadata    map[string]string `json:"metadata,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`

	// Timing
	EventTime *time.Time `json:"event_time,omitempty"`
}

// BulkCreateEventsRequest represents a request to create multiple events
type BulkCreateEventsRequest struct {
	Events []CreateEventRequest `json:"events" binding:"required,min=1,max=100"`
}
