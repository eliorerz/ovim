package spoke

import (
	"context"
	"time"
)

// Agent represents the main spoke agent interface
type Agent interface {
	// Start starts the agent with the given context
	Start(ctx context.Context) error

	// Stop gracefully stops the agent
	Stop() error

	// GetStatus returns the current agent status
	GetStatus() *StatusReport

	// GetHealth returns the current health status
	GetHealth() *AgentHealth

	// GetProcessor returns the operation processor for direct operation processing
	GetProcessor() OperationProcessor
}

// HubClient defines the interface for communicating with the OVIM hub
type HubClient interface {
	// Connect establishes connection to the hub
	Connect(ctx context.Context) error

	// Disconnect closes the connection to the hub
	Disconnect() error

	// SendStatusReport sends a status report to the hub
	SendStatusReport(ctx context.Context, report *StatusReport) error

	// ReceiveOperations returns a channel for receiving operations from the hub
	ReceiveOperations() <-chan *Operation

	// ReceiveOperation receives a single operation via push notification
	ReceiveOperation(operation *Operation)

	// SendOperationResult sends an operation result back to the hub
	SendOperationResult(ctx context.Context, result *OperationResult) error

	// IsConnected returns true if connected to the hub
	IsConnected() bool

	// GetLastContact returns the time of last successful contact with hub
	GetLastContact() time.Time
}

// VMManager defines the interface for managing VMs on the spoke cluster
type VMManager interface {
	// CreateVM creates a new VM from the given request
	CreateVM(ctx context.Context, req *VMCreateRequest) (*VMStatus, error)

	// DeleteVM deletes the specified VM
	DeleteVM(ctx context.Context, namespace, name string) error

	// StartVM starts the specified VM
	StartVM(ctx context.Context, namespace, name string) error

	// StopVM stops the specified VM
	StopVM(ctx context.Context, namespace, name string) error

	// GetVMStatus returns the status of the specified VM
	GetVMStatus(ctx context.Context, namespace, name string) (*VMStatus, error)

	// ListVMs returns a list of all VMs managed by this agent
	ListVMs(ctx context.Context) ([]VMStatus, error)

	// WatchVMs returns a channel for VM status updates
	WatchVMs(ctx context.Context) (<-chan VMStatus, error)
}

// VDCManager defines the interface for managing VDCs on the spoke cluster
type VDCManager interface {
	// CreateVDC creates a new VDC from the given request
	CreateVDC(ctx context.Context, req *VDCCreateRequest) (*VDCStatus, error)

	// DeleteVDC deletes the specified VDC and all its resources
	DeleteVDC(ctx context.Context, namespace string) error

	// GetVDCStatus returns the status of the specified VDC
	GetVDCStatus(ctx context.Context, namespace string) (*VDCStatus, error)

	// ListVDCs returns a list of all VDCs managed by this agent
	ListVDCs(ctx context.Context) ([]VDCStatus, error)

	// UpdateVDCQuotas updates the resource quotas for the specified VDC
	UpdateVDCQuotas(ctx context.Context, namespace string, cpuQuota, memoryQuota, storageQuota int64) error
}

// MetricsCollector defines the interface for collecting cluster metrics
type MetricsCollector interface {
	// CollectClusterMetrics collects overall cluster metrics
	CollectClusterMetrics(ctx context.Context) (*ClusterMetrics, error)

	// CollectVDCMetrics collects metrics for a specific VDC
	CollectVDCMetrics(ctx context.Context, namespace string) (*ResourceMetrics, error)

	// CollectNodeMetrics collects metrics for all nodes
	CollectNodeMetrics(ctx context.Context) ([]NodeStatus, error)

	// StartPeriodicCollection starts periodic metrics collection
	StartPeriodicCollection(ctx context.Context, interval time.Duration) error
}

// HealthReporter defines the interface for health reporting
type HealthReporter interface {
	// CheckHealth performs a comprehensive health check
	CheckHealth(ctx context.Context) (*AgentHealth, error)

	// CheckComponent checks the health of a specific component
	CheckComponent(ctx context.Context, component string) (*HealthCheck, error)

	// StartPeriodicReporting starts periodic health reporting to the hub
	StartPeriodicReporting(ctx context.Context, interval time.Duration) error

	// RegisterHealthCheck registers a custom health check
	RegisterHealthCheck(name string, check func(ctx context.Context) *HealthCheck)
}

// TemplateManager defines the interface for managing VM templates
type TemplateManager interface {
	// SyncTemplates synchronizes templates from the hub
	SyncTemplates(ctx context.Context, templates []Template) error

	// GetTemplate returns a specific template
	GetTemplate(ctx context.Context, namespace, name string) (*Template, error)

	// ListTemplates returns all cached templates
	ListTemplates(ctx context.Context) ([]Template, error)

	// ValidateTemplate validates if a template is compatible with this cluster
	ValidateTemplate(ctx context.Context, template *Template) error
}

// OperationProcessor defines the interface for processing operations from the hub
type OperationProcessor interface {
	// ProcessOperation processes a single operation from the hub
	ProcessOperation(ctx context.Context, operation *Operation) *OperationResult

	// StartProcessing starts processing operations from the operations channel
	StartProcessing(ctx context.Context, operations <-chan *Operation, results chan<- *OperationResult) error

	// RegisterHandler registers a handler for a specific operation type
	RegisterHandler(operationType OperationType, handler OperationHandler)
}

// OperationHandler defines the interface for handling specific operation types
type OperationHandler interface {
	// Handle processes the operation and returns the result
	Handle(ctx context.Context, operation *Operation) *OperationResult

	// CanHandle returns true if this handler can process the given operation type
	CanHandle(operationType OperationType) bool
}

// LocalAPIServer defines the interface for the local debugging API
type LocalAPIServer interface {
	// Start starts the local API server
	Start(ctx context.Context, addr string) error

	// Stop stops the local API server
	Stop() error

	// RegisterRoutes registers additional routes
	RegisterRoutes(routes map[string]interface{})
}

// ConfigManager defines the interface for configuration management
type ConfigManager interface {
	// Load loads configuration from various sources
	Load() error

	// Get returns a configuration value
	Get(key string) interface{}

	// Set sets a configuration value
	Set(key string, value interface{})

	// Watch watches for configuration changes
	Watch(ctx context.Context) (<-chan ConfigChange, error)
}

// ConfigChange represents a configuration change event
type ConfigChange struct {
	Key      string      `json:"key"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
	Time     time.Time   `json:"time"`
}

// EventRecorder defines the interface for recording events
type EventRecorder interface {
	// RecordEvent records an event
	RecordEvent(eventType, reason, message string, object interface{})

	// RecordWarning records a warning event
	RecordWarning(reason, message string, object interface{})

	// RecordError records an error event
	RecordError(reason, message string, object interface{})
}
