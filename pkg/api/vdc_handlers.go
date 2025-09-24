package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/auth"
	"github.com/eliorerz/ovim-updated/pkg/models"
	"github.com/eliorerz/ovim-updated/pkg/openshift"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"github.com/eliorerz/ovim-updated/pkg/util"
)

// VDCHandlers handles VDC-related requests
type VDCHandlers struct {
	storage         storage.Storage
	k8sClient       client.Client
	openShiftClient *openshift.Client
	eventRecorder   *EventRecorder
}

// NewVDCHandlers creates a new VDC handlers instance
func NewVDCHandlers(storage storage.Storage, k8sClient client.Client, openShiftClient *openshift.Client) *VDCHandlers {
	return &VDCHandlers{
		storage:         storage,
		k8sClient:       k8sClient,
		openShiftClient: openShiftClient,
	}
}

// SetEventRecorder sets the event recorder for this handler
func (h *VDCHandlers) SetEventRecorder(recorder *EventRecorder) {
	h.eventRecorder = recorder
}

// List handles listing VDCs
func (h *VDCHandlers) List(c *gin.Context) {
	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get query parameters for filtering
	zoneFilter := c.Query("zone_id")

	var orgFilter string
	// Filter VDCs based on user role
	if role == models.RoleSystemAdmin {
		// System admin can see all VDCs
		orgFilter = ""
	} else if role == models.RoleOrgAdmin || role == models.RoleOrgUser {
		// Org admin and users can only see VDCs from their organization
		if userOrgID == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "User not associated with any organization"})
			return
		}
		orgFilter = userOrgID
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	vdcs, err := h.storage.ListVDCs(orgFilter)
	if err != nil {
		klog.Errorf("Failed to list VDCs for user %s (%s): %v", username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list VDCs"})
		return
	}

	// Apply zone filtering if specified
	if zoneFilter != "" {
		var filteredVDCs []*models.VirtualDataCenter
		for _, vdc := range vdcs {
			if vdc.ZoneID != nil && *vdc.ZoneID == zoneFilter {
				filteredVDCs = append(filteredVDCs, vdc)
			}
		}
		vdcs = filteredVDCs
	}

	klog.V(6).Infof("Listed %d VDCs for user %s (%s)", len(vdcs), username, userID)
	c.JSON(http.StatusOK, gin.H{
		"vdcs":  vdcs,
		"total": len(vdcs),
	})
}

// Get handles getting a specific VDC
func (h *VDCHandlers) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Check access permissions
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this VDC"})
			return
		}
	}

	c.JSON(http.StatusOK, vdc)
}

// Create handles creating a new VDC
func (h *VDCHandlers) Create(c *gin.Context) {
	var req models.CreateVDCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.Errorf("Invalid create VDC request JSON binding failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request format: %v", err)})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - only system admin and org admin can create VDCs
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to create VDC"})
		return
	}

	// For org admin, ensure they can only create VDCs in their own organization
	if role == models.RoleOrgAdmin {
		if userOrgID == "" || userOrgID != req.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only create VDCs in your own organization"})
			return
		}
	}

	// Verify that the organization exists
	_, err := h.storage.GetOrganization(req.OrgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Organization not found"})
			return
		}
		klog.Errorf("Failed to verify organization %s: %v", req.OrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify organization"})
		return
	}

	// Verify that the zone exists and is available
	zone, err := h.storage.GetZone(req.ZoneID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Zone not found"})
			return
		}
		klog.Errorf("Failed to verify zone %s: %v", req.ZoneID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify zone"})
		return
	}

	// Check if zone is available
	if zone.Status != models.ZoneStatusAvailable {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Zone '%s' is not available (status: %s)", zone.Name, zone.Status),
		})
		return
	}

	// For org admins, verify they have access to this zone
	if role == models.RoleOrgAdmin {
		// Check if organization has access to this zone
		zoneAccess, err := h.storage.GetOrganizationZoneAccess(req.OrgID)
		if err != nil && err != storage.ErrNotFound {
			klog.Errorf("Failed to get organization zone access for %s: %v", req.OrgID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify zone access"})
			return
		}

		// If specific zone access is defined, check if this zone is allowed
		if len(zoneAccess) > 0 {
			hasAccess := false
			for _, access := range zoneAccess {
				if access.ZoneID == req.ZoneID {
					hasAccess = true
					break
				}
			}
			if !hasAccess {
				c.JSON(http.StatusForbidden, gin.H{
					"error": fmt.Sprintf("Organization does not have access to zone '%s'", zone.Name),
				})
				return
			}
		}
	}

	// Generate VDC ID (use sanitized name for CRD)
	vdcID := util.SanitizeKubernetesName(req.Name)

	// Create VirtualDataCenter CRD
	vdcCR := &ovimv1.VirtualDataCenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vdcID,
			Namespace: fmt.Sprintf("org-%s", req.OrgID), // VDC CRs live in org namespace
			Annotations: map[string]string{
				"ovim.io/created-by": username,
				"ovim.io/created-at": time.Now().Format(time.RFC3339),
			},
		},
		Spec: ovimv1.VirtualDataCenterSpec{
			OrganizationRef: req.OrgID,
			ZoneID:          req.ZoneID,
			DisplayName:     req.DisplayName,
			Description:     req.Description,
			Quota: ovimv1.ResourceQuota{
				CPU:     fmt.Sprintf("%d", req.CPUQuota),
				Memory:  fmt.Sprintf("%dGi", req.MemoryQuota),
				Storage: fmt.Sprintf("%dTi", (req.StorageQuota+1023)/1024), // Convert GB to TB (round up)
			},
			NetworkPolicy: req.NetworkPolicy,
		},
	}

	// Add LimitRange if provided
	if req.MinCPU != nil || req.MaxCPU != nil || req.MinMemory != nil || req.MaxMemory != nil {
		vdcCR.Spec.LimitRange = &ovimv1.LimitRange{
			MinCpu:    *req.MinCPU * 1000,    // Convert CPU cores to millicores
			MaxCpu:    *req.MaxCPU * 1000,    // Convert CPU cores to millicores
			MinMemory: *req.MinMemory * 1024, // Convert GB to MB
			MaxMemory: *req.MaxMemory * 1024, // Convert GB to MB
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.k8sClient.Create(ctx, vdcCR); err != nil {
		klog.Errorf("Failed to create VirtualDataCenter CRD %s: %v", vdcID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create VDC CRD"})
		return
	}

	klog.Infof("Created VirtualDataCenter CRD %s in org %s by user %s (%s)", vdcID, req.OrgID, username, userID)

	// Return VDC response from CRD
	response := &models.VirtualDataCenter{
		ID:                vdcID,
		Name:              req.Name,
		Description:       req.Description,
		OrgID:             req.OrgID,
		ZoneID:            &req.ZoneID, // Zone where VDC is deployed
		DisplayName:       &req.DisplayName,
		CRName:            vdcID,
		CRNamespace:       fmt.Sprintf("org-%s", req.OrgID),
		WorkloadNamespace: fmt.Sprintf("vdc-org-%s-%s", req.OrgID, vdcID),
		CPUQuota:          req.CPUQuota,
		MemoryQuota:       req.MemoryQuota,
		StorageQuota:      req.StorageQuota,
		NetworkPolicy:     req.NetworkPolicy,
		Phase:             "Pending", // Controller will handle creation
	}

	klog.Infof("VDC %s (%s) creation initiated in org %s by user %s (%s) - controller will handle resource creation",
		req.DisplayName, vdcID, req.OrgID, username, userID)

	// Record API event
	if h.eventRecorder != nil {
		h.eventRecorder.RecordVDCCreated(ctx, vdcID, req.OrgID, username)
	}

	c.JSON(http.StatusCreated, response)
}

// Update handles updating a VDC
func (h *VDCHandlers) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	var req models.UpdateVDCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.V(4).Infof("Invalid update VDC request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First, find the VDC CRD - we need to check both the org namespace and discover the right one
	var vdcCR *ovimv1.VirtualDataCenter
	var orgNamespace string

	// Try to find the VDC by listing all VDCs and finding the one with matching name
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := h.k8sClient.List(ctx, vdcList); err != nil {
		klog.Errorf("Failed to list VDCs to find %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find VDC"})
		return
	}

	for _, vdc := range vdcList.Items {
		if vdc.Name == id {
			vdcCR = &vdc
			orgNamespace = vdc.Namespace
			break
		}
	}

	if vdcCR == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
		return
	}

	// Check permissions - only system admin and org admin can update VDCs
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to update VDC"})
		return
	}

	// For org admin, ensure they can only update VDCs in their own organization
	if role == models.RoleOrgAdmin {
		expectedOrgNamespace := fmt.Sprintf("org-%s", userOrgID)
		if userOrgID == "" || orgNamespace != expectedOrgNamespace {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only update VDCs in your own organization"})
			return
		}
	}

	// Update fields
	if req.DisplayName != nil {
		vdcCR.Spec.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		vdcCR.Spec.Description = *req.Description
	}
	if req.CPUQuota != nil {
		vdcCR.Spec.Quota.CPU = fmt.Sprintf("%d", *req.CPUQuota)
	}
	if req.MemoryQuota != nil {
		vdcCR.Spec.Quota.Memory = fmt.Sprintf("%dGi", *req.MemoryQuota)
	}
	if req.StorageQuota != nil {
		vdcCR.Spec.Quota.Storage = fmt.Sprintf("%dTi", (*req.StorageQuota+1023)/1024) // Convert GB to TB (round up)
	}
	if req.NetworkPolicy != nil {
		vdcCR.Spec.NetworkPolicy = *req.NetworkPolicy
	}

	// Add update annotation
	if vdcCR.Annotations == nil {
		vdcCR.Annotations = make(map[string]string)
	}
	vdcCR.Annotations["ovim.io/updated-by"] = username
	vdcCR.Annotations["ovim.io/updated-at"] = time.Now().Format(time.RFC3339)

	if err := h.k8sClient.Update(ctx, vdcCR); err != nil {
		klog.Errorf("Failed to update VirtualDataCenter CRD %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update VDC CRD"})
		return
	}

	klog.Infof("Updated VirtualDataCenter CRD %s by user %s (%s)", id, username, userID)

	// Record API event
	if h.eventRecorder != nil {
		h.eventRecorder.RecordVDCUpdated(ctx, id, vdcCR.Spec.OrganizationRef, username)
	}

	// Return updated VDC data from CRD
	response := &models.VirtualDataCenter{
		ID:                vdcCR.Name,
		Name:              vdcCR.Spec.DisplayName,
		Description:       vdcCR.Spec.Description,
		OrgID:             vdcCR.Spec.OrganizationRef,
		DisplayName:       &vdcCR.Spec.DisplayName,
		CRName:            vdcCR.Name,
		CRNamespace:       vdcCR.Namespace,
		WorkloadNamespace: vdcCR.Status.Namespace,
		NetworkPolicy:     vdcCR.Spec.NetworkPolicy,
		Phase:             string(vdcCR.Status.Phase),
	}

	c.JSON(http.StatusOK, response)
}

// Delete handles deleting a VDC
func (h *VDCHandlers) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First, find the VDC CRD
	var vdcCR *ovimv1.VirtualDataCenter
	var orgNamespace string

	// Try to find the VDC by listing all VDCs and finding the one with matching name
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := h.k8sClient.List(ctx, vdcList); err != nil {
		klog.Errorf("Failed to list VDCs to find %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find VDC"})
		return
	}

	for _, vdc := range vdcList.Items {
		if vdc.Name == id {
			vdcCR = &vdc
			orgNamespace = vdc.Namespace
			break
		}
	}

	if vdcCR == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
		return
	}

	// Check permissions - only system admin and org admin can delete VDCs
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to delete VDC"})
		return
	}

	// For org admin, ensure they can only delete VDCs in their own organization
	if role == models.RoleOrgAdmin {
		expectedOrgNamespace := fmt.Sprintf("org-%s", userOrgID)
		if userOrgID == "" || orgNamespace != expectedOrgNamespace {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only delete VDCs in your own organization"})
			return
		}
	}

	// Check for dependent VMs
	vms, err := h.storage.ListVMs("")
	if err != nil {
		klog.Errorf("Failed to list VMs for VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check VMs"})
		return
	}

	// Check if any VMs are assigned to this VDC
	var vmsInVDC []*models.VirtualMachine
	for _, vm := range vms {
		if vm.VDCID != nil && *vm.VDCID == id {
			vmsInVDC = append(vmsInVDC, vm)
		}
	}

	if len(vmsInVDC) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":    "Cannot delete VDC with existing VMs",
			"vm_count": len(vmsInVDC),
		})
		return
	}

	// Add deletion annotation for audit
	if vdcCR.Annotations == nil {
		vdcCR.Annotations = make(map[string]string)
	}
	vdcCR.Annotations["ovim.io/deleted-by"] = username
	vdcCR.Annotations["ovim.io/deleted-at"] = time.Now().Format(time.RFC3339)

	if err := h.k8sClient.Update(ctx, vdcCR); err != nil {
		klog.Warningf("Failed to add deletion annotation to VDC CRD %s: %v", id, err)
	}

	// Delete the VDC CRD
	if err := h.k8sClient.Delete(ctx, vdcCR); err != nil {
		klog.Errorf("Failed to delete VirtualDataCenter CRD %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete VDC CRD"})
		return
	}

	klog.Infof("Deleted VirtualDataCenter CRD %s by user %s (%s) - controller will handle cleanup", id, username, userID)

	// Record API event
	if h.eventRecorder != nil {
		h.eventRecorder.RecordVDCDeleted(ctx, id, vdcCR.Spec.OrganizationRef, username)
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListUserVDCs handles listing VDCs for the current user's organization
func (h *VDCHandlers) ListUserVDCs(c *gin.Context) {
	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Only org users and org admins can use this endpoint
	if role != models.RoleOrgAdmin && role != models.RoleOrgUser {
		c.JSON(http.StatusForbidden, gin.H{"error": "This endpoint is for organization users only"})
		return
	}

	// Check if user has an organization
	if userOrgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is not assigned to any organization"})
		return
	}

	// Get query parameters for filtering
	zoneFilter := c.Query("zone_id")

	// Get VDCs for the user's organization
	vdcs, err := h.storage.ListVDCs(userOrgID)
	if err != nil {
		klog.Errorf("Failed to list VDCs for user %s (%s) in org %s: %v", username, userID, userOrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list VDCs"})
		return
	}

	// Apply zone filtering if specified
	if zoneFilter != "" {
		var filteredVDCs []*models.VirtualDataCenter
		for _, vdc := range vdcs {
			if vdc.ZoneID != nil && *vdc.ZoneID == zoneFilter {
				filteredVDCs = append(filteredVDCs, vdc)
			}
		}
		vdcs = filteredVDCs
	}

	klog.V(6).Infof("Listed %d VDCs for user %s (%s) in org %s", len(vdcs), username, userID, userOrgID)
	c.JSON(http.StatusOK, gin.H{
		"vdcs":  vdcs,
		"total": len(vdcs),
	})
}

// GetResourceUsage handles getting VDC resource usage
func (h *VDCHandlers) GetResourceUsage(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get VDC
	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Check permissions - only system admin can view any VDC, others can only view VDCs from their org
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only view resource usage for VDCs in your organization"})
			return
		}
	}

	// Get VMs for this VDC (we need all VMs in the organization to pass to the method)
	vms, err := h.storage.ListVMs(vdc.OrgID)
	if err != nil {
		klog.Errorf("Failed to list VMs for VDC %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VMs"})
		return
	}

	// Calculate resource usage
	usage := vdc.GetResourceUsage(vms)

	klog.V(6).Infof("Retrieved resource usage for VDC %s (CPU: %d/%d, Memory: %d/%d, Storage: %d/%d, VMs: %d)",
		vdc.Name, usage.CPUUsed, usage.CPUQuota, usage.MemoryUsed, usage.MemoryQuota, usage.StorageUsed, usage.StorageQuota, usage.VMCount)

	c.JSON(http.StatusOK, usage)
}

// CheckVDCRequirements handles checking if an organization has functioning VDCs for VM deployment
func (h *VDCHandlers) CheckVDCRequirements(c *gin.Context) {
	orgID := c.Param("id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Check permissions - system admin can check any org, others can only check their own org
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only check VDC requirements for your own organization"})
			return
		}
	}

	// Get VDCs for the organization
	vdcs, err := h.storage.ListVDCs(orgID)
	if err != nil {
		klog.Errorf("Failed to get VDCs for organization %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check VDC requirements"})
		return
	}

	// Check if there's at least one functioning VDC (Active or Ready phase)
	hasFunctioningVDC := false
	functioningVDCs := []string{}
	for _, vdc := range vdcs {
		if vdc.Phase == "Active" || vdc.Phase == "Ready" {
			hasFunctioningVDC = true
			functioningVDCs = append(functioningVDCs, vdc.Name)
		}
	}

	klog.V(6).Infof("Checked VDC requirements for org %s by user %s (%s): %d total VDCs, %d functioning",
		orgID, username, userID, len(vdcs), len(functioningVDCs))

	c.JSON(http.StatusOK, gin.H{
		"canDeployVMs":        hasFunctioningVDC,
		"totalVDCs":           len(vdcs),
		"functioningVDCs":     len(functioningVDCs),
		"functioningVDCNames": functioningVDCs,
		"message": func() string {
			if hasFunctioningVDC {
				return "Organization has functioning VDCs and can deploy VMs"
			}
			return "Organization requires at least one active Virtual Data Center (VDC) before deploying virtual machines"
		}(),
	})
}

// GetStatus handles getting VDC status from CRD
func (h *VDCHandlers) GetStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First, find the VDC CRD
	var vdcCR *ovimv1.VirtualDataCenter
	var orgNamespace string

	// Try to find the VDC by listing all VDCs and finding the one with matching name
	vdcList := &ovimv1.VirtualDataCenterList{}
	if err := h.k8sClient.List(ctx, vdcList); err != nil {
		klog.Errorf("Failed to list VDCs to find %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find VDC"})
		return
	}

	for _, vdc := range vdcList.Items {
		if vdc.Name == id {
			vdcCR = &vdc
			orgNamespace = vdc.Namespace
			break
		}
	}

	if vdcCR == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
		return
	}

	// Check permissions - only system admin and org admin can access VDC status
	if role != models.RoleSystemAdmin && role != models.RoleOrgAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to view VDC status"})
		return
	}

	// For org admin, ensure they can only access VDCs in their own organization
	if role == models.RoleOrgAdmin {
		expectedOrgNamespace := fmt.Sprintf("org-%s", userOrgID)
		if userOrgID == "" || orgNamespace != expectedOrgNamespace {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only view VDC status in your own organization"})
			return
		}
	}

	klog.V(6).Infof("Retrieved VDC status for %s by user %s (%s)", id, username, userID)

	// Return CRD status
	response := gin.H{
		"phase":      string(vdcCR.Status.Phase),
		"conditions": vdcCR.Status.Conditions,
		"namespace":  vdcCR.Status.Namespace,
	}

	c.JSON(http.StatusOK, response)
}

// GetLimitRange handles getting VDC LimitRange information
func (h *VDCHandlers) GetLimitRange(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "VDC ID required"})
		return
	}

	// Get user info from context
	userID, username, role, userOrgID, ok := auth.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User context not found"})
		return
	}

	// Get VDC
	vdc, err := h.storage.GetVDC(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VDC not found"})
			return
		}
		klog.Errorf("Failed to get VDC %s for user %s (%s): %v", id, username, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VDC"})
		return
	}

	// Check permissions - only system admin can view any VDC, others can only view VDCs from their org
	if role != models.RoleSystemAdmin {
		if userOrgID == "" || userOrgID != vdc.OrgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Can only view LimitRange for VDCs in your organization"})
			return
		}
	}

	// Use OpenShift client to get LimitRange information from the VDC workload namespace
	if h.openShiftClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OpenShift integration not available"})
		return
	}

	ctx := context.Background()
	limitRangeInfo, err := h.openShiftClient.GetLimitRange(ctx, vdc.WorkloadNamespace)
	if err != nil {
		klog.Errorf("Failed to get LimitRange for VDC %s namespace %s: %v", id, vdc.WorkloadNamespace, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get LimitRange information"})
		return
	}

	klog.V(6).Infof("Retrieved LimitRange for VDC %s by user %s (%s)", id, username, userID)

	c.JSON(http.StatusOK, limitRangeInfo)
}
