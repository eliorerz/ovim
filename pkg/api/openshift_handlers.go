package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/auth"
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

	// If user has an organization, override the target namespace with organization's namespace
	if userOrgID != "" && h.storage != nil {
		org, err := h.storage.GetOrganization(userOrgID)
		if err != nil {
			klog.Errorf("Failed to get organization %s for user %s (%s): %v", userOrgID, username, userID, err)
			// Don't fail deployment - use provided namespace as fallback
		} else {
			// Check if organization has at least one functioning VDC
			vdcs, err := h.storage.GetVDCsByOrganization(userOrgID)
			if err != nil {
				klog.Errorf("Failed to get VDCs for organization %s: %v", userOrgID, err)
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Failed to validate organization VDCs",
					Message: "Unable to check VDC status for VM deployment",
				})
				return
			}

			// Check if there's at least one functioning VDC (Active phase)
			hasFunctioningVDC := false
			for _, vdc := range vdcs {
				if vdc.Phase == "Active" || vdc.Phase == "Ready" {
					hasFunctioningVDC = true
					break
				}
			}

			if !hasFunctioningVDC {
				klog.Warningf("User %s (%s) attempted to deploy VM but organization %s has no functioning VDCs", username, userID, userOrgID)
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error:   "No functioning Virtual Data Center available",
					Message: "Your organization must have at least one active Virtual Data Center (VDC) before deploying virtual machines. Please contact your organization administrator to create a VDC.",
				})
				return
			}

			// Override target namespace with organization's namespace
			originalNamespace := req.TargetNamespace
			req.TargetNamespace = org.Namespace
			klog.Infof("Overriding VM deployment namespace from %s to organization namespace %s for user %s",
				originalNamespace, req.TargetNamespace, username)
		}
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
