package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/openshift"
	"github.com/eliorerz/ovim-updated/pkg/storage"
)

// OpenShiftHandlers provides handlers for OpenShift integration endpoints
type OpenShiftHandlers struct {
	client  *openshift.Client
	storage storage.Storage
}

// NewOpenShiftHandlers creates a new OpenShift handlers instance
func NewOpenShiftHandlers(client *openshift.Client, storage storage.Storage) *OpenShiftHandlers {
	return &OpenShiftHandlers{
		client:  client,
		storage: storage,
	}
}

// GetOpenShiftTemplates retrieves available VM templates from OpenShift
// @Summary Get OpenShift VM templates
// @Description Retrieve available VM templates from OpenShift cluster
// @Tags openshift
// @Produce json
// @Success 200 {array} openshift.Template
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/openshift/templates [get]
func (h *OpenShiftHandlers) GetOpenShiftTemplates(c *gin.Context) {
	klog.Info("Getting OpenShift templates")

	templates, err := h.client.GetTemplates(c.Request.Context())
	if err != nil {
		klog.Errorf("Failed to get OpenShift templates: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve OpenShift templates",
			Message: err.Error(),
		})
		return
	}

	klog.Infof("Successfully retrieved %d OpenShift templates", len(templates))
	c.JSON(http.StatusOK, templates)
}

// DeployVMFromTemplate deploys a new VM from an OpenShift template
// @Summary Deploy VM from OpenShift template
// @Description Deploy a new virtual machine from an OpenShift template
// @Tags openshift
// @Accept json
// @Produce json
// @Param request body openshift.DeployVMRequest true "VM deployment request"
// @Success 201 {object} openshift.VirtualMachine
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/openshift/vms [post]
func (h *OpenShiftHandlers) DeployVMFromTemplate(c *gin.Context) {
	var req openshift.DeployVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.Errorf("Invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Get user info from context
	userID, username, _, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		klog.Error("User context not found")
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "User context not found",
			Message: "Authentication required",
		})
		return
	}

	// Validate that VDCID is provided
	if req.VDCID == "" {
		klog.Errorf("VDC ID not provided for VM deployment by user %s (%s)", username, userID)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VDC selection required",
			Message: "You must select a Virtual Data Center (VDC) for VM deployment",
		})
		return
	}

	// Get the selected VDC and validate resource availability
	if h.storage != nil {
		vdc, err := h.storage.GetVDC(req.VDCID)
		if err != nil {
			if err == storage.ErrNotFound {
				klog.Errorf("Selected VDC %s not found for user %s (%s)", req.VDCID, username, userID)
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error:   "Selected VDC not found",
					Message: "The selected Virtual Data Center does not exist",
				})
			} else {
				klog.Errorf("Failed to get VDC %s for user %s (%s): %v", req.VDCID, username, userID, err)
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Failed to validate VDC",
					Message: "Unable to verify the selected Virtual Data Center",
				})
			}
			return
		}

		// Handle VDCs with zero storage quota (not yet configured) by providing a reasonable default
		if vdc.StorageQuota == 0 {
			klog.Infof("VDC %s has zero storage quota, setting default to 500GB for validation", req.VDCID)
			vdc.StorageQuota = 500 // 500GB default for validation purposes
		}

		// Verify VDC belongs to user's organization (if user has organization)
		if userOrgID != "" && vdc.OrgID != userOrgID {
			klog.Errorf("User %s (%s) attempted to deploy VM in VDC %s belonging to different organization", username, userID, req.VDCID)
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error:   "Access denied to selected VDC",
				Message: "You can only deploy VMs in VDCs belonging to your organization",
			})
			return
		}

		// Check VDC phase - must be Active or Ready
		if vdc.Phase != "Active" && vdc.Phase != "Ready" {
			klog.Errorf("VDC %s is in phase %s, cannot deploy VM for user %s (%s)", req.VDCID, vdc.Phase, username, userID)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VDC not ready for VM deployment",
				Message: fmt.Sprintf("The selected VDC is in '%s' phase and cannot accept new VMs. Please wait for the VDC to become ready or select a different VDC.", vdc.Phase),
			})
			return
		}

		// Get current VMs in the VDC to calculate resource usage
		allVMs, err := h.storage.ListVMs(vdc.OrgID)
		if err != nil {
			klog.Errorf("Failed to list VMs for resource validation in VDC %s: %v", req.VDCID, err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to validate VDC resources",
				Message: "Unable to check current resource usage in the selected VDC",
			})
			return
		}

		// Calculate current resource usage for this VDC
		usage := vdc.GetResourceUsage(allVMs)
		klog.Infof("VDC %s current usage: CPU %d/%d, Memory %d/%d GB, Storage %d/%d GB, VMs: %d",
			req.VDCID, usage.CPUUsed, usage.CPUQuota, usage.MemoryUsed, usage.MemoryQuota, usage.StorageUsed, usage.StorageQuota, usage.VMCount)

		// Get resource requirements for the new VM from request
		// Use conservative defaults for CPU and memory, but parse actual disk size
		newVMCPU := 1                                           // 1 CPU core (could be enhanced to extract from template)
		newVMMemory := 2                                        // 2 GB memory (could be enhanced to extract from template)
		newVMStorage := models.ParseStorageString(req.DiskSize) // Parse actual disk size from request
		klog.Infof("Parsed disk size '%s' as %d GB for new VM", req.DiskSize, newVMStorage)

		// Validate parsed storage size
		if newVMStorage <= 0 {
			klog.Errorf("Invalid disk size %s for VM deployment in VDC %s by user %s (%s)", req.DiskSize, req.VDCID, username, userID)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid disk size format",
				Message: fmt.Sprintf("Unable to parse disk size '%s'. Please use valid formats like '20Gi', '50GB', etc.", req.DiskSize),
			})
			return
		}

		// Check if VDC has enough available resources
		availableCPU := usage.CPUQuota - usage.CPUUsed
		availableMemory := usage.MemoryQuota - usage.MemoryUsed
		availableStorage := usage.StorageQuota - usage.StorageUsed

		if availableCPU < newVMCPU {
			klog.Warningf("Insufficient CPU in VDC %s for VM deployment: need %d, available %d", req.VDCID, newVMCPU, availableCPU)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Insufficient CPU resources in selected VDC",
				Message: fmt.Sprintf("The selected VDC does not have enough CPU resources. Required: %d cores, Available: %d cores. Current usage: %d/%d cores.", newVMCPU, availableCPU, usage.CPUUsed, usage.CPUQuota),
			})
			return
		}

		if availableMemory < newVMMemory {
			klog.Warningf("Insufficient memory in VDC %s for VM deployment: need %d GB, available %d GB", req.VDCID, newVMMemory, availableMemory)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Insufficient memory resources in selected VDC",
				Message: fmt.Sprintf("The selected VDC does not have enough memory resources. Required: %d GB, Available: %d GB. Current usage: %d/%d GB.", newVMMemory, availableMemory, usage.MemoryUsed, usage.MemoryQuota),
			})
			return
		}

		if availableStorage < newVMStorage {
			klog.Warningf("Insufficient storage in VDC %s for VM deployment: need %d GB, available %d GB", req.VDCID, newVMStorage, availableStorage)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Insufficient storage resources in selected VDC",
				Message: fmt.Sprintf("The selected VDC does not have enough storage resources. Required: %d GB, Available: %d GB. Current usage: %d/%d GB.", newVMStorage, availableStorage, usage.StorageUsed, usage.StorageQuota),
			})
			return
		}

		klog.Infof("VDC %s resource validation passed for VM deployment by user %s (%s): CPU %d/%d, Memory %d/%d GB, Storage %d/%d GB",
			req.VDCID, username, userID, usage.CPUUsed+newVMCPU, usage.CPUQuota, usage.MemoryUsed+newVMMemory, usage.MemoryQuota, usage.StorageUsed+newVMStorage, usage.StorageQuota)

		// Use VDC's workload namespace as target namespace
		req.TargetNamespace = vdc.WorkloadNamespace
		klog.Infof("Using VDC workload namespace %s for VM deployment", req.TargetNamespace)
	}

	klog.Infof("Deploying VM %s from template %s to namespace %s for user %s (%s)",
		req.VMName, req.TemplateName, req.TargetNamespace, username, userID)

	vm, err := h.client.DeployVM(c.Request.Context(), req)
	if err != nil {
		klog.Errorf("Failed to deploy VM: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to deploy VM",
			Message: err.Error(),
		})
		return
	}

	klog.Infof("Successfully deployed VM %s with ID %s in namespace %s for user %s",
		vm.Name, vm.ID, req.TargetNamespace, username)
	c.JSON(http.StatusCreated, vm)
}

// GetOpenShiftVMs retrieves deployed VMs from OpenShift
// @Summary Get OpenShift VMs
// @Description Retrieve deployed virtual machines from OpenShift cluster
// @Tags openshift
// @Produce json
// @Param namespace query string false "Namespace to filter VMs"
// @Success 200 {array} openshift.VirtualMachine
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/openshift/vms [get]
func (h *OpenShiftHandlers) GetOpenShiftVMs(c *gin.Context) {
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}

	klog.Infof("Getting OpenShift VMs from namespace: %s", namespace)

	vms, err := h.client.GetVMs(c.Request.Context(), namespace)
	if err != nil {
		klog.Errorf("Failed to get OpenShift VMs: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve OpenShift VMs",
			Message: err.Error(),
		})
		return
	}

	klog.Infof("Successfully retrieved %d OpenShift VMs", len(vms))
	c.JSON(http.StatusOK, vms)
}

// GetOpenShiftStatus checks the OpenShift connection status
// @Summary Get OpenShift connection status
// @Description Check if OpenShift integration is connected and operational
// @Tags openshift
// @Produce json
// @Success 200 {object} StatusResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/openshift/status [get]
func (h *OpenShiftHandlers) GetOpenShiftStatus(c *gin.Context) {
	klog.Info("Checking OpenShift connection status")

	connected := h.client.IsConnected(c.Request.Context())

	status := StatusResponse{
		Status:  "disconnected",
		Message: "OpenShift integration is not available",
	}

	if connected {
		status.Status = "connected"
		status.Message = "OpenShift integration is operational"
		c.JSON(http.StatusOK, status)
	} else {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "OpenShift connection failed",
			Message: "Unable to connect to OpenShift cluster",
		})
	}
}

// StatusResponse represents a status check response
type StatusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
