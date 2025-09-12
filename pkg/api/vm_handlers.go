package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/eliorerz/ovim-updated/pkg/util"
)

// VMHandlers handles VM-related requests
type VMHandlers struct {
	storage     storage.Storage
	provisioner kubevirt.VMProvisioner
	k8sClient   client.Client
}

// NewVMHandlers creates a new VM handlers instance
func NewVMHandlers(storage storage.Storage, provisioner kubevirt.VMProvisioner, k8sClient client.Client) *VMHandlers {
	return &VMHandlers{
		storage:     storage,
		provisioner: provisioner,
		k8sClient:   k8sClient,
	}
}

// List handles listing VMs
func (h *VMHandlers) List(c *gin.Context) {
	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	var orgFilter string
	// Filter VMs based on user role
	if role == models.RoleSystemAdmin {
		// System admin can see all VMs
		orgFilter = ""
	} else if role == models.RoleOrgAdmin || role == models.RoleOrgUser {
		// Org admin and users can only see VMs from their organization
		if userOrgID == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "User not associated with any organization"})
			return
		}
		orgFilter = userOrgID
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	vms, err := h.storage.ListVMs(orgFilter)
	if err != nil {
		klog.Errorf("Failed to list VMs for user %s (%s): %v", username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list VMs"})
		return
	}

	// For org users, filter VMs to only show their own
	if role == models.RoleOrgUser {
		userVMs := make([]*models.VirtualMachine, 0)
		for _, vm := range vms {
			if vm.OwnerID == userID {
				userVMs = append(userVMs, vm)
			}
		}
		vms = userVMs
	}

	klog.V(6).Infof("Listed %d VMs for user %s (%s)", len(vms), username, userID)
	c.JSON(http.StatusOK, gin.H{
		"vms":   vms,
		"total": len(vms),
	})
}

// Create handles creating a new VM
func (h *VMHandlers) Create(c *gin.Context) {
	var req models.CreateVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid create VM request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check if user can create VMs (all authenticated users can create VMs in their org)
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin && role != models.RoleOrgUser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to create VM"})
		return
	}

	// Ensure user is associated with an organization
	if userOrgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User not associated with any organization"})
		return
	}

	// Verify the template exists
	template, err := h.storage.GetTemplate(req.TemplateID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Template not found"})
			return
		}
		klog.Errorf("Failed to verify template %s: %v", req.TemplateID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify template"})
		return
	}

	// Find a VDC in the user's organization using CRDs
	var selectedVDC *ovimv1.VirtualDataCenter

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List VDCs in the organization namespace
	vdcList := &ovimv1.VirtualDataCenterList{}
	orgNamespace := fmt.Sprintf("org-%s", userOrgID)
	if err := h.k8sClient.List(ctx, vdcList, client.InNamespace(orgNamespace)); err != nil {
		klog.Errorf("Failed to list VDCs for organization %s: %v", userOrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find VDC"})
		return
	}

	if len(vdcList.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No VDC available in organization"})
		return
	}

	// Use the first active VDC
	for _, vdcItem := range vdcList.Items {
		if vdcItem.Status.Phase == ovimv1.VirtualDataCenterPhaseActive && vdcItem.Status.Namespace != "" {
			selectedVDC = &vdcItem
			break
		}
	}

	if selectedVDC == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active VDC available in organization"})
		return
	}

	// Generate VM ID
	vmID, err := util.GenerateID(16)
	if err != nil {
		klog.Errorf("Failed to generate VM ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate VM ID"})
		return
	}
	vmID = "vm-" + vmID

	// Set VM specifications from request or use template defaults
	cpu := req.CPU
	if cpu <= 0 {
		cpu = template.CPU
	}

	memory := req.Memory
	if memory == "" {
		memory = template.Memory
	}

	diskSize := req.DiskSize
	if diskSize == "" {
		diskSize = template.DiskSize
	}

	// Validate VM specs and prepare VDC model - CRD-based only
	if selectedVDC == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No VDC found for organization"})
		return
	}

	// CRD-based validation and setup
	if err := h.validateVMLimitRangeCRD(selectedVDC, cpu, memory); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vdcID := selectedVDC.Name
	vdcForProvisioner := &models.VirtualDataCenter{
		ID:                selectedVDC.Name,
		Name:              selectedVDC.Spec.DisplayName,
		OrgID:             selectedVDC.Spec.OrganizationRef,
		WorkloadNamespace: selectedVDC.Status.Namespace,
	}

	// Create VM model
	vm := &models.VirtualMachine{
		ID:         vmID,
		Name:       req.Name,
		OrgID:      userOrgID,
		VDCID:      &vdcID,
		TemplateID: req.TemplateID,
		OwnerID:    userID,
		Status:     models.VMStatusPending,
		CPU:        cpu,
		Memory:     memory,
		DiskSize:   diskSize,
		IPAddress:  "", // Will be assigned during deployment
		Metadata: map[string]string{
			"template_name": template.Name,
			"os_type":       template.OSType,
			"os_version":    template.OSVersion,
			"created_by":    username,
		},
	}

	// Create VM in database first
	if err := h.storage.CreateVM(vm); err != nil {
		if err == storage.ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "VM already exists"})
			return
		}
		klog.Errorf("Failed to create VM in storage: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create VM"})
		return
	}

	// Create VM in KubeVirt cluster
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	if err := h.provisioner.CreateVM(ctx2, vm, vdcForProvisioner, template); err != nil {
		klog.Errorf("Failed to provision VM %s in KubeVirt: %v", vm.ID, err)

		// Update VM status to error in database
		vm.Status = models.VMStatusError
		if updateErr := h.storage.UpdateVM(vm); updateErr != nil {
			klog.Errorf("Failed to update VM %s status to error: %v", vm.ID, updateErr)
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to provision VM in cluster"})
		return
	}

	// Update VM status to provisioning
	vm.Status = models.VMStatusProvisioning
	if err := h.storage.UpdateVM(vm); err != nil {
		klog.Errorf("Failed to update VM %s status to provisioning: %v", vm.ID, err)
		// Don't fail the request - VM was created successfully
	}

	vdcName := selectedVDC.Spec.DisplayName
	klog.Infof("VM %s (%s) created and provisioned in VDC %s (org %s) by user %s (%s)", vm.Name, vm.ID, vdcName, userOrgID, username, userID)

	c.JSON(http.StatusCreated, vm)
}

// Get handles getting a specific VM
func (h *VMHandlers) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VM ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	vm, err := h.storage.GetVM(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
			return
		}
		klog.Errorf("Failed to get VM %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VM"})
		return
	}

	// Check access permissions
	if role == models.RoleSystemAdmin {
		// System admin can access any VM
	} else if role == models.RoleOrgAdmin {
		// Org admin can access VMs in their organization
		if userOrgID == "" || userOrgID != vm.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VM"})
			return
		}
	} else if role == models.RoleOrgUser {
		// Org user can only access their own VMs
		if userOrgID == "" || userOrgID != vm.OrgID || userID != vm.OwnerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VM"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	c.JSON(http.StatusOK, vm)
}

// GetStatus handles getting VM status from KubeVirt cluster
func (h *VMHandlers) GetStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VM ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get VM from database to check permissions
	vm, err := h.storage.GetVM(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
			return
		}
		klog.Errorf("Failed to get VM %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VM"})
		return
	}

	// Check access permissions
	if role == models.RoleSystemAdmin {
		// System admin can access any VM
	} else if role == models.RoleOrgAdmin {
		// Org admin can access VMs in their organization
		if userOrgID == "" || userOrgID != vm.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VM"})
			return
		}
	} else if role == models.RoleOrgUser {
		// Org user can only access their own VMs
		if userOrgID == "" || userOrgID != vm.OrgID || userID != vm.OwnerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VM"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Get VDC to determine namespace
	if vm.VDCID == nil {
		klog.Errorf("VM %s has no VDC ID", vm.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "VM has no VDC association"})
		return
	}
	vdc, err := h.storage.GetVDC(*vm.VDCID)
	if err != nil {
		klog.Errorf("Failed to get VDC %s for VM %s: %v", *vm.VDCID, vm.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Get VM status from KubeVirt
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := h.provisioner.GetVMStatus(ctx, vm.ID, vdc.WorkloadNamespace)
	if err != nil {
		klog.Errorf("Failed to get VM %s status from KubeVirt: %v", vm.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VM status from cluster"})
		return
	}

	// Update VM status and IP in database if changed
	clusterStatus := mapKubeVirtStatusToModel(status.Phase, status.Ready)
	if vm.Status != clusterStatus || (status.IPAddress != "" && vm.IPAddress != status.IPAddress) {
		vm.Status = clusterStatus
		if status.IPAddress != "" {
			vm.IPAddress = status.IPAddress
		}
		if err := h.storage.UpdateVM(vm); err != nil {
			klog.Errorf("Failed to update VM %s status in database: %v", vm.ID, err)
			// Don't fail the request - we can still return the current status
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"vm_id":      vm.ID,
		"status":     vm.Status,
		"ip_address": vm.IPAddress,
		"cluster":    status,
	})
}

// UpdatePower handles updating VM power state
func (h *VMHandlers) UpdatePower(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VM ID required"})
		return
	}

	var req models.UpdateVMPowerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid update VM power request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate action
	validActions := map[string]bool{
		"start":   true,
		"stop":    true,
		"restart": true,
	}
	if !validActions[req.Action] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action. Must be start, stop, or restart"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get existing VM
	vm, err := h.storage.GetVM(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
			return
		}
		klog.Errorf("Failed to get VM %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VM"})
		return
	}

	// Check access permissions
	if role == models.RoleSystemAdmin {
		// System admin can control any VM
	} else if role == models.RoleOrgAdmin {
		// Org admin can control VMs in their organization
		if userOrgID == "" || userOrgID != vm.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VM"})
			return
		}
	} else if role == models.RoleOrgUser {
		// Org user can only control their own VMs
		if userOrgID == "" || userOrgID != vm.OrgID || userID != vm.OwnerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VM"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Get VDC to determine namespace
	if vm.VDCID == nil {
		klog.Errorf("VM %s has no VDC ID", vm.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "VM has no VDC association"})
		return
	}
	vdc, err := h.storage.GetVDC(*vm.VDCID)
	if err != nil {
		klog.Errorf("Failed to get VDC %s for VM %s: %v", *vm.VDCID, vm.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Perform power action on KubeVirt cluster
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var newStatus string
	switch req.Action {
	case "start":
		if vm.Status == models.VMStatusRunning {
			c.JSON(http.StatusBadRequest, gin.H{"error": "VM is already running"})
			return
		}
		// Allow starting VMs in pending or stopped state
		if err := h.provisioner.StartVM(ctx, vm.ID, vdc.WorkloadNamespace); err != nil {
			klog.Errorf("Failed to start VM %s in KubeVirt: %v", vm.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start VM in cluster"})
			return
		}
		newStatus = models.VMStatusRunning

	case "stop":
		if vm.Status == models.VMStatusStopped {
			c.JSON(http.StatusBadRequest, gin.H{"error": "VM is already stopped"})
			return
		}
		if err := h.provisioner.StopVM(ctx, vm.ID, vdc.WorkloadNamespace); err != nil {
			klog.Errorf("Failed to stop VM %s in KubeVirt: %v", vm.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop VM in cluster"})
			return
		}
		newStatus = models.VMStatusStopped
		vm.IPAddress = "" // Clear IP when stopped

	case "restart":
		if vm.Status != models.VMStatusRunning {
			c.JSON(http.StatusBadRequest, gin.H{"error": "VM must be running to restart"})
			return
		}
		if err := h.provisioner.RestartVM(ctx, vm.ID, vdc.WorkloadNamespace); err != nil {
			klog.Errorf("Failed to restart VM %s in KubeVirt: %v", vm.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restart VM in cluster"})
			return
		}
		newStatus = models.VMStatusRunning
	}

	// Update VM status in database
	vm.Status = newStatus
	if err := h.storage.UpdateVM(vm); err != nil {
		klog.Errorf("Failed to update VM %s power state in database: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update VM power state"})
		return
	}

	klog.Infof("VM %s (%s) power action '%s' performed by user %s (%s)", vm.Name, vm.ID, req.Action, username, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "VM power state updated successfully",
		"action":  req.Action,
		"status":  vm.Status,
	})
}

// Delete handles deleting a VM
func (h *VMHandlers) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VM ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get existing VM
	vm, err := h.storage.GetVM(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
			return
		}
		klog.Errorf("Failed to get VM %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VM"})
		return
	}

	// Check access permissions
	if role == models.RoleSystemAdmin {
		// System admin can delete any VM
	} else if role == models.RoleOrgAdmin {
		// Org admin can delete VMs in their organization
		if userOrgID == "" || userOrgID != vm.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VM"})
			return
		}
	} else if role == models.RoleOrgUser {
		// Org user can only delete their own VMs
		if userOrgID == "" || userOrgID != vm.OrgID || userID != vm.OwnerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VM"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Get VDC to determine namespace
	if vm.VDCID == nil {
		klog.Errorf("VM %s has no VDC ID", vm.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "VM has no VDC association"})
		return
	}
	vdc, err := h.storage.GetVDC(*vm.VDCID)
	if err != nil {
		klog.Errorf("Failed to get VDC %s for VM %s: %v", *vm.VDCID, vm.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Set VM status to deleting before actual deletion
	vm.Status = models.VMStatusDeleting
	if err := h.storage.UpdateVM(vm); err != nil {
		klog.Errorf("Failed to update VM %s status to deleting: %v", id, err)
		// Continue with deletion anyway
	}

	// Delete VM from KubeVirt cluster first
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.provisioner.DeleteVM(ctx, vm.ID, vdc.WorkloadNamespace); err != nil {
		klog.Errorf("Failed to delete VM %s from KubeVirt: %v", vm.ID, err)
		// Update status back to error instead of continuing
		vm.Status = models.VMStatusError
		if updateErr := h.storage.UpdateVM(vm); updateErr != nil {
			klog.Errorf("Failed to update VM %s status to error: %v", vm.ID, updateErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete VM from cluster"})
		return
	}

	// Delete VM from database
	if err := h.storage.DeleteVM(id); err != nil {
		klog.Errorf("Failed to delete VM %s from database: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete VM from database"})
		return
	}

	klog.Infof("VM %s (%s) deleted from cluster and database by user %s (%s)", vm.Name, vm.ID, username, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "VM deleted successfully",
	})
}

// mapKubeVirtStatusToModel maps KubeVirt VM phase and ready status to our model status
func mapKubeVirtStatusToModel(phase string, ready bool) string {
	switch phase {
	case "Pending", "Scheduling":
		return models.VMStatusProvisioning
	case "Running":
		if ready {
			return models.VMStatusRunning
		}
		return models.VMStatusProvisioning
	case "Succeeded", "Stopped":
		return models.VMStatusStopped
	case "Failed":
		return models.VMStatusError
	default:
		if ready {
			return models.VMStatusRunning
		}
		return models.VMStatusPending
	}
}

// validateVMLimitRangeCRD validates VM CPU and memory specifications against VDC CRD LimitRange constraints
func (h *VMHandlers) validateVMLimitRangeCRD(vdc *ovimv1.VirtualDataCenter, cpu int, memory string) error {
	// Skip validation if VDC has no LimitRange defined
	if vdc.Spec.LimitRange == nil {
		klog.V(6).Infof("No LimitRange defined for VDC %s, allowing VM creation without constraints", vdc.Name)
		return nil
	}

	limitRange := vdc.Spec.LimitRange

	// Parse memory string to GB for comparison
	memoryGB := models.ParseMemoryString(memory)

	// Validate CPU constraints
	if limitRange.MinCpu > 0 && cpu < limitRange.MinCpu {
		return fmt.Errorf("VM CPU (%d cores) is below VDC minimum limit (%d cores)", cpu, limitRange.MinCpu)
	}
	if limitRange.MaxCpu > 0 && cpu > limitRange.MaxCpu {
		return fmt.Errorf("VM CPU (%d cores) exceeds VDC maximum limit (%d cores)", cpu, limitRange.MaxCpu)
	}

	// Validate memory constraints
	if limitRange.MinMemory > 0 && memoryGB < limitRange.MinMemory {
		return fmt.Errorf("VM memory (%dGB) is below VDC minimum limit (%dGB)", memoryGB, limitRange.MinMemory)
	}
	if limitRange.MaxMemory > 0 && memoryGB > limitRange.MaxMemory {
		return fmt.Errorf("VM memory (%dGB) exceeds VDC maximum limit (%dGB)", memoryGB, limitRange.MaxMemory)
	}

	klog.V(6).Infof("VM specs validated successfully against VDC %s LimitRange: CPU=%d (limits: %d-%d), Memory=%dGB (limits: %d-%d)",
		vdc.Name, cpu, limitRange.MinCpu, limitRange.MaxCpu, memoryGB, limitRange.MinMemory, limitRange.MaxMemory)

	return nil
}
