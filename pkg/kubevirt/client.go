package kubevirt

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// Client implements the VMProvisioner interface using KubeVirt
type Client struct {
	dynamicClient dynamic.Interface
	client        client.Client
}

var (
	// KubeVirt GVRs
	vmGVR = schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachines",
	}
	vmiGVR = schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "virtualmachineinstances",
	}
)

// NewClient creates a new KubeVirt client
func NewClient(config *rest.Config, k8sClient client.Client) (*Client, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{
		dynamicClient: dynamicClient,
		client:        k8sClient,
	}, nil
}

// CreateVM creates a new virtual machine in the KubeVirt cluster
func (c *Client) CreateVM(ctx context.Context, vm *models.VirtualMachine, vdc *models.VirtualDataCenter, template *models.Template) error {
	logger := log.FromContext(ctx).WithValues("vm", vm.Name, "vdc", vdc.WorkloadNamespace)

	// Create VirtualMachine manifest
	vmManifest := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubevirt.io/v1",
			"kind":       "VirtualMachine",
			"metadata": map[string]interface{}{
				"name":      vm.Name,
				"namespace": vdc.WorkloadNamespace,
				"labels": map[string]interface{}{
					"ovim.io/vm":                   vm.Name,
					"ovim.io/vdc":                  vdc.ID,
					"ovim.io/organization":         vdc.OrgID,
					"ovim.io/template":             template.ID,
					"app.kubernetes.io/managed-by": "ovim",
				},
				"annotations": map[string]interface{}{
					"ovim.io/vm-id":         vm.ID,
					"ovim.io/created-by":    "ovim-controller",
					"ovim.io/template-name": template.Name,
				},
			},
			"spec": map[string]interface{}{
				"running": vm.Status == "running",
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"ovim.io/vm": vm.Name,
						},
					},
					"spec": map[string]interface{}{
						"domain": map[string]interface{}{
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{
									"memory": vm.Memory, // Memory is already a string like "4Gi"
									"cpu":    fmt.Sprintf("%d", vm.CPU),
								},
							},
							"devices": map[string]interface{}{
								"disks": []interface{}{
									map[string]interface{}{
										"name": "containerdisk",
										"disk": map[string]interface{}{
											"bus": "virtio",
										},
									},
									map[string]interface{}{
										"name": "cloudinitdisk",
										"disk": map[string]interface{}{
											"bus": "virtio",
										},
									},
								},
								"interfaces": []interface{}{
									map[string]interface{}{
										"name":   "default",
										"bridge": map[string]interface{}{},
									},
								},
							},
						},
						"networks": []interface{}{
							map[string]interface{}{
								"name": "default",
								"pod":  map[string]interface{}{},
							},
						},
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "containerdisk",
								"containerDisk": map[string]interface{}{
									"image": template.ImageURL,
								},
							},
							map[string]interface{}{
								"name": "cloudinitdisk",
								"cloudInitNoCloud": map[string]interface{}{
									"userData": generateCloudInitUserData(vm),
								},
							},
						},
					},
				},
			},
		},
	}

	// Create the VirtualMachine
	_, err := c.dynamicClient.Resource(vmGVR).Namespace(vdc.WorkloadNamespace).Create(ctx, vmManifest, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "failed to create VirtualMachine")
		return fmt.Errorf("failed to create VirtualMachine: %w", err)
	}

	logger.Info("VirtualMachine created successfully")
	return nil
}

// GetVMStatus retrieves the current status of a virtual machine
func (c *Client) GetVMStatus(ctx context.Context, vmID, namespace string) (*VMStatus, error) {
	logger := log.FromContext(ctx).WithValues("vm", vmID, "namespace", namespace)

	// Find VirtualMachine by ovim.io/vm-id annotation
	vm, err := c.findVMByID(ctx, vmID, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get VirtualMachine: %w", err)
	}

	// Extract status from VM
	status := &VMStatus{
		Phase:       "Unknown",
		Ready:       false,
		Annotations: make(map[string]string),
	}

	// Get running state
	if running, found, err := unstructured.NestedBool(vm.Object, "spec", "running"); err == nil && found {
		if running {
			status.Phase = "Starting"
		} else {
			status.Phase = "Stopped"
		}
	}

	// Get annotations
	if annotations, found, err := unstructured.NestedStringMap(vm.Object, "metadata", "annotations"); err == nil && found {
		status.Annotations = annotations
	}

	// Get the actual VM name for VMI lookup
	vmName, found, err := unstructured.NestedString(vm.Object, "metadata", "name")
	if err != nil || !found {
		return nil, fmt.Errorf("failed to get VirtualMachine name")
	}

	// Try to get VirtualMachineInstance for more detailed status
	vmi, err := c.dynamicClient.Resource(vmiGVR).Namespace(namespace).Get(ctx, vmName, metav1.GetOptions{})
	if err == nil {
		// VMI exists, get detailed status
		if phase, found, err := unstructured.NestedString(vmi.Object, "status", "phase"); err == nil && found {
			status.Phase = phase
			status.Ready = phase == "Running"
		}

		// Get node name
		if nodeName, found, err := unstructured.NestedString(vmi.Object, "status", "nodeName"); err == nil && found {
			status.NodeName = nodeName
		}

		// Get interfaces and IP addresses
		if interfaces, found, err := unstructured.NestedSlice(vmi.Object, "status", "interfaces"); err == nil && found {
			for _, iface := range interfaces {
				if ifaceMap, ok := iface.(map[string]interface{}); ok {
					vmIface := VMInterface{}
					if name, found, err := unstructured.NestedString(ifaceMap, "name"); err == nil && found {
						vmIface.Name = name
					}
					if ip, found, err := unstructured.NestedString(ifaceMap, "ipAddress"); err == nil && found {
						vmIface.IP = ip
						if status.IPAddress == "" {
							status.IPAddress = ip
						}
					}
					if mac, found, err := unstructured.NestedString(ifaceMap, "mac"); err == nil && found {
						vmIface.MAC = mac
					}
					status.Interfaces = append(status.Interfaces, vmIface)
				}
			}
		}

		// Get conditions
		if conditions, found, err := unstructured.NestedSlice(vmi.Object, "status", "conditions"); err == nil && found {
			for _, cond := range conditions {
				if condMap, ok := cond.(map[string]interface{}); ok {
					vmCond := VMCondition{}
					if condType, found, err := unstructured.NestedString(condMap, "type"); err == nil && found {
						vmCond.Type = condType
					}
					if condStatus, found, err := unstructured.NestedString(condMap, "status"); err == nil && found {
						vmCond.Status = condStatus
					}
					if reason, found, err := unstructured.NestedString(condMap, "reason"); err == nil && found {
						vmCond.Reason = reason
					}
					status.Conditions = append(status.Conditions, vmCond)
				}
			}
		}
	}

	logger.V(1).Info("Retrieved VM status", "phase", status.Phase, "ready", status.Ready)
	return status, nil
}

// StartVM starts a stopped virtual machine
func (c *Client) StartVM(ctx context.Context, vmID, namespace string) error {
	return c.updateVMRunningState(ctx, vmID, namespace, true)
}

// StopVM stops a running virtual machine
func (c *Client) StopVM(ctx context.Context, vmID, namespace string) error {
	return c.updateVMRunningState(ctx, vmID, namespace, false)
}

// RestartVM restarts a virtual machine
func (c *Client) RestartVM(ctx context.Context, vmID, namespace string) error {
	logger := log.FromContext(ctx).WithValues("vm", vmID, "namespace", namespace)

	// Stop the VM first
	if err := c.StopVM(ctx, vmID, namespace); err != nil {
		return fmt.Errorf("failed to stop VM for restart: %w", err)
	}

	// Wait a moment for graceful shutdown
	time.Sleep(2 * time.Second)

	// Start the VM again
	if err := c.StartVM(ctx, vmID, namespace); err != nil {
		return fmt.Errorf("failed to start VM after restart: %w", err)
	}

	logger.Info("VM restarted successfully")
	return nil
}

// DeleteVM deletes a virtual machine and its associated resources
func (c *Client) DeleteVM(ctx context.Context, vmID, namespace string) error {
	logger := log.FromContext(ctx).WithValues("vm", vmID, "namespace", namespace)

	// Find VirtualMachine by ovim.io/vm-id annotation to get its actual name
	vm, err := c.findVMByID(ctx, vmID, namespace)
	if err != nil {
		return fmt.Errorf("failed to find VirtualMachine: %w", err)
	}

	// Get the actual VM name from metadata
	vmName, found, err := unstructured.NestedString(vm.Object, "metadata", "name")
	if err != nil || !found {
		return fmt.Errorf("failed to get VirtualMachine name")
	}

	// Delete the VirtualMachine (this will also delete the VMI)
	err = c.dynamicClient.Resource(vmGVR).Namespace(namespace).Delete(ctx, vmName, metav1.DeleteOptions{})
	if err != nil {
		logger.Error(err, "failed to delete VirtualMachine")
		return fmt.Errorf("failed to delete VirtualMachine: %w", err)
	}

	logger.Info("VirtualMachine deleted successfully")
	return nil
}

// GetVMIPAddress retrieves the IP address of a running virtual machine
func (c *Client) GetVMIPAddress(ctx context.Context, vmID, namespace string) (string, error) {
	status, err := c.GetVMStatus(ctx, vmID, namespace)
	if err != nil {
		return "", err
	}
	return status.IPAddress, nil
}

// CheckConnection verifies connectivity to the KubeVirt cluster
func (c *Client) CheckConnection(ctx context.Context) error {
	// Try to list VirtualMachines in default namespace to check connectivity
	_, err := c.dynamicClient.Resource(vmGVR).Namespace("default").List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to connect to KubeVirt cluster: %w", err)
	}
	return nil
}

// updateVMRunningState updates the running state of a VirtualMachine
func (c *Client) updateVMRunningState(ctx context.Context, vmID, namespace string, running bool) error {
	logger := log.FromContext(ctx).WithValues("vm", vmID, "namespace", namespace, "running", running)

	// Find VirtualMachine by ovim.io/vm-id annotation
	vm, err := c.findVMByID(ctx, vmID, namespace)
	if err != nil {
		return fmt.Errorf("failed to get VirtualMachine: %w", err)
	}

	// Update the running field
	if err := unstructured.SetNestedField(vm.Object, running, "spec", "running"); err != nil {
		return fmt.Errorf("failed to set running field: %w", err)
	}

	// Update the VM
	_, err = c.dynamicClient.Resource(vmGVR).Namespace(namespace).Update(ctx, vm, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "failed to update VirtualMachine running state")
		return fmt.Errorf("failed to update VirtualMachine: %w", err)
	}

	action := "stopped"
	if running {
		action = "started"
	}
	logger.Info("VirtualMachine " + action + " successfully")
	return nil
}

// GetVMConsoleURL retrieves the console access URL for a virtual machine
func (c *Client) GetVMConsoleURL(ctx context.Context, vmID, namespace string) (string, error) {
	logger := log.FromContext(ctx).WithValues("vmID", vmID, "namespace", namespace)
	logger.Info("Getting console URL for VirtualMachine")

	// Find VirtualMachine by ovim.io/vm-id annotation to get its actual name
	vm, err := c.findVMByID(ctx, vmID, namespace)
	if err != nil {
		return "", fmt.Errorf("failed to find VirtualMachine: %w", err)
	}

	// Get the actual VM name
	vmName, found, err := unstructured.NestedString(vm.Object, "metadata", "name")
	if err != nil || !found {
		return "", fmt.Errorf("failed to get VirtualMachine name")
	}

	// Get the VMI (VirtualMachineInstance) to check if VM is running
	vmi, err := c.dynamicClient.Resource(vmiGVR).Namespace(namespace).Get(ctx, vmName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "failed to get VirtualMachineInstance")
		return "", fmt.Errorf("VM is not running or does not exist")
	}

	// Check if VMI is running
	status, found, err := unstructured.NestedString(vmi.Object, "status", "phase")
	if err != nil || !found {
		return "", fmt.Errorf("failed to get VMI status")
	}

	if status != "Running" {
		return "", fmt.Errorf("VM must be running to access console (current status: %s)", status)
	}

	// For KubeVirt, we need to construct a console URL
	// This typically involves the cluster's console URL and the VM's details
	// In a real implementation, you would:
	// 1. Get the cluster's console URL from the KubeVirt configuration
	// 2. Generate a secure token for console access
	// 3. Return a URL that points to the VNC/console proxy

	// For now, we'll return a URL that can be used with kubectl port-forward or similar
	// In production, this should be replaced with proper KubeVirt console integration
	consoleURL := fmt.Sprintf("/k8s/api/v1/namespaces/%s/services/virt-console-proxy:8001/proxy/vm/%s/console", namespace, vmID)

	logger.Info("Generated console URL for VirtualMachine", "url", consoleURL)
	return consoleURL, nil
}

// findVMByID finds a VirtualMachine by its ovim.io/vm-id annotation
func (c *Client) findVMByID(ctx context.Context, vmID, namespace string) (*unstructured.Unstructured, error) {
	logger := log.FromContext(ctx).WithValues("vmID", vmID, "namespace", namespace)

	// List all VirtualMachines in the namespace
	vmList, err := c.dynamicClient.Resource(vmGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list VirtualMachines: %w", err)
	}

	// Find VM with matching ovim.io/vm-id annotation
	for _, vm := range vmList.Items {
		if annotations, found, err := unstructured.NestedStringMap(vm.Object, "metadata", "annotations"); err == nil && found {
			if annotations["ovim.io/vm-id"] == vmID {
				logger.V(1).Info("Found VirtualMachine by ovim.io/vm-id annotation", "vmName", vm.GetName())
				return &vm, nil
			}
		}
	}

	return nil, fmt.Errorf("VirtualMachine with ovim.io/vm-id=%s not found in namespace %s", vmID, namespace)
}

// generateCloudInitUserData generates cloud-init user data for VM initialization
func generateCloudInitUserData(vm *models.VirtualMachine) string {
	userData := `#cloud-config
hostname: ` + vm.Name + `
users:
  - name: ovim
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... # Default OVIM key
runcmd:
  - systemctl enable ssh
  - systemctl start ssh
`
	return userData
}
