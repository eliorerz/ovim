package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// CRD-related constants
const (
	// Organization phases
	OrgPhasePending     = "Pending"
	OrgPhaseActive      = "Active"
	OrgPhaseFailed      = "Failed"
	OrgPhaseTerminating = "Terminating"

	// VDC phases
	VDCPhasePending     = "Pending"
	VDCPhaseActive      = "Active"
	VDCPhaseFailed      = "Failed"
	VDCPhaseSuspended   = "Suspended"
	VDCPhaseTerminating = "Terminating"

	// Catalog phases
	CatalogPhasePending   = "Pending"
	CatalogPhaseSyncing   = "Syncing"
	CatalogPhaseReady     = "Ready"
	CatalogPhaseFailed    = "Failed"
	CatalogPhaseSuspended = "Suspended"

	// Network policies
	NetworkPolicyDefault  = "default"
	NetworkPolicyIsolated = "isolated"
	NetworkPolicyCustom   = "custom"

	// Catalog types
	CatalogTypeVMTemplate       = "vm-template"
	CatalogTypeApplicationStack = "application-stack"
	CatalogTypeMixed            = "mixed"

	// Catalog source types
	CatalogSourceGit   = "git"
	CatalogSourceOCI   = "oci"
	CatalogSourceS3    = "s3"
	CatalogSourceHTTP  = "http"
	CatalogSourceLocal = "local"
)

// JSONBArray represents a JSONB array field
type JSONBArray []string

// Scan implements the Scanner interface for database deserialization
func (ja *JSONBArray) Scan(value interface{}) error {
	if value == nil {
		*ja = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into JSONBArray", value)
	}

	if len(bytes) == 0 {
		*ja = nil
		return nil
	}

	var result []string
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*ja = JSONBArray(result)
	return nil
}

// Value implements the driver Valuer interface for database serialization
func (ja JSONBArray) Value() (driver.Value, error) {
	if ja == nil {
		return nil, nil
	}
	return json.Marshal([]string(ja))
}

// JSONBMap represents a JSONB object field
type JSONBMap map[string]interface{}

// Scan implements the Scanner interface for database deserialization
func (jm *JSONBMap) Scan(value interface{}) error {
	if value == nil {
		*jm = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into JSONBMap", value)
	}

	if len(bytes) == 0 {
		*jm = nil
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*jm = JSONBMap(result)
	return nil
}

// Value implements the driver Valuer interface for database serialization
func (jm JSONBMap) Value() (driver.Value, error) {
	if jm == nil {
		return nil, nil
	}
	return json.Marshal(map[string]interface{}(jm))
}

// Condition represents a Kubernetes-style condition
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"` // True, False, Unknown
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
}

// ConditionsArray represents an array of conditions stored as JSONB
type ConditionsArray []Condition

// Scan implements the Scanner interface for database deserialization
func (ca *ConditionsArray) Scan(value interface{}) error {
	if value == nil {
		*ca = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into ConditionsArray", value)
	}

	if len(bytes) == 0 {
		*ca = nil
		return nil
	}

	var result []Condition
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*ca = ConditionsArray(result)
	return nil
}

// Value implements the driver Valuer interface for database serialization
func (ca ConditionsArray) Value() (driver.Value, error) {
	if ca == nil {
		return nil, nil
	}
	return json.Marshal([]Condition(ca))
}

// SetCondition updates or adds a condition to the conditions array
func (ca *ConditionsArray) SetCondition(conditionType, status, reason, message string) {
	newCondition := Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: time.Now(),
		Reason:             reason,
		Message:            message,
	}

	if *ca == nil {
		*ca = ConditionsArray{newCondition}
		return
	}

	// Update existing condition or add new one
	for i, condition := range *ca {
		if condition.Type == conditionType {
			// Only update timestamp if status changed
			if condition.Status != status {
				newCondition.LastTransitionTime = time.Now()
			} else {
				newCondition.LastTransitionTime = condition.LastTransitionTime
			}
			(*ca)[i] = newCondition
			return
		}
	}

	// Add new condition
	*ca = append(*ca, newCondition)
}

// GetCondition returns a condition by type
func (ca ConditionsArray) GetCondition(conditionType string) *Condition {
	for _, condition := range ca {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// IsConditionTrue checks if a condition is true
func (ca ConditionsArray) IsConditionTrue(conditionType string) bool {
	condition := ca.GetCondition(conditionType)
	return condition != nil && condition.Status == "True"
}

// Organization represents a tenant organization (identity and catalog container)
// Updated for CRD integration
type Organization struct {
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"uniqueIndex"`
	Description string `json:"description"`
	Namespace   string `json:"namespace" gorm:"uniqueIndex"`
	IsEnabled   bool   `json:"is_enabled" gorm:"default:true"`

	// CRD integration fields
	DisplayName        *string    `json:"display_name,omitempty"`
	CRName             string     `json:"cr_name" gorm:"uniqueIndex"`
	CRNamespace        string     `json:"cr_namespace" gorm:"default:default"`
	VDCCount           int        `json:"vdc_count" gorm:"default:0"`
	LastRBACSync       *time.Time `json:"last_rbac_sync,omitempty"`
	ObservedGeneration int64      `json:"observed_generation" gorm:"default:0"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	VirtualDataCenters []VirtualDataCenter `json:"virtual_data_centers,omitempty" gorm:"foreignKey:OrgID"`
	Catalogs           []Catalog           `json:"catalogs,omitempty" gorm:"foreignKey:OrgID"`
}

// VirtualDataCenter represents a virtual data center within an organization (resource container)
// Updated for CRD integration
type VirtualDataCenter struct {
	ID          string  `json:"id" gorm:"primaryKey"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	OrgID       string  `json:"org_id" gorm:"index"`
	ZoneID      *string `json:"zone_id,omitempty" gorm:"index"` // Zone where VDC is deployed

	// CRD integration fields
	DisplayName       *string `json:"display_name,omitempty"`
	CRName            string  `json:"cr_name" gorm:"index"`
	CRNamespace       string  `json:"cr_namespace" gorm:"index"`
	WorkloadNamespace string  `json:"workload_namespace" gorm:"uniqueIndex"`

	// Resource quotas (stored as integers for easier calculation)
	CPUQuota     int `json:"cpu_quota" gorm:"default:0"`
	MemoryQuota  int `json:"memory_quota" gorm:"default:0"`  // in GB
	StorageQuota int `json:"storage_quota" gorm:"default:0"` // in GB
	PodsQuota    int `json:"pods_quota" gorm:"default:100"`
	VMsQuota     int `json:"vms_quota" gorm:"default:50"`

	// VM LimitRange (optional)
	MinCPU    *int `json:"min_cpu,omitempty"`    // millicores
	MaxCPU    *int `json:"max_cpu,omitempty"`    // millicores
	MinMemory *int `json:"min_memory,omitempty"` // MiB
	MaxMemory *int `json:"max_memory,omitempty"` // MiB

	// Network and status
	NetworkPolicy       string     `json:"network_policy" gorm:"default:default"`
	CustomNetworkConfig JSONBMap   `json:"custom_network_config,omitempty" gorm:"type:jsonb"`
	CatalogRestrictions JSONBArray `json:"catalog_restrictions,omitempty" gorm:"type:jsonb"`

	// Status tracking
	Phase              string          `json:"phase" gorm:"default:Pending"`
	Conditions         ConditionsArray `json:"conditions,omitempty" gorm:"type:jsonb"`
	ObservedGeneration int64           `json:"observed_generation" gorm:"default:0"`

	// Resource usage tracking (updated by metrics controller)
	CPUUsed           int     `json:"cpu_used" gorm:"default:0"`
	MemoryUsed        int     `json:"memory_used" gorm:"default:0"`
	StorageUsed       int     `json:"storage_used" gorm:"default:0"`
	CPUPercentage     float64 `json:"cpu_percentage" gorm:"default:0.0"`
	MemoryPercentage  float64 `json:"memory_percentage" gorm:"default:0.0"`
	StoragePercentage float64 `json:"storage_percentage" gorm:"default:0.0"`

	// Workload counts
	TotalPods   int `json:"total_pods" gorm:"default:0"`
	RunningPods int `json:"running_pods" gorm:"default:0"`
	TotalVMs    int `json:"total_vms" gorm:"default:0"`
	RunningVMs  int `json:"running_vms" gorm:"default:0"`

	// Sync tracking
	LastMetricsUpdate *time.Time `json:"last_metrics_update,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Organization    *Organization    `json:"organization,omitempty" gorm:"foreignKey:OrgID"`
	VirtualMachines []VirtualMachine `json:"virtual_machines,omitempty" gorm:"foreignKey:VDCID"`
}

// GetResourceUsagePercentages calculates and returns resource usage percentages
func (vdc *VirtualDataCenter) GetResourceUsagePercentages() (cpuPct, memPct, storagePct float64) {
	if vdc.CPUQuota > 0 {
		cpuPct = float64(vdc.CPUUsed) / float64(vdc.CPUQuota) * 100
	}
	if vdc.MemoryQuota > 0 {
		memPct = float64(vdc.MemoryUsed) / float64(vdc.MemoryQuota) * 100
	}
	if vdc.StorageQuota > 0 {
		storagePct = float64(vdc.StorageUsed) / float64(vdc.StorageQuota) * 100
	}
	return cpuPct, memPct, storagePct
}

// UpdateResourceUsage updates the resource usage fields and percentages
func (vdc *VirtualDataCenter) UpdateResourceUsage(cpuUsed, memoryUsed, storageUsed int) {
	vdc.CPUUsed = cpuUsed
	vdc.MemoryUsed = memoryUsed
	vdc.StorageUsed = storageUsed

	vdc.CPUPercentage, vdc.MemoryPercentage, vdc.StoragePercentage = vdc.GetResourceUsagePercentages()

	now := time.Now()
	vdc.LastMetricsUpdate = &now
}

// Catalog represents a catalog resource in an organization
type Catalog struct {
	ID          string `json:"id" gorm:"primaryKey"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OrgID       string `json:"org_id" gorm:"index"`

	// CRD integration
	DisplayName *string `json:"display_name,omitempty"`
	CRName      string  `json:"cr_name" gorm:"index"`
	CRNamespace string  `json:"cr_namespace" gorm:"index"`

	// Catalog configuration
	Type                  string  `json:"type" gorm:"default:vm-template"` // vm-template, application-stack, mixed
	SourceType            string  `json:"source_type"`                     // git, oci, s3, http, local
	SourceURL             string  `json:"source_url"`
	SourceBranch          string  `json:"source_branch" gorm:"default:main"`
	SourcePath            string  `json:"source_path" gorm:"default:/"`
	SourceCredentials     *string `json:"source_credentials,omitempty"` // Secret name
	InsecureSkipTLSVerify bool    `json:"insecure_skip_tls_verify" gorm:"default:false"`
	RefreshInterval       string  `json:"refresh_interval" gorm:"default:1h"`

	// Content filtering
	IncludePatterns JSONBArray `json:"include_patterns,omitempty" gorm:"type:jsonb"`
	ExcludePatterns JSONBArray `json:"exclude_patterns,omitempty" gorm:"type:jsonb"`
	RequiredTags    JSONBArray `json:"required_tags,omitempty" gorm:"type:jsonb"`

	// Permissions
	AllowedVDCs   JSONBArray `json:"allowed_vdcs,omitempty" gorm:"type:jsonb"`
	AllowedGroups JSONBArray `json:"allowed_groups,omitempty" gorm:"type:jsonb"`
	ReadOnly      bool       `json:"read_only" gorm:"default:true"`

	// Status
	IsEnabled  bool            `json:"is_enabled" gorm:"default:true"`
	Phase      string          `json:"phase" gorm:"default:Pending"`
	Conditions ConditionsArray `json:"conditions,omitempty" gorm:"type:jsonb"`

	// Content summary
	TotalItems        int        `json:"total_items" gorm:"default:0"`
	VMTemplates       int        `json:"vm_templates" gorm:"default:0"`
	ApplicationStacks int        `json:"application_stacks" gorm:"default:0"`
	Categories        JSONBArray `json:"categories,omitempty" gorm:"type:jsonb"`

	// Sync status
	LastSync          *time.Time `json:"last_sync,omitempty"`
	LastSyncAttempt   *time.Time `json:"last_sync_attempt,omitempty"`
	SyncErrors        JSONBMap   `json:"sync_errors,omitempty" gorm:"type:jsonb"`
	NextSyncScheduled *time.Time `json:"next_sync_scheduled,omitempty"`

	ObservedGeneration int64 `json:"observed_generation" gorm:"default:0"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrgID"`
	Templates    []Template    `json:"templates,omitempty" gorm:"foreignKey:CatalogID"`
}

// UpdateSyncStatus updates the sync status fields
func (c *Catalog) UpdateSyncStatus(success bool, errorMsg string) {
	now := time.Now()
	c.LastSyncAttempt = &now

	if success {
		c.LastSync = &now
		c.Phase = CatalogPhaseReady
		c.Conditions.SetCondition("Synced", "True", "SyncSuccessful", "Catalog synced successfully")
	} else {
		c.Phase = CatalogPhaseFailed
		c.Conditions.SetCondition("Synced", "False", "SyncFailed", errorMsg)

		// Add to sync errors
		if c.SyncErrors == nil {
			c.SyncErrors = make(JSONBMap)
		}
		c.SyncErrors[now.Format(time.RFC3339)] = map[string]interface{}{
			"message":   errorMsg,
			"timestamp": now,
		}
	}
}

// CRD Request/Response types

// CreateOrganizationRequest represents a request to create an organization (CRD-aware)
type CreateOrganizationRequest struct {
	Name        string   `json:"name" binding:"required"`
	DisplayName string   `json:"display_name" binding:"required"`
	Description string   `json:"description"`
	Admins      []string `json:"admins" binding:"required,min=1"`
	IsEnabled   bool     `json:"is_enabled"`
}

// UpdateOrganizationRequest represents a request to update an organization
type UpdateOrganizationRequest struct {
	DisplayName *string  `json:"display_name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Admins      []string `json:"admins,omitempty"`
	IsEnabled   *bool    `json:"is_enabled,omitempty"`
}

// CreateVDCRequest represents a request to create a virtual data center (CRD-aware)
type CreateVDCRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Description string `json:"description"`
	OrgID       string `json:"org_id" binding:"required"`
	ZoneID      string `json:"zone_id" binding:"required"` // Zone where VDC will be deployed

	// Resource quotas
	CPUQuota     int `json:"cpu_quota" binding:"required,min=1"`
	MemoryQuota  int `json:"memory_quota" binding:"required,min=1"`  // in GB
	StorageQuota int `json:"storage_quota" binding:"required,min=1"` // in GB
	PodsQuota    int `json:"pods_quota,omitempty"`
	VMsQuota     int `json:"vms_quota,omitempty"`

	// Optional LimitRange parameters for VM resource constraints
	MinCPU    *int `json:"min_cpu,omitempty"`    // millicores
	MaxCPU    *int `json:"max_cpu,omitempty"`    // millicores
	MinMemory *int `json:"min_memory,omitempty"` // MiB
	MaxMemory *int `json:"max_memory,omitempty"` // MiB

	// Network configuration
	NetworkPolicy       string                 `json:"network_policy,omitempty"`
	CustomNetworkConfig map[string]interface{} `json:"custom_network_config,omitempty"`

	// Catalog restrictions
	CatalogRestrictions []string `json:"catalog_restrictions,omitempty"`
}

// UpdateVDCRequest represents a request to update a virtual data center
type UpdateVDCRequest struct {
	DisplayName         *string                `json:"display_name,omitempty"`
	Description         *string                `json:"description,omitempty"`
	CPUQuota            *int                   `json:"cpu_quota,omitempty"`
	MemoryQuota         *int                   `json:"memory_quota,omitempty"`
	StorageQuota        *int                   `json:"storage_quota,omitempty"`
	PodsQuota           *int                   `json:"pods_quota,omitempty"`
	VMsQuota            *int                   `json:"vms_quota,omitempty"`
	NetworkPolicy       *string                `json:"network_policy,omitempty"`
	CustomNetworkConfig map[string]interface{} `json:"custom_network_config,omitempty"`
	CatalogRestrictions []string               `json:"catalog_restrictions,omitempty"`
}

// CreateCatalogRequest represents a request to create a catalog
type CreateCatalogRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Description string `json:"description"`
	OrgID       string `json:"org_id" binding:"required"`

	// Catalog configuration
	Type                  string `json:"type" binding:"required"`
	SourceType            string `json:"source_type" binding:"required"`
	SourceURL             string `json:"source_url" binding:"required"`
	SourceBranch          string `json:"source_branch,omitempty"`
	SourcePath            string `json:"source_path,omitempty"`
	SourceCredentials     string `json:"source_credentials,omitempty"`
	InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify,omitempty"`
	RefreshInterval       string `json:"refresh_interval,omitempty"`

	// Content filtering
	IncludePatterns []string `json:"include_patterns,omitempty"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
	RequiredTags    []string `json:"required_tags,omitempty"`

	// Permissions
	AllowedVDCs   []string `json:"allowed_vdcs,omitempty"`
	AllowedGroups []string `json:"allowed_groups,omitempty"`
	ReadOnly      bool     `json:"read_only,omitempty"`
	IsEnabled     bool     `json:"is_enabled,omitempty"`
}

// UpdateCatalogRequest represents a request to update a catalog
type UpdateCatalogRequest struct {
	DisplayName           *string  `json:"display_name,omitempty"`
	Description           *string  `json:"description,omitempty"`
	SourceURL             *string  `json:"source_url,omitempty"`
	SourceBranch          *string  `json:"source_branch,omitempty"`
	SourcePath            *string  `json:"source_path,omitempty"`
	SourceCredentials     *string  `json:"source_credentials,omitempty"`
	InsecureSkipTLSVerify *bool    `json:"insecure_skip_tls_verify,omitempty"`
	RefreshInterval       *string  `json:"refresh_interval,omitempty"`
	IncludePatterns       []string `json:"include_patterns,omitempty"`
	ExcludePatterns       []string `json:"exclude_patterns,omitempty"`
	RequiredTags          []string `json:"required_tags,omitempty"`
	AllowedVDCs           []string `json:"allowed_vdcs,omitempty"`
	AllowedGroups         []string `json:"allowed_groups,omitempty"`
	ReadOnly              *bool    `json:"read_only,omitempty"`
	IsEnabled             *bool    `json:"is_enabled,omitempty"`
}
