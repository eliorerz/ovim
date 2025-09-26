package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrganizationSpec defines the desired state of Organization
type OrganizationSpec struct {
	// DisplayName is the human-readable organization name
	DisplayName string `json:"displayName"`

	// Description describes the organization
	Description string `json:"description,omitempty"`

	// Admins is a list of admin group names
	Admins []string `json:"admins"`

	// IsEnabled indicates if the organization is active
	IsEnabled bool `json:"isEnabled"`

	// Catalogs contains catalog resources managed by this org
	Catalogs []CatalogReference `json:"catalogs,omitempty"`
}

// CatalogReference represents a catalog resource reference
type CatalogReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"` // vm-template, application-stack
}

// OrganizationStatus defines the observed state of Organization
type OrganizationStatus struct {
	// Namespace is the created org namespace name
	Namespace string `json:"namespace,omitempty"`

	// Phase represents the current phase of the organization
	Phase OrganizationPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// VDCCount is the number of VDCs in this organization
	VDCCount int `json:"vdcCount,omitempty"`

	// LastRBACSync is the last time RBAC was synced to VDCs
	LastRBACSync *metav1.Time `json:"lastRBACSync,omitempty"`
}

// OrganizationPhase represents the phase of an organization
type OrganizationPhase string

const (
	OrganizationPhasePending OrganizationPhase = "Pending"
	OrganizationPhaseActive  OrganizationPhase = "Active"
	OrganizationPhaseFailed  OrganizationPhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Organization is the Schema for the organizations API
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrganizationSpec   `json:"spec,omitempty"`
	Status OrganizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrganizationList contains a list of Organization
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}

// VirtualDataCenterSpec defines the desired state of VirtualDataCenter
type VirtualDataCenterSpec struct {
	// OrganizationRef references the parent Organization
	OrganizationRef string `json:"organizationRef"`

	// ZoneID specifies the zone where this VDC will be deployed
	ZoneID string `json:"zoneId"`

	// DisplayName is the human-readable VDC name
	DisplayName string `json:"displayName"`

	// Description describes the VDC
	Description string `json:"description,omitempty"`

	// Quota defines resource quotas for this VDC
	Quota ResourceQuota `json:"quota"`

	// LimitRange defines VM resource constraints (optional)
	LimitRange *LimitRange `json:"limitRange,omitempty"`

	// NetworkPolicy defines network isolation
	NetworkPolicy string `json:"networkPolicy,omitempty"`

	// CustomNetworkConfig defines custom network configuration when NetworkPolicy is "custom"
	// +kubebuilder:pruning:PreserveUnknownFields
	CustomNetworkConfig map[string]string `json:"customNetworkConfig,omitempty"`

	// VDCType specifies whether this is hub-managed or spoke-local
	// +kubebuilder:validation:Enum=hub-managed;spoke-local
	// +kubebuilder:default=hub-managed
	VDCType string `json:"vdcType,omitempty"`

	// HubVDCName is the original VDC name from hub (for spoke VDCs)
	HubVDCName string `json:"hubVDCName,omitempty"`

	// HubVDCNamespace is the hub namespace (for spoke VDCs)
	HubVDCNamespace string `json:"hubVDCNamespace,omitempty"`

	// OrgNamespace is the org namespace on spoke (for spoke VDCs)
	OrgNamespace string `json:"orgNamespace,omitempty"`

	// TargetNamespace is the workload namespace (for spoke VDCs)
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// HubOperationID is the current operation ID from hub (for spoke VDCs)
	HubOperationID string `json:"hubOperationID,omitempty"`

	// LastHubSync is the last successful hub sync timestamp (for spoke VDCs)
	LastHubSync *metav1.Time `json:"lastHubSync,omitempty"`

	// ReconcileUntilSuccess keeps reconciling until hub confirms success (for spoke VDCs)
	ReconcileUntilSuccess bool `json:"reconcileUntilSuccess,omitempty"`
}

// ResourceQuota defines resource limits
type ResourceQuota struct {
	CPU             string `json:"cpu"`                       // e.g., "20"
	Memory          string `json:"memory"`                    // e.g., "64Gi"
	Storage         string `json:"storage"`                   // e.g., "500Ti"
	Pods            int    `json:"pods,omitempty"`            // Maximum number of pods
	VirtualMachines int    `json:"virtualMachines,omitempty"` // Maximum number of virtual machines
}

// LimitRange defines VM resource constraints
type LimitRange struct {
	MinCpu    int `json:"minCpu"`    // Minimum CPU cores per VM
	MaxCpu    int `json:"maxCpu"`    // Maximum CPU cores per VM
	MinMemory int `json:"minMemory"` // Minimum memory in GB per VM
	MaxMemory int `json:"maxMemory"` // Maximum memory in GB per VM
}

// VirtualDataCenterStatus defines the observed state of VirtualDataCenter
type VirtualDataCenterStatus struct {
	// Namespace is the VDC workload namespace
	Namespace string `json:"namespace,omitempty"`

	// Phase represents the current phase of the VDC
	Phase VirtualDataCenterPhase `json:"phase,omitempty"`

	// ResourceUsage shows current resource consumption
	ResourceUsage *ResourceUsage `json:"resourceUsage,omitempty"`

	// LastMetricsUpdate is when metrics were last collected
	LastMetricsUpdate *metav1.Time `json:"lastMetricsUpdate,omitempty"`

	// TotalPods is the current number of pods in VDC
	TotalPods int `json:"totalPods,omitempty"`

	// TotalVMs is the current number of VMs in VDC
	TotalVMs int `json:"totalVMs,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// OrgNamespace is the created org namespace (for spoke VDCs)
	OrgNamespace string `json:"orgNamespace,omitempty"`

	// WorkloadNamespace is the created workload namespace (for spoke VDCs)
	WorkloadNamespace string `json:"workloadNamespace,omitempty"`

	// LastReconcile is the last reconcile attempt timestamp (for spoke VDCs)
	LastReconcile *metav1.Time `json:"lastReconcile,omitempty"`

	// HubSyncStatus indicates the hub synchronization status (for spoke VDCs)
	HubSyncStatus string `json:"hubSyncStatus,omitempty"`

	// RetryCount is the number of reconcile attempts (for spoke VDCs)
	RetryCount int `json:"retryCount,omitempty"`

	// LastHubSync is the last successful hub sync timestamp (for spoke VDCs)
	LastHubSync *metav1.Time `json:"lastHubSync,omitempty"`
}

// ResourceUsage represents current resource consumption
type ResourceUsage struct {
	CPUUsed     string `json:"cpuUsed"`
	MemoryUsed  string `json:"memoryUsed"`
	StorageUsed string `json:"storageUsed"`
}

// VirtualDataCenterPhase represents the phase of a VDC
type VirtualDataCenterPhase string

const (
	VirtualDataCenterPhasePending         VirtualDataCenterPhase = "Pending"
	VirtualDataCenterPhaseActive          VirtualDataCenterPhase = "Active"
	VirtualDataCenterPhaseFailed          VirtualDataCenterPhase = "Failed"
	VirtualDataCenterPhaseSuspended       VirtualDataCenterPhase = "Suspended"
	VirtualDataCenterPhaseDeletionPending VirtualDataCenterPhase = "DeletionPending"
	VirtualDataCenterPhaseDeletionFailed  VirtualDataCenterPhase = "DeletionFailed"
)

// VDC type constants
const (
	VDCTypeHubManaged = "hub-managed"
	VDCTypeSpokeLocal = "spoke-local"
)

// Hub sync status constants
const (
	HubSyncStatusPending = "pending"
	HubSyncStatusSuccess = "success"
	HubSyncStatusFailed  = "failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

// VirtualDataCenter is the Schema for the virtualdatacenters API
type VirtualDataCenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualDataCenterSpec   `json:"spec,omitempty"`
	Status VirtualDataCenterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VirtualDataCenterList contains a list of VirtualDataCenter
type VirtualDataCenterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualDataCenter `json:"items"`
}

// CatalogSpec defines the desired state of Catalog
type CatalogSpec struct {
	// OrganizationRef references the owning Organization
	OrganizationRef string `json:"organizationRef"`

	// DisplayName is the human-readable catalog name
	DisplayName string `json:"displayName"`

	// Description describes the catalog
	Description string `json:"description,omitempty"`

	// Type defines the catalog content type
	Type string `json:"type"` // vm-template, application-stack

	// Source defines where catalog content comes from
	Source CatalogSource `json:"source"`
}

// CatalogSource defines catalog content source
type CatalogSource struct {
	Type        string `json:"type"`        // git, oci, s3
	URL         string `json:"url"`         // Source URL
	Credentials string `json:"credentials"` // Secret reference for authentication
}

// CatalogStatus defines the observed state of Catalog
type CatalogStatus struct {
	// Phase represents the current phase of the catalog
	Phase CatalogPhase `json:"phase,omitempty"`

	// ItemCount is the number of items in this catalog
	ItemCount int `json:"itemCount,omitempty"`

	// LastSync is when the catalog was last synchronized
	LastSync *metav1.Time `json:"lastSync,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// CatalogPhase represents the phase of a catalog
type CatalogPhase string

const (
	CatalogPhasePending CatalogPhase = "Pending"
	CatalogPhaseReady   CatalogPhase = "Ready"
	CatalogPhaseFailed  CatalogPhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

// Catalog is the Schema for the catalogs API
type Catalog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CatalogSpec   `json:"spec,omitempty"`
	Status CatalogStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CatalogList contains a list of Catalog
type CatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Catalog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
	SchemeBuilder.Register(&VirtualDataCenter{}, &VirtualDataCenterList{})
	SchemeBuilder.Register(&Catalog{}, &CatalogList{})
}
