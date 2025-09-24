package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// ZoneResponse represents a zone in API responses
type ZoneResponse struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	ClusterName     string                `json:"cluster_name"`
	APIUrl          string                `json:"api_url"`
	Status          string                `json:"status"`
	Region          string                `json:"region"`
	CloudProvider   string                `json:"cloud_provider"`
	NodeCount       int                   `json:"node_count"`
	CPUCapacity     int                   `json:"cpu_capacity"`
	MemoryCapacity  int                   `json:"memory_capacity"`
	StorageCapacity int                   `json:"storage_capacity"`
	CPUQuota        int                   `json:"cpu_quota"`
	MemoryQuota     int                   `json:"memory_quota"`
	StorageQuota    int                   `json:"storage_quota"`
	VMCount         int                   `json:"vm_count"`
	Labels          map[string]string     `json:"labels,omitempty"`
	Annotations     map[string]string     `json:"annotations,omitempty"`
	CreatedAt       string                `json:"created_at"`
	UpdatedAt       string                `json:"updated_at"`
	LastSync        string                `json:"last_sync"`
	SpokeAgent      *SpokeAgentZoneStatus `json:"spoke_agent,omitempty"`
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
	VMCount       int    `json:"vm_count"`
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

// SimpleZoneResponse represents a zone response compatible with UI
type SimpleZoneResponse struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	Location        string                `json:"location"`
	Status          string                `json:"status"`
	ClusterEndpoint string                `json:"cluster_endpoint,omitempty"`
	CPUCapacity     int                   `json:"cpu_capacity"`
	MemoryCapacity  int                   `json:"memory_capacity"`
	StorageCapacity int                   `json:"storage_capacity"`
	CPUQuota        int                   `json:"cpu_quota"`
	MemoryQuota     int                   `json:"memory_quota"`
	StorageQuota    int                   `json:"storage_quota"`
	VMCount         int                   `json:"vm_count"`
	CreatedAt       string                `json:"created_at"`
	UpdatedAt       string                `json:"updated_at"`
	SpokeAgent      *SpokeAgentZoneStatus `json:"spoke_agent,omitempty"`
}

// SpokeAgentZoneStatus represents the status of a spoke agent for a zone
type SpokeAgentZoneStatus struct {
	AgentID     string    `json:"agent_id"`
	Status      string    `json:"status"`
	LastContact time.Time `json:"last_contact"`
	Version     string    `json:"version,omitempty"`
	IsConnected bool      `json:"is_connected"`
	VMCount     int       `json:"vm_count"`
	VDCCount    int       `json:"vdc_count"`
	ErrorCount  int       `json:"error_count"`
}

// ListZones handles GET /api/v1/zones
func (s *Server) ListZones(c *gin.Context) {
	klog.V(4).Info("Listing zones from ACM managed clusters")

	// Get query parameters for filtering
	status := c.Query("status")
	provider := c.Query("provider")
	region := c.Query("region")

	// Check if Kubernetes client is available
	if s.k8sClientset == nil {
		klog.Warning("Kubernetes client not available, returning empty zones list")
		c.JSON(http.StatusOK, gin.H{
			"zones": []SimpleZoneResponse{},
			"total": 0,
		})
		return
	}

	// Fetch managed clusters from ACM using the same approach as ACM client
	ctx := context.Background()
	result := s.k8sClientset.CoreV1().RESTClient().Get().
		AbsPath("/apis/cluster.open-cluster-management.io/v1/managedclusters").
		Do(ctx)

	data, err := result.Raw()
	if err != nil {
		klog.Errorf("ACM API call failed: %v", err)

		// Check if ACM is installed by trying to access the API group
		_, apiErr := s.k8sClientset.Discovery().ServerResourcesForGroupVersion("cluster.open-cluster-management.io/v1")
		if apiErr != nil {
			klog.Warningf("ACM (Advanced Cluster Management) is not installed or accessible: %v", apiErr)
			c.JSON(http.StatusOK, gin.H{
				"zones": []SimpleZoneResponse{},
				"total": 0,
			})
			return
		}

		klog.Errorf("Failed to list managed clusters from ACM API - insufficient RBAC permissions or ACM not properly configured: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve zones from ACM",
		})
		return
	}

	// Parse the raw JSON response
	var rawList map[string]interface{}
	if err := json.Unmarshal(data, &rawList); err != nil {
		klog.Errorf("Failed to decode managed clusters response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to parse ACM response",
		})
		return
	}

	// Get zone utilization for VM counts
	var vmCountMap map[string]int
	if allUtilizations, err := s.storage.GetZoneUtilization(); err == nil {
		vmCountMap = make(map[string]int)
		for _, util := range allUtilizations {
			vmCountMap[util.ID] = util.VMCount
		}
	} else {
		klog.Warningf("Failed to get zone utilizations for zone listing: %v", err)
		vmCountMap = make(map[string]int) // Empty map as fallback
	}

	// Get spoke agent statuses for all zones
	zoneAgentStatuses := s.spokeHandlers.GetAllZoneStatuses()

	// Convert managed clusters to zones
	var zones []SimpleZoneResponse
	if items, ok := rawList["items"].([]interface{}); ok {
		for _, item := range items {
			if clusterData, ok := item.(map[string]interface{}); ok {
				zone := convertManagedClusterToZone(clusterData)
				if zone != nil {
					// Apply filtering
					if status != "" && zone.Status != status {
						continue
					}
					if provider != "" && zone.Location != provider {
						continue
					}
					if region != "" && zone.Location != region {
						continue
					}

					// Add spoke agent status if available and use it for VM count
					if agentStatus, exists := zoneAgentStatuses[zone.ID]; exists {
						// Calculate if agent is connected (last contact within 2 minutes)
						isConnected := time.Since(agentStatus.LastHubContact) < 2*time.Minute

						// Use VM count from spoke agent (real-time data)
						zone.VMCount = len(agentStatus.VMs)

						zone.SpokeAgent = &SpokeAgentZoneStatus{
							AgentID:     agentStatus.AgentID,
							Status:      agentStatus.Status,
							LastContact: agentStatus.LastHubContact,
							Version:     agentStatus.Version,
							IsConnected: isConnected,
							VMCount:     len(agentStatus.VMs),
							VDCCount:    len(agentStatus.VDCs),
							ErrorCount:  len(agentStatus.Errors),
						}
					} else {
						// Fallback to storage utilization if no spoke agent available
						zone.VMCount = vmCountMap[zone.ID]
					}

					zones = append(zones, *zone)
				}
			}
		}
	}

	klog.V(4).Infof("Found %d zones from ACM managed clusters", len(zones))

	c.JSON(http.StatusOK, gin.H{
		"zones": zones,
		"total": len(zones),
	})
}

// convertManagedClusterToZone converts a managed cluster to a zone response
func convertManagedClusterToZone(clusterData map[string]interface{}) *SimpleZoneResponse {
	// Extract metadata
	metadata, ok := clusterData["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	name, ok := metadata["name"].(string)
	if !ok {
		return nil
	}

	// Extract creation timestamp
	createdAt := time.Now().Format(time.RFC3339)
	if creationTimestamp, ok := metadata["creationTimestamp"].(string); ok {
		createdAt = creationTimestamp
	}

	// Extract status
	status := "unknown"
	if statusData, ok := clusterData["status"].(map[string]interface{}); ok {
		if conditions, ok := statusData["conditions"].([]interface{}); ok {
			for _, conditionInterface := range conditions {
				if condition, ok := conditionInterface.(map[string]interface{}); ok {
					if conditionType, ok := condition["type"].(string); ok && conditionType == "ManagedClusterConditionAvailable" {
						if conditionStatus, ok := condition["status"].(string); ok {
							if conditionStatus == "True" {
								status = "available"
							} else {
								status = "unavailable"
							}
						}
					}
				}
			}
		}
	}

	// Extract labels for additional info
	location := "unknown"
	clusterEndpoint := ""
	if labels, ok := metadata["labels"].(map[string]interface{}); ok {
		if cloud, ok := labels["cloud"].(string); ok {
			location = cloud
		}
		if region, ok := labels["region"].(string); ok && region != "" {
			location = region
		}
	}

	// Try to get cluster endpoint from spec
	if spec, ok := clusterData["spec"].(map[string]interface{}); ok {
		if hubAcceptsClient, ok := spec["hubAcceptsClient"].(bool); ok && hubAcceptsClient {
			if managedClusterClientConfigs, ok := spec["managedClusterClientConfigs"].([]interface{}); ok {
				for _, configInterface := range managedClusterClientConfigs {
					if config, ok := configInterface.(map[string]interface{}); ok {
						if url, ok := config["url"].(string); ok && url != "" {
							clusterEndpoint = url
							break
						}
					}
				}
			}
		}
	}

	// Extract capacity information from status
	cpuCapacity := 0
	memoryCapacity := 0
	storageCapacity := 0
	cpuQuota := 0
	memoryQuota := 0
	storageQuota := 0

	if statusData, ok := clusterData["status"].(map[string]interface{}); ok {
		// Extract capacity information
		if capacity, ok := statusData["capacity"].(map[string]interface{}); ok {
			if cpu, ok := capacity["cpu"].(string); ok {
				if cpuInt, err := parseResourceQuantity(cpu); err == nil {
					cpuCapacity = cpuInt
				}
			}
			if memory, ok := capacity["memory"].(string); ok {
				if memoryBytes, err := parseResourceQuantityToBytes(memory); err == nil {
					memoryCapacity = memoryBytes
				}
			}
			if ephemeralStorage, ok := capacity["ephemeral-storage"].(string); ok {
				if storageBytes, err := parseResourceQuantityToBytes(ephemeralStorage); err == nil {
					storageCapacity = storageBytes
				}
			}
		}

		// Extract allocatable information (could be used for more accurate quotas)
		if allocatable, ok := statusData["allocatable"].(map[string]interface{}); ok {
			if cpu, ok := allocatable["cpu"].(string); ok {
				if cpuInt, err := parseResourceQuantity(cpu); err == nil {
					cpuQuota = cpuInt // Use allocatable for quota
				}
			}
			if memory, ok := allocatable["memory"].(string); ok {
				if memoryBytes, err := parseResourceQuantityToBytes(memory); err == nil {
					memoryQuota = memoryBytes // Use allocatable for quota
				}
			}
			if storage, ok := allocatable["ephemeral-storage"].(string); ok {
				if storageBytes, err := parseResourceQuantityToBytes(storage); err == nil {
					storageQuota = storageBytes // Use allocatable for quota
				}
			}
		}
	}

	return &SimpleZoneResponse{
		ID:              name,
		Name:            name,
		Description:     "Managed cluster from ACM",
		Location:        location,
		Status:          status,
		ClusterEndpoint: clusterEndpoint,
		CPUCapacity:     cpuCapacity,
		MemoryCapacity:  memoryCapacity,
		StorageCapacity: storageCapacity,
		CPUQuota:        cpuQuota,
		MemoryQuota:     memoryQuota,
		StorageQuota:    storageQuota,
		VMCount:         0, // Will be populated by the caller from zone utilization
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}
}

// GetZone handles GET /api/v1/zones/:id
func (s *Server) GetZone(c *gin.Context) {
	zoneID := c.Param("id")
	klog.V(4).Infof("Getting zone from ACM: %s", zoneID)

	// Check if Kubernetes client is available
	if s.k8sClientset == nil {
		klog.Warning("Kubernetes client not available")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Zone not found",
		})
		return
	}

	// Fetch specific managed cluster from ACM
	ctx := context.Background()
	result := s.k8sClientset.CoreV1().RESTClient().Get().
		AbsPath("/apis/cluster.open-cluster-management.io/v1/managedclusters/" + zoneID).
		Do(ctx)

	data, err := result.Raw()
	if err != nil {
		klog.Errorf("Failed to get managed cluster %s: %v", zoneID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Zone not found",
		})
		return
	}

	// Parse the raw JSON response
	var clusterData map[string]interface{}
	if err := json.Unmarshal(data, &clusterData); err != nil {
		klog.Errorf("Failed to decode managed cluster response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to parse ACM response",
		})
		return
	}

	// Convert to zone response
	simpleZone := convertManagedClusterToZone(clusterData)
	if simpleZone == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Zone not found",
		})
		return
	}

	// Convert to detailed zone response
	response := convertManagedClusterToDetailedZone(clusterData)
	if response == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Zone not found",
		})
		return
	}

	// Get VM count from spoke agent status first, fallback to zone utilization
	zoneAgentStatuses := s.spokeHandlers.GetAllZoneStatuses()
	if agentStatus, exists := zoneAgentStatuses[response.ID]; exists {
		// Use VM count from spoke agent (real-time data)
		response.VMCount = len(agentStatus.VMs)

		// Also add spoke agent status to detailed response
		isConnected := time.Since(agentStatus.LastHubContact) < 2*time.Minute
		response.SpokeAgent = &SpokeAgentZoneStatus{
			AgentID:     agentStatus.AgentID,
			Status:      agentStatus.Status,
			LastContact: agentStatus.LastHubContact,
			Version:     agentStatus.Version,
			IsConnected: isConnected,
			VMCount:     len(agentStatus.VMs),
			VDCCount:    len(agentStatus.VDCs),
			ErrorCount:  len(agentStatus.Errors),
		}
	} else {
		// Fallback to storage utilization if no spoke agent available
		if allUtilizations, err := s.storage.GetZoneUtilization(); err == nil {
			for _, util := range allUtilizations {
				if util.ID == response.ID {
					response.VMCount = util.VMCount
					break
				}
			}
		}
	}

	c.JSON(http.StatusOK, *response)
}

// convertManagedClusterToDetailedZone converts a managed cluster to a detailed zone response
func convertManagedClusterToDetailedZone(clusterData map[string]interface{}) *ZoneResponse {
	// Extract metadata
	metadata, ok := clusterData["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	name, ok := metadata["name"].(string)
	if !ok {
		return nil
	}

	// Extract creation timestamp
	createdAt := time.Now().Format(time.RFC3339)
	if creationTimestamp, ok := metadata["creationTimestamp"].(string); ok {
		createdAt = creationTimestamp
	}

	// Extract labels and annotations
	labels := make(map[string]string)
	annotations := make(map[string]string)

	if metaLabels, ok := metadata["labels"].(map[string]interface{}); ok {
		for k, v := range metaLabels {
			if strValue, ok := v.(string); ok {
				labels[k] = strValue
			}
		}
	}

	if metaAnnotations, ok := metadata["annotations"].(map[string]interface{}); ok {
		for k, v := range metaAnnotations {
			if strValue, ok := v.(string); ok {
				annotations[k] = strValue
			}
		}
	}

	// Extract basic info
	region := "unknown"
	cloudProvider := "unknown"
	if cloud, ok := labels["cloud"]; ok && cloud != "" {
		cloudProvider = cloud
	}
	if regionLabel, ok := labels["region"]; ok && regionLabel != "" {
		region = regionLabel
	}

	// Extract cluster endpoint from spec
	apiUrl := ""
	if spec, ok := clusterData["spec"].(map[string]interface{}); ok {
		if hubAcceptsClient, ok := spec["hubAcceptsClient"].(bool); ok && hubAcceptsClient {
			if managedClusterClientConfigs, ok := spec["managedClusterClientConfigs"].([]interface{}); ok {
				for _, configInterface := range managedClusterClientConfigs {
					if config, ok := configInterface.(map[string]interface{}); ok {
						if url, ok := config["url"].(string); ok && url != "" {
							apiUrl = url
							break
						}
					}
				}
			}
		}
	}

	// Extract status
	status := "unknown"
	nodeCount := 0
	cpuCapacity := 0
	memoryCapacity := 0
	storageCapacity := 0
	cpuQuota := 0
	memoryQuota := 0
	storageQuota := 0
	lastSync := createdAt

	if statusData, ok := clusterData["status"].(map[string]interface{}); ok {
		// Extract cluster conditions for status
		if conditions, ok := statusData["conditions"].([]interface{}); ok {
			for _, conditionInterface := range conditions {
				if condition, ok := conditionInterface.(map[string]interface{}); ok {
					if conditionType, ok := condition["type"].(string); ok && conditionType == "ManagedClusterConditionAvailable" {
						if conditionStatus, ok := condition["status"].(string); ok {
							if conditionStatus == "True" {
								status = "available"
							} else {
								status = "unavailable"
							}
						}
						// Extract last transition time
						if lastTransitionTime, ok := condition["lastTransitionTime"].(string); ok {
							lastSync = lastTransitionTime
						}
					}
				}
			}
		}

		// Extract capacity information
		if capacity, ok := statusData["capacity"].(map[string]interface{}); ok {
			if cpu, ok := capacity["cpu"].(string); ok {
				if cpuInt, err := parseResourceQuantity(cpu); err == nil {
					cpuCapacity = cpuInt
					cpuQuota = cpuInt // Set quota same as capacity for now
				}
			}
			if memory, ok := capacity["memory"].(string); ok {
				if memoryBytes, err := parseResourceQuantityToBytes(memory); err == nil {
					memoryCapacity = memoryBytes
					memoryQuota = memoryBytes // Set quota same as capacity for now
				}
			}
			if ephemeralStorage, ok := capacity["ephemeral-storage"].(string); ok {
				if storageBytes, err := parseResourceQuantityToBytes(ephemeralStorage); err == nil {
					storageCapacity = storageBytes
					storageQuota = storageBytes // Set quota same as capacity for now
				}
			}
		}

		// Extract allocatable information (could be used for more accurate quotas)
		if allocatable, ok := statusData["allocatable"].(map[string]interface{}); ok {
			if cpu, ok := allocatable["cpu"].(string); ok {
				if cpuInt, err := parseResourceQuantity(cpu); err == nil {
					cpuQuota = cpuInt // Use allocatable for quota
				}
			}
			if memory, ok := allocatable["memory"].(string); ok {
				if memoryBytes, err := parseResourceQuantityToBytes(memory); err == nil {
					memoryQuota = memoryBytes // Use allocatable for quota
				}
			}
			if storage, ok := allocatable["ephemeral-storage"].(string); ok {
				if storageBytes, err := parseResourceQuantityToBytes(storage); err == nil {
					storageQuota = storageBytes // Use allocatable for quota
				}
			}
		}

		// Extract node count from cluster claims or version
		if clusterClaims, ok := statusData["clusterClaims"].([]interface{}); ok {
			for _, claimInterface := range clusterClaims {
				if claim, ok := claimInterface.(map[string]interface{}); ok {
					if claimName, ok := claim["name"].(string); ok {
						if claimName == "core_worker.count" || claimName == "socket_worker.count" {
							if value, ok := claim["value"].(string); ok {
								if nodeCountInt, err := parseResourceQuantity(value); err == nil {
									nodeCount += nodeCountInt
								}
							}
						}
					}
				}
			}
		}
	}

	return &ZoneResponse{
		ID:              name,
		Name:            name,
		ClusterName:     name,
		APIUrl:          apiUrl,
		Status:          status,
		Region:          region,
		CloudProvider:   cloudProvider,
		NodeCount:       nodeCount,
		CPUCapacity:     cpuCapacity,
		MemoryCapacity:  memoryCapacity,
		StorageCapacity: storageCapacity,
		CPUQuota:        cpuQuota,
		MemoryQuota:     memoryQuota,
		StorageQuota:    storageQuota,
		VMCount:         0, // Will be populated by the caller from zone utilization
		Labels:          labels,
		Annotations:     annotations,
		CreatedAt:       createdAt,
		UpdatedAt:       lastSync,
		LastSync:        lastSync,
	}
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
		VMCount:            utilization.VMCount,
	}

	c.JSON(http.StatusOK, response)
}

// ListOrganizationZones handles GET /api/v1/organizations/:orgId/zones
func (s *Server) ListOrganizationZones(c *gin.Context) {
	orgID := c.Param("orgId")
	klog.V(4).Infof("Listing zones for organization: %s", orgID)

	// Verify organization exists
	// Create temporary organization handlers to access the same storage pattern used by working endpoints
	orgHandlers := NewOrganizationHandlers(s.storage, s.k8sClient, s.openshiftClient)
	_, err := orgHandlers.storage.GetOrganization(orgID)
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

	// Get spoke agent statuses for VM counts (prioritize over storage utilization)
	zoneAgentStatuses := s.spokeHandlers.GetAllZoneStatuses()

	// Also get zone utilization as fallback
	allUtilizations, err := s.storage.GetZoneUtilization()
	if err != nil {
		klog.Warningf("Failed to get zone utilizations for organization zones: %v", err)
	}

	// Create a map for quick lookup of VM counts from storage (fallback)
	vmCountMap := make(map[string]int)
	for _, util := range allUtilizations {
		vmCountMap[util.ID] = util.VMCount
	}

	// Get zones that the organization has access to
	var response []ZoneSummary
	for _, access := range zoneAccess {
		zone, err := s.storage.GetZone(access.ZoneID)
		if err != nil {
			klog.Warningf("Failed to get zone %s for organization %s: %v", access.ZoneID, orgID, err)
			continue
		}

		// Get VM count from spoke agent first, fallback to storage utilization
		vmCount := 0
		if agentStatus, exists := zoneAgentStatuses[zone.ID]; exists {
			vmCount = len(agentStatus.VMs) // Use real-time data from spoke agent
		} else {
			vmCount = vmCountMap[zone.ID] // Fallback to storage utilization
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
			VMCount:       vmCount,
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

// parseResourceQuantity parses Kubernetes resource quantities (e.g., "4", "8Gi", "100Gi")
func parseResourceQuantity(quantity string) (int, error) {
	if quantity == "" {
		return 0, strconv.ErrSyntax
	}

	// Handle plain numbers
	if val, err := strconv.Atoi(quantity); err == nil {
		return val, nil
	}

	// Handle suffixed quantities
	quantity = strings.TrimSpace(quantity)

	// Binary suffixes (powers of 1024)
	binarySuffixes := map[string]int64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"Pi": 1024 * 1024 * 1024 * 1024 * 1024,
		"Ei": 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	}

	// Decimal suffixes (powers of 1000)
	decimalSuffixes := map[string]int64{
		"k": 1000,
		"M": 1000 * 1000,
		"G": 1000 * 1000 * 1000,
		"T": 1000 * 1000 * 1000 * 1000,
		"P": 1000 * 1000 * 1000 * 1000 * 1000,
		"E": 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	}

	// Try binary suffixes first
	for suffix, multiplier := range binarySuffixes {
		if strings.HasSuffix(quantity, suffix) {
			numStr := strings.TrimSuffix(quantity, suffix)
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int(num * float64(multiplier)), nil
			}
		}
	}

	// Try decimal suffixes
	for suffix, multiplier := range decimalSuffixes {
		if strings.HasSuffix(quantity, suffix) {
			numStr := strings.TrimSuffix(quantity, suffix)
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int(num * float64(multiplier)), nil
			}
		}
	}

	// If no suffix matched, try parsing as float and convert to int
	if num, err := strconv.ParseFloat(quantity, 64); err == nil {
		return int(num), nil
	}

	return 0, strconv.ErrSyntax
}

// parseResourceQuantityToBytes parses Kubernetes resource quantities and returns the value in bytes
func parseResourceQuantityToBytes(quantity string) (int, error) {
	if quantity == "" {
		return 0, strconv.ErrSyntax
	}

	// Handle plain numbers (assume bytes)
	if val, err := strconv.Atoi(quantity); err == nil {
		return val, nil
	}

	// Handle suffixed quantities
	quantity = strings.TrimSpace(quantity)

	// Binary suffixes (powers of 1024) - these are the correct ones for memory
	binarySuffixes := map[string]int64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"Pi": 1024 * 1024 * 1024 * 1024 * 1024,
		"Ei": 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	}

	// Decimal suffixes (powers of 1000)
	decimalSuffixes := map[string]int64{
		"k": 1000,
		"M": 1000 * 1000,
		"G": 1000 * 1000 * 1000,
		"T": 1000 * 1000 * 1000 * 1000,
		"P": 1000 * 1000 * 1000 * 1000 * 1000,
		"E": 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	}

	// Try binary suffixes first (most common for Kubernetes memory)
	for suffix, multiplier := range binarySuffixes {
		if strings.HasSuffix(quantity, suffix) {
			numStr := strings.TrimSuffix(quantity, suffix)
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int(num * float64(multiplier)), nil
			}
		}
	}

	// Try decimal suffixes
	for suffix, multiplier := range decimalSuffixes {
		if strings.HasSuffix(quantity, suffix) {
			numStr := strings.TrimSuffix(quantity, suffix)
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int(num * float64(multiplier)), nil
			}
		}
	}

	// If no suffix matched, try parsing as float and convert to int (assume bytes)
	if num, err := strconv.ParseFloat(quantity, 64); err == nil {
		return int(num), nil
	}

	return 0, strconv.ErrSyntax
}
