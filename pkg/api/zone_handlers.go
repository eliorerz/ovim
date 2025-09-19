package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// ZoneResponse represents a zone in API responses
type ZoneResponse struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	ClusterName     string            `json:"cluster_name"`
	APIUrl          string            `json:"api_url"`
	Status          string            `json:"status"`
	Region          string            `json:"region"`
	CloudProvider   string            `json:"cloud_provider"`
	NodeCount       int               `json:"node_count"`
	CPUCapacity     int               `json:"cpu_capacity"`
	MemoryCapacity  int               `json:"memory_capacity"`
	StorageCapacity int               `json:"storage_capacity"`
	CPUQuota        int               `json:"cpu_quota"`
	MemoryQuota     int               `json:"memory_quota"`
	StorageQuota    int               `json:"storage_quota"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
	LastSync        string            `json:"last_sync"`
}

// ZoneSummary represents a simplified zone for listing
type ZoneSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Region        string `json:"region"`
	CloudProvider string `json:"cloud_provider"`
	NodeCount     int    `json:"node_count"`
	CPUQuota      int    `json:"cpu_quota"`
	MemoryQuota   int    `json:"memory_quota"`
	StorageQuota  int    `json:"storage_quota"`
}

// ZoneUtilizationResponse represents zone utilization metrics
type ZoneUtilizationResponse struct {
	ZoneID             string  `json:"zone_id"`
	ZoneName           string  `json:"zone_name"`
	CPUUsed            int     `json:"cpu_used"`
	MemoryUsed         int     `json:"memory_used"`
	StorageUsed        int     `json:"storage_used"`
	CPUCapacity        int     `json:"cpu_capacity"`
	MemoryCapacity     int     `json:"memory_capacity"`
	StorageCapacity    int     `json:"storage_capacity"`
	CPUUtilization     float64 `json:"cpu_utilization"`
	MemoryUtilization  float64 `json:"memory_utilization"`
	StorageUtilization float64 `json:"storage_utilization"`
	VDCCount           int     `json:"vdc_count"`
	VMCount            int     `json:"vm_count"`
}

// OrganizationZoneQuotaResponse represents org-specific zone quota
type OrganizationZoneQuotaResponse struct {
	OrganizationID string `json:"organization_id"`
	ZoneID         string `json:"zone_id"`
	CPUQuota       int    `json:"cpu_quota"`
	MemoryQuota    int    `json:"memory_quota"`
	StorageQuota   int    `json:"storage_quota"`
	CPUUsed        int    `json:"cpu_used"`
	MemoryUsed     int    `json:"memory_used"`
	StorageUsed    int    `json:"storage_used"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// OrganizationZoneQuotaRequest represents quota update request
type OrganizationZoneQuotaRequest struct {
	CPUQuota     int `json:"cpu_quota" binding:"min=0"`
	MemoryQuota  int `json:"memory_quota" binding:"min=0"`
	StorageQuota int `json:"storage_quota" binding:"min=0"`
}

// ListZones handles GET /api/v1/zones
func (s *Server) ListZones(c *gin.Context) {
	klog.V(4).Info("Listing zones")

	// Get query parameters for filtering
	status := c.Query("status")
	provider := c.Query("provider")
	region := c.Query("region")

	// Get all zones from storage
	zones, err := s.storage.ListZones()
	if err != nil {
		klog.Errorf("Failed to list zones: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve zones",
		})
		return
	}

	// Filter zones based on query parameters
	var filteredZones []*models.Zone
	for _, zone := range zones {
		if status != "" && zone.Status != status {
			continue
		}
		if provider != "" && zone.CloudProvider != provider {
			continue
		}
		if region != "" && zone.Region != region {
			continue
		}
		filteredZones = append(filteredZones, zone)
	}

	// Convert to response format
	var response []ZoneSummary
	for _, zone := range filteredZones {
		response = append(response, ZoneSummary{
			ID:            zone.ID,
			Name:          zone.Name,
			Status:        zone.Status,
			Region:        zone.Region,
			CloudProvider: zone.CloudProvider,
			NodeCount:     zone.NodeCount,
			CPUQuota:      zone.CPUQuota,
			MemoryQuota:   zone.MemoryQuota,
			StorageQuota:  zone.StorageQuota,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"zones": response,
		"total": len(response),
	})
}

// GetZone handles GET /api/v1/zones/:id
func (s *Server) GetZone(c *gin.Context) {
	zoneID := c.Param("id")
	klog.V(4).Infof("Getting zone: %s", zoneID)

	zone, err := s.storage.GetZone(zoneID)
	if err != nil {
		klog.Errorf("Failed to get zone %s: %v", zoneID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Zone not found",
		})
		return
	}

	// Convert to response format
	response := ZoneResponse{
		ID:              zone.ID,
		Name:            zone.Name,
		ClusterName:     zone.ClusterName,
		APIUrl:          zone.APIUrl,
		Status:          zone.Status,
		Region:          zone.Region,
		CloudProvider:   zone.CloudProvider,
		NodeCount:       zone.NodeCount,
		CPUCapacity:     zone.CPUCapacity,
		MemoryCapacity:  zone.MemoryCapacity,
		StorageCapacity: zone.StorageCapacity,
		CPUQuota:        zone.CPUQuota,
		MemoryQuota:     zone.MemoryQuota,
		StorageQuota:    zone.StorageQuota,
		Labels:          convertStringMap(zone.Labels),
		Annotations:     convertStringMap(zone.Annotations),
		CreatedAt:       zone.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       zone.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastSync:        zone.LastSync.Format("2006-01-02T15:04:05Z07:00"),
	}

	c.JSON(http.StatusOK, response)
}

// GetZoneUtilization handles GET /api/v1/zones/:id/utilization
func (s *Server) GetZoneUtilization(c *gin.Context) {
	zoneID := c.Param("id")
	klog.V(4).Infof("Getting zone utilization: %s", zoneID)

	// Get zone details
	zone, err := s.storage.GetZone(zoneID)
	if err != nil {
		klog.Errorf("Failed to get zone %s: %v", zoneID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Zone not found",
		})
		return
	}

	// Get zone utilization
	allUtilizations, err := s.storage.GetZoneUtilization()
	if err != nil {
		klog.Errorf("Failed to get zone utilizations: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve zone utilization",
		})
		return
	}

	// Find utilization for this specific zone
	var utilization *models.ZoneUtilization
	for _, util := range allUtilizations {
		if util.ID == zoneID {
			utilization = util
			break
		}
	}

	// If no utilization found, create empty one
	if utilization == nil {
		utilization = &models.ZoneUtilization{
			ID:          zoneID,
			CPUUsed:     0,
			MemoryUsed:  0,
			StorageUsed: 0,
			VDCCount:    0,
		}
	}

	// Calculate utilization percentages
	var cpuUtil, memoryUtil, storageUtil float64
	if zone.CPUCapacity > 0 {
		cpuUtil = float64(utilization.CPUUsed) / float64(zone.CPUCapacity) * 100
	}
	if zone.MemoryCapacity > 0 {
		memoryUtil = float64(utilization.MemoryUsed) / float64(zone.MemoryCapacity) * 100
	}
	if zone.StorageCapacity > 0 {
		storageUtil = float64(utilization.StorageUsed) / float64(zone.StorageCapacity) * 100
	}

	response := ZoneUtilizationResponse{
		ZoneID:             zoneID,
		ZoneName:           zone.Name,
		CPUUsed:            utilization.CPUUsed,
		MemoryUsed:         utilization.MemoryUsed,
		StorageUsed:        utilization.StorageUsed,
		CPUCapacity:        zone.CPUCapacity,
		MemoryCapacity:     zone.MemoryCapacity,
		StorageCapacity:    zone.StorageCapacity,
		CPUUtilization:     cpuUtil,
		MemoryUtilization:  memoryUtil,
		StorageUtilization: storageUtil,
		VDCCount:           utilization.VDCCount,
		VMCount:            0, // VM count not tracked in current utilization model
	}

	c.JSON(http.StatusOK, response)
}

// ListOrganizationZones handles GET /api/v1/organizations/:orgId/zones
func (s *Server) ListOrganizationZones(c *gin.Context) {
	orgID := c.Param("orgId")
	klog.V(4).Infof("Listing zones for organization: %s", orgID)

	// Verify organization exists
	_, err := s.storage.GetOrganization(orgID)
	if err != nil {
		klog.Errorf("Failed to get organization %s: %v", orgID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Organization not found",
		})
		return
	}

	// Get organization zone access
	zoneAccess, err := s.storage.GetOrganizationZoneAccess(orgID)
	if err != nil {
		klog.Errorf("Failed to get organization zone access %s: %v", orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve organization zone access",
		})
		return
	}

	// If no specific access defined, return all available zones
	if len(zoneAccess) == 0 {
		s.ListZones(c)
		return
	}

	// Get zones that the organization has access to
	var response []ZoneSummary
	for _, access := range zoneAccess {
		zone, err := s.storage.GetZone(access.ZoneID)
		if err != nil {
			klog.Warningf("Failed to get zone %s for organization %s: %v", access.ZoneID, orgID, err)
			continue
		}

		response = append(response, ZoneSummary{
			ID:            zone.ID,
			Name:          zone.Name,
			Status:        zone.Status,
			Region:        zone.Region,
			CloudProvider: zone.CloudProvider,
			NodeCount:     zone.NodeCount,
			CPUQuota:      access.CPUQuota,
			MemoryQuota:   access.MemoryQuota,
			StorageQuota:  access.StorageQuota,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"zones": response,
		"total": len(response),
	})
}

// GetOrganizationZoneQuota handles GET /api/v1/organizations/:orgId/zones/:zoneId/quota
func (s *Server) GetOrganizationZoneQuota(c *gin.Context) {
	orgID := c.Param("orgId")
	zoneID := c.Param("zoneId")
	klog.V(4).Infof("Getting zone quota for organization %s in zone %s", orgID, zoneID)

	// Get organization zone quota
	quota, err := s.storage.GetOrganizationZoneQuota(orgID, zoneID)
	if err != nil {
		klog.Errorf("Failed to get organization zone quota %s/%s: %v", orgID, zoneID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Organization zone quota not found",
		})
		return
	}

	// Get current utilization for this organization in this zone
	allUtilizations, err := s.storage.GetZoneUtilization()
	if err != nil {
		klog.Warningf("Failed to get zone utilizations for quota response: %v", err)
	}

	// Find utilization for this specific zone
	var utilization *models.ZoneUtilization
	if allUtilizations != nil {
		for _, util := range allUtilizations {
			if util.ID == zoneID {
				utilization = util
				break
			}
		}
	}

	// If no utilization found, create empty one
	if utilization == nil {
		utilization = &models.ZoneUtilization{
			ID:          zoneID,
			CPUUsed:     0,
			MemoryUsed:  0,
			StorageUsed: 0,
		}
	}

	response := OrganizationZoneQuotaResponse{
		OrganizationID: quota.OrganizationID,
		ZoneID:         quota.ZoneID,
		CPUQuota:       quota.CPUQuota,
		MemoryQuota:    quota.MemoryQuota,
		StorageQuota:   quota.StorageQuota,
		CPUUsed:        utilization.CPUUsed,
		MemoryUsed:     utilization.MemoryUsed,
		StorageUsed:    utilization.StorageUsed,
		CreatedAt:      quota.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      quota.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	c.JSON(http.StatusOK, response)
}

// SetOrganizationZoneQuota handles PUT /api/v1/organizations/:orgId/zones/:zoneId/quota
func (s *Server) SetOrganizationZoneQuota(c *gin.Context) {
	orgID := c.Param("orgId")
	zoneID := c.Param("zoneId")
	klog.V(4).Infof("Setting zone quota for organization %s in zone %s", orgID, zoneID)

	// Parse request body
	var req OrganizationZoneQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		klog.Errorf("Failed to parse quota request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// Verify organization exists
	_, err := s.storage.GetOrganization(orgID)
	if err != nil {
		klog.Errorf("Failed to get organization %s: %v", orgID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Organization not found",
		})
		return
	}

	// Verify zone exists
	zone, err := s.storage.GetZone(zoneID)
	if err != nil {
		klog.Errorf("Failed to get zone %s: %v", zoneID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Zone not found",
		})
		return
	}

	// Validate quotas don't exceed zone capacity
	if req.CPUQuota > zone.CPUQuota {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "CPU quota exceeds zone capacity",
		})
		return
	}
	if req.MemoryQuota > zone.MemoryQuota {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Memory quota exceeds zone capacity",
		})
		return
	}
	if req.StorageQuota > zone.StorageQuota {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Storage quota exceeds zone capacity",
		})
		return
	}

	// Create or update quota
	quota := &models.OrganizationZoneQuota{
		OrganizationID: orgID,
		ZoneID:         zoneID,
		CPUQuota:       req.CPUQuota,
		MemoryQuota:    req.MemoryQuota,
		StorageQuota:   req.StorageQuota,
	}

	// Try to get existing quota first
	existingQuota, err := s.storage.GetOrganizationZoneQuota(orgID, zoneID)
	if err == nil {
		// Update existing quota
		quota.ID = existingQuota.ID
		quota.CreatedAt = existingQuota.CreatedAt
		err = s.storage.UpdateOrganizationZoneQuota(quota)
	} else {
		// Create new quota
		err = s.storage.CreateOrganizationZoneQuota(quota)
	}

	if err != nil {
		klog.Errorf("Failed to set organization zone quota %s/%s: %v", orgID, zoneID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to set quota",
		})
		return
	}

	// Record event
	if s.eventRecorder != nil {
		s.eventRecorder.RecordQuotaEvent(orgID, zoneID, "QuotaUpdated",
			"Organization zone quota updated successfully")
	}

	klog.Infof("Successfully set zone quota for organization %s in zone %s", orgID, zoneID)

	// Return the created/updated quota
	s.GetOrganizationZoneQuota(c)
}

// Helper function to convert StringMap to regular map
func convertStringMap(sm models.StringMap) map[string]string {
	if sm == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range sm {
		result[k] = string(v)
	}
	return result
}
