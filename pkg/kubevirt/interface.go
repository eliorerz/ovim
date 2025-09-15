package kubevirt

import (
	"context"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// VMProvisioner defines the interface for VM provisioning operations
type VMProvisioner interface {
	// CreateVM creates a new virtual machine in the KubeVirt cluster
	CreateVM(ctx context.Context, vm *models.VirtualMachine, vdc *models.VirtualDataCenter, template *models.Template) error

	// GetVMStatus retrieves the current status of a virtual machine
	GetVMStatus(ctx context.Context, vmID, namespace string) (*VMStatus, error)

	// StartVM starts a stopped virtual machine
	StartVM(ctx context.Context, vmID, namespace string) error

	// StopVM stops a running virtual machine
	StopVM(ctx context.Context, vmID, namespace string) error

	// RestartVM restarts a virtual machine
	RestartVM(ctx context.Context, vmID, namespace string) error

	// DeleteVM deletes a virtual machine and its associated resources
	DeleteVM(ctx context.Context, vmID, namespace string) error

	// GetVMIPAddress retrieves the IP address of a running virtual machine
	GetVMIPAddress(ctx context.Context, vmID, namespace string) (string, error)

	// GetVMConsoleURL retrieves the console access URL for a virtual machine
	GetVMConsoleURL(ctx context.Context, vmID, namespace string) (string, error)

	// CheckConnection verifies connectivity to the KubeVirt cluster
	CheckConnection(ctx context.Context) error
}

// VMStatus represents the current status of a virtual machine
type VMStatus struct {
	Phase       string            `json:"phase"`
	Ready       bool              `json:"ready"`
	IPAddress   string            `json:"ip_address,omitempty"`
	NodeName    string            `json:"node_name,omitempty"`
	Conditions  []VMCondition     `json:"conditions,omitempty"`
	Interfaces  []VMInterface     `json:"interfaces,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// VMCondition represents a condition of the virtual machine
type VMCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// VMInterface represents a network interface of the virtual machine
type VMInterface struct {
	Name      string `json:"name"`
	IP        string `json:"ip,omitempty"`
	MAC       string `json:"mac,omitempty"`
	Network   string `json:"network,omitempty"`
	Interface string `json:"interface,omitempty"`
}
