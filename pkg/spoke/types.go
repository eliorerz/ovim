package spoke

import (
	"time"
)

// OperationType defines the type of operation requested from the hub
type OperationType string

const (
	OperationCreateVM      OperationType = "create_vm"
	OperationDeleteVM      OperationType = "delete_vm"
	OperationStartVM       OperationType = "start_vm"
	OperationStopVM        OperationType = "stop_vm"
	OperationGetVMStatus   OperationType = "get_vm_status"
	OperationListVMs       OperationType = "list_vms"
	OperationGetHealth     OperationType = "get_health"
	OperationGetMetrics    OperationType = "get_metrics"
	OperationCreateVDC     OperationType = "create_vdc"
	OperationDeleteVDC     OperationType = "delete_vdc"
	OperationSyncTemplates OperationType = "sync_templates"
)

// OperationStatus defines the status of an operation
type OperationStatus string

const (
	OperationStatusPending   OperationStatus = "pending"
	OperationStatusRunning   OperationStatus = "running"
	OperationStatusCompleted OperationStatus = "completed"
	OperationStatusFailed    OperationStatus = "failed"
)

// AgentStatus defines the overall status of the spoke agent
type AgentStatus string

const (
	AgentStatusHealthy     AgentStatus = "healthy"
	AgentStatusDegraded    AgentStatus = "degraded"
	AgentStatusUnavailable AgentStatus = "unavailable"
)

// Operation represents a request from the hub to perform an operation
type Operation struct {
	ID          string                 `json:"id"`
	Type        OperationType          `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	Timestamp   time.Time              `json:"timestamp"`
	RetryCount  int                    `json:"retry_count,omitempty"`
	TimeoutSecs int                    `json:"timeout_seconds,omitempty"`
}

// OperationResult represents the result of an operation sent back to the hub
type OperationResult struct {
	OperationID string                 `json:"operation_id"`
	Status      OperationStatus        `json:"status"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Duration    time.Duration          `json:"duration,omitempty"`
}

// VMStatus represents the status of a VM on the spoke cluster
type VMStatus struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Status    string            `json:"status"`
	Phase     string            `json:"phase"`
	NodeName  string            `json:"node_name,omitempty"`
	IPAddress string            `json:"ip_address,omitempty"`
	CPU       string            `json:"cpu,omitempty"`
	Memory    string            `json:"memory,omitempty"`
	Storage   string            `json:"storage,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// VDCStatus represents the status of a VDC on the spoke cluster
type VDCStatus struct {
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	Status        string            `json:"status"`
	ResourceUsage ResourceMetrics   `json:"resource_usage"`
	Labels        map[string]string `json:"labels,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// ResourceMetrics represents resource usage and capacity
type ResourceMetrics struct {
	CPUUsed         int64   `json:"cpu_used"`     // millicores
	CPUCapacity     int64   `json:"cpu_capacity"` // millicores
	CPUPercent      float64 `json:"cpu_percent"`
	MemoryUsed      int64   `json:"memory_used"`     // bytes
	MemoryCapacity  int64   `json:"memory_capacity"` // bytes
	MemoryPercent   float64 `json:"memory_percent"`
	StorageUsed     int64   `json:"storage_used"`     // bytes
	StorageCapacity int64   `json:"storage_capacity"` // bytes
	StoragePercent  float64 `json:"storage_percent"`
	NodeCount       int     `json:"node_count"`
	PodCount        int     `json:"pod_count"`
	VMCount         int     `json:"vm_count"`
}

// ClusterMetrics represents overall cluster metrics
type ClusterMetrics struct {
	ClusterID     string          `json:"cluster_id"`
	ZoneID        string          `json:"zone_id"`
	Resources     ResourceMetrics `json:"resources"`
	NodeStatus    []NodeStatus    `json:"node_status"`
	LastCollected time.Time       `json:"last_collected"`
}

// NodeStatus represents individual node metrics
type NodeStatus struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	CPU        ResourceMetrics   `json:"cpu"`
	Memory     ResourceMetrics   `json:"memory"`
	Storage    ResourceMetrics   `json:"storage"`
	Labels     map[string]string `json:"labels,omitempty"`
	Conditions []NodeCondition   `json:"conditions,omitempty"`
}

// NodeCondition represents a node condition
type NodeCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"last_transition_time"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// StatusReport represents the periodic status report sent to the hub
type StatusReport struct {
	AgentID        string         `json:"agent_id"`
	ClusterID      string         `json:"cluster_id"`
	ZoneID         string         `json:"zone_id"`
	Status         AgentStatus    `json:"status"`
	Version        string         `json:"version"`
	Metrics        ClusterMetrics `json:"metrics"`
	VDCs           []VDCStatus    `json:"vdcs"`
	VMs            []VMStatus     `json:"vms"`
	LastHubContact time.Time      `json:"last_hub_contact"`
	ReportTime     time.Time      `json:"report_time"`
	Errors         []string       `json:"errors,omitempty"`
	CallbackURL    string         `json:"callback_url,omitempty"`
}

// VMCreateRequest represents a request to create a VM
type VMCreateRequest struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	TemplateName string            `json:"template_name"`
	CPU          string            `json:"cpu"`
	Memory       string            `json:"memory"`
	Storage      string            `json:"storage"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	NetworkName  string            `json:"network_name,omitempty"`
}

// VDCCreateRequest represents a request to create a VDC
type VDCCreateRequest struct {
	Name             string            `json:"name"`
	OrganizationName string            `json:"organization_name"`
	CPUQuota         int64             `json:"cpu_quota"`
	MemoryQuota      int64             `json:"memory_quota"`
	StorageQuota     int64             `json:"storage_quota"`
	NetworkPolicy    string            `json:"network_policy"`
	Labels           map[string]string `json:"labels,omitempty"`
	Annotations      map[string]string `json:"annotations,omitempty"`
}

// Template represents a VM template cached locally
type Template struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	DisplayName string            `json:"display_name"`
	Description string            `json:"description"`
	OSType      string            `json:"os_type"`
	OSVersion   string            `json:"os_version"`
	CPU         string            `json:"cpu"`
	Memory      string            `json:"memory"`
	Storage     string            `json:"storage"`
	Labels      map[string]string `json:"labels,omitempty"`
	SyncedAt    time.Time         `json:"synced_at"`
}

// HealthCheck represents a health check result
type HealthCheck struct {
	Component   string        `json:"component"`
	Status      string        `json:"status"` // "healthy", "warning", "critical"
	Message     string        `json:"message,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
	Duration    time.Duration `json:"duration,omitempty"`
}

// AgentHealth represents the overall health of the agent
type AgentHealth struct {
	Status     AgentStatus   `json:"status"`
	Checks     []HealthCheck `json:"checks"`
	Uptime     time.Duration `json:"uptime"`
	Version    string        `json:"version"`
	LastReport time.Time     `json:"last_report"`
}

// VMMetrics represents virtual machine resource metrics
type VMMetrics struct {
	VMName      string  `json:"vm_name"`
	Namespace   string  `json:"namespace"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	NetworkRx   uint64  `json:"network_rx"`
	NetworkTx   uint64  `json:"network_tx"`
	DiskRead    uint64  `json:"disk_read"`
	DiskWrite   uint64  `json:"disk_write"`
}

// CreateVMRequest represents a request to create a new virtual machine
type CreateVMRequest struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Template    string            `json:"template"`
	CPU         int               `json:"cpu"`
	Memory      string            `json:"memory"`
	DiskSize    string            `json:"disk_size"`
	NetworkName string            `json:"network_name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// UpdateVMRequest represents a request to update a virtual machine
type UpdateVMRequest struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	CPU         *int              `json:"cpu,omitempty"`
	Memory      *string           `json:"memory,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}
