package acm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// ClusterDiscovery handles discovery and processing of ACM managed clusters
type ClusterDiscovery struct {
	client *Client
	config SyncConfig
}

// NewClusterDiscovery creates a new cluster discovery service
func NewClusterDiscovery(client *Client, config SyncConfig) *ClusterDiscovery {
	return &ClusterDiscovery{
		client: client,
		config: config,
	}
}

// DiscoverClusters discovers all managed clusters from ACM
func (cd *ClusterDiscovery) DiscoverClusters(ctx context.Context) ([]*ClusterInfo, error) {
	klog.V(2).Info("Starting cluster discovery from ACM")

	// List all managed clusters
	clusterList, err := cd.client.ListManagedClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list managed clusters: %w", err)
	}

	var discoveredClusters []*ClusterInfo

	for _, cluster := range clusterList.Items {
		// Skip excluded clusters
		if cd.isClusterExcluded(cluster.Name) {
			klog.V(3).Infof("Skipping excluded cluster: %s", cluster.Name)
			continue
		}

		// Check required labels
		if !cd.hasRequiredLabels(&cluster) {
			klog.V(3).Infof("Skipping cluster %s: missing required labels", cluster.Name)
			continue
		}

		// Process cluster into ClusterInfo
		clusterInfo := cd.client.GetClusterInfo(&cluster)
		if clusterInfo == nil {
			klog.Warningf("Failed to process cluster info for %s", cluster.Name)
			continue
		}

		// Enhance cluster info with additional processing
		cd.enhanceClusterInfo(clusterInfo, &cluster)

		discoveredClusters = append(discoveredClusters, clusterInfo)
		klog.V(3).Infof("Discovered cluster: %s (status: %s, nodes: %d, cpu: %d, memory: %dGB)",
			clusterInfo.Name, clusterInfo.Status, clusterInfo.NodeCount, clusterInfo.CPUCores, clusterInfo.MemoryGB)
	}

	klog.Infof("Discovery completed: found %d clusters (total %d, excluded %d)",
		len(discoveredClusters), len(clusterList.Items), len(clusterList.Items)-len(discoveredClusters))

	return discoveredClusters, nil
}

// ConvertToZones converts discovered clusters to OVIM zone models
func (cd *ClusterDiscovery) ConvertToZones(clusters []*ClusterInfo) ([]*models.Zone, error) {
	var zones []*models.Zone

	for _, cluster := range clusters {
		zone, err := cd.convertClusterToZone(cluster)
		if err != nil {
			klog.Errorf("Failed to convert cluster %s to zone: %v", cluster.Name, err)
			continue
		}

		zones = append(zones, zone)
	}

	return zones, nil
}

// convertClusterToZone converts a single ClusterInfo to a Zone model
func (cd *ClusterDiscovery) convertClusterToZone(cluster *ClusterInfo) (*models.Zone, error) {
	// Generate zone ID and name
	zoneID := cd.generateZoneID(cluster.Name)
	zoneName := cd.generateZoneName(cluster.Name)

	// Map cluster status to zone status
	zoneStatus := cd.mapClusterStatusToZoneStatus(cluster.Status)

	// Calculate resource quotas based on capacity
	cpuQuota := cd.calculateQuota(cluster.CPUCores)
	memoryQuota := cd.calculateQuota(cluster.MemoryGB)
	storageQuota := cd.calculateQuota(cluster.StorageGB)

	// Create zone model
	zone := &models.Zone{
		ID:            zoneID,
		Name:          zoneName,
		ClusterName:   cluster.Name,
		APIUrl:        cluster.APIEndpoint,
		Status:        zoneStatus,
		Region:        cluster.Region,
		CloudProvider: cluster.Provider,

		// Node information
		NodeCount: cluster.NodeCount,

		// Capacity (actual cluster capacity)
		CPUCapacity:     cluster.CPUCores,
		MemoryCapacity:  cluster.MemoryGB,
		StorageCapacity: cluster.StorageGB,

		// Quota (allocatable to organizations)
		CPUQuota:     cpuQuota,
		MemoryQuota:  memoryQuota,
		StorageQuota: storageQuota,

		// Metadata
		Labels:      cd.generateZoneLabels(cluster),
		Annotations: cd.generateZoneAnnotations(cluster),

		// Timestamps
		LastSync:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return zone, nil
}

// generateZoneID creates a zone ID from cluster name
func (cd *ClusterDiscovery) generateZoneID(clusterName string) string {
	// Clean cluster name for use as ID
	id := strings.ToLower(clusterName)
	id = strings.ReplaceAll(id, "_", "-")
	id = strings.ReplaceAll(id, " ", "-")

	// Add prefix if configured
	if cd.config.ZonePrefix != "" {
		id = cd.config.ZonePrefix + "-" + id
	}

	return id
}

// generateZoneName creates a zone name from cluster name
func (cd *ClusterDiscovery) generateZoneName(clusterName string) string {
	// Use display name or cluster name
	name := clusterName

	// Add prefix if configured
	if cd.config.ZonePrefix != "" {
		name = cd.config.ZonePrefix + "-" + name
	}

	return name
}

// mapClusterStatusToZoneStatus maps ACM cluster status to zone status
func (cd *ClusterDiscovery) mapClusterStatusToZoneStatus(clusterStatus string) string {
	switch clusterStatus {
	case "available":
		return models.ZoneStatusAvailable
	case "maintenance":
		return models.ZoneStatusMaintenance
	case "unavailable", "pending-acceptance", "joining":
		return models.ZoneStatusUnavailable
	default:
		return models.ZoneStatusUnavailable
	}
}

// calculateQuota calculates the allocatable quota from capacity
func (cd *ClusterDiscovery) calculateQuota(capacity int) int {
	if capacity <= 0 {
		return 0
	}

	percentage := cd.config.DefaultQuotaPercentage
	if percentage <= 0 || percentage > 100 {
		percentage = 80 // Default to 80%
	}

	quota := (capacity * percentage) / 100
	return quota
}

// generateZoneLabels creates labels for the zone based on cluster info
func (cd *ClusterDiscovery) generateZoneLabels(cluster *ClusterInfo) models.StringMap {
	labels := make(models.StringMap)

	// Copy relevant cluster labels
	for k, v := range cluster.Labels {
		// Include known important labels
		if strings.Contains(k, "open-cluster-management.io") ||
			strings.Contains(k, "cluster.") ||
			k == "environment" ||
			k == "region" ||
			k == "provider" {
			labels[k] = v
		}
	}

	// Add OVIM-specific labels
	labels[LabelZoneType] = "acm-managed"
	labels[LabelManagedBy] = "ovim-acm-sync"
	labels["zone.ovim.io/cluster-name"] = cluster.Name
	labels["zone.ovim.io/sync-source"] = "acm"

	if cluster.Provider != "" {
		labels["zone.ovim.io/provider"] = cluster.Provider
	}
	if cluster.Region != "" {
		labels["zone.ovim.io/region"] = cluster.Region
	}

	return labels
}

// generateZoneAnnotations creates annotations for the zone
func (cd *ClusterDiscovery) generateZoneAnnotations(cluster *ClusterInfo) models.StringMap {
	annotations := make(models.StringMap)

	// Add descriptive information
	annotations["zone.ovim.io/description"] = fmt.Sprintf("ACM managed cluster: %s", cluster.Name)
	annotations["zone.ovim.io/cluster-api-endpoint"] = cluster.APIEndpoint
	annotations["zone.ovim.io/kubernetes-version"] = cluster.KubeVersion
	annotations["zone.ovim.io/last-discovered"] = time.Now().Format(time.RFC3339)

	// Add cluster claims as annotations
	for claimName, claimValue := range cluster.Claims {
		annotationKey := fmt.Sprintf("zone.ovim.io/claim-%s",
			strings.ReplaceAll(claimName, ".", "-"))
		annotations[annotationKey] = claimValue
	}

	// Copy some cluster annotations
	for k, v := range cluster.Annotations {
		if strings.Contains(k, "cluster.open-cluster-management.io") {
			annotations[k] = v
		}
	}

	return annotations
}

// isClusterExcluded checks if a cluster is in the exclusion list
func (cd *ClusterDiscovery) isClusterExcluded(clusterName string) bool {
	for _, excluded := range cd.config.ExcludedClusters {
		if excluded == clusterName {
			return true
		}
	}
	return false
}

// hasRequiredLabels checks if a cluster has all required labels
func (cd *ClusterDiscovery) hasRequiredLabels(cluster *ManagedCluster) bool {
	if len(cd.config.RequiredLabels) == 0 {
		return true // No required labels
	}

	for key, value := range cd.config.RequiredLabels {
		if clusterValue, exists := cluster.Labels[key]; !exists || clusterValue != value {
			return false
		}
	}

	return true
}

// enhanceClusterInfo adds additional processing to cluster info
func (cd *ClusterDiscovery) enhanceClusterInfo(info *ClusterInfo, cluster *ManagedCluster) {
	// Set defaults for missing values
	if info.NodeCount == 0 {
		info.NodeCount = 1 // Assume at least one node
	}

	// Estimate resources if not available
	if info.CPUCores == 0 || info.MemoryGB == 0 || info.StorageGB == 0 {
		cd.estimateResources(info)
	}

	// Validate and clean up values
	if info.CPUCores < 0 {
		info.CPUCores = 0
	}
	if info.MemoryGB < 0 {
		info.MemoryGB = 0
	}
	if info.StorageGB < 0 {
		info.StorageGB = 0
	}

	// Set provider if not detected
	if info.Provider == "" {
		info.Provider = cd.detectProvider(cluster)
	}
}

// estimateResources provides basic resource estimates if actual values aren't available
func (cd *ClusterDiscovery) estimateResources(info *ClusterInfo) {
	nodeCount := info.NodeCount
	if nodeCount == 0 {
		nodeCount = 1
	}

	// Basic estimates per node (conservative values)
	cpuPerNode := 4       // 4 cores per node
	memoryPerNode := 16   // 16 GB RAM per node
	storagePerNode := 100 // 100 GB storage per node

	if info.CPUCores == 0 {
		info.CPUCores = nodeCount * cpuPerNode
	}
	if info.MemoryGB == 0 {
		info.MemoryGB = nodeCount * memoryPerNode
	}
	if info.StorageGB == 0 {
		info.StorageGB = nodeCount * storagePerNode
	}

	klog.V(4).Infof("Estimated resources for cluster %s: CPU=%d, Memory=%dGB, Storage=%dGB",
		info.Name, info.CPUCores, info.MemoryGB, info.StorageGB)
}

// detectProvider attempts to detect the cloud provider from cluster metadata
func (cd *ClusterDiscovery) detectProvider(cluster *ManagedCluster) string {
	// Check labels for provider information
	if instanceType := cluster.Labels["node.kubernetes.io/instance-type"]; instanceType != "" {
		// AWS instance types (m5.large, t3.medium, etc.)
		if strings.Contains(instanceType, ".") && (strings.HasPrefix(instanceType, "m") ||
			strings.HasPrefix(instanceType, "t") || strings.HasPrefix(instanceType, "c") ||
			strings.HasPrefix(instanceType, "r") || strings.HasPrefix(instanceType, "i")) {
			return "aws"
		}
		// Azure instance types (Standard_D2s_v3, etc.)
		if strings.HasPrefix(instanceType, "Standard_") {
			return "azure"
		}
		// GCP instance types (e2-medium, n1-standard-1, etc.)
		if strings.Contains(instanceType, "-") && (strings.HasPrefix(instanceType, "e") ||
			strings.HasPrefix(instanceType, "n") || strings.HasPrefix(instanceType, "c2")) {
			return "gcp"
		}
	}

	// Check other labels
	for k, v := range cluster.Labels {
		if strings.Contains(k, "aws") || strings.Contains(v, "aws") {
			return "aws"
		}
		if strings.Contains(k, "azure") || strings.Contains(v, "azure") {
			return "azure"
		}
		if strings.Contains(k, "gcp") || strings.Contains(v, "gcp") {
			return "gcp"
		}
	}

	// Check annotations
	for k, v := range cluster.Annotations {
		if strings.Contains(k, "aws") || strings.Contains(v, "aws") {
			return "aws"
		}
		if strings.Contains(k, "azure") || strings.Contains(v, "azure") {
			return "azure"
		}
		if strings.Contains(k, "gcp") || strings.Contains(v, "gcp") {
			return "gcp"
		}
	}

	return "unknown"
}

// Helper functions for parsing resource quantities

// parseIntClaim parses an integer value from a cluster claim
func parseIntClaim(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
}

// parseResourceQuantity parses a Kubernetes resource quantity (e.g., "4" or "4000m")
func parseResourceQuantity(value string) (int, error) {
	qty, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, err
	}

	// Convert to integer (cores for CPU)
	return int(qty.Value()), nil
}

// parseMemoryToGB parses memory quantity and converts to GB
func parseMemoryToGB(value string) (int, error) {
	qty, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, err
	}

	// Convert to GB (divide by 1024^3)
	bytes := qty.Value()
	gb := bytes / (1024 * 1024 * 1024)
	return int(gb), nil
}

// parseStorageToGB parses storage quantity and converts to GB
func parseStorageToGB(value string) (int, error) {
	qty, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, err
	}

	// Convert to GB (divide by 1024^3)
	bytes := qty.Value()
	gb := bytes / (1024 * 1024 * 1024)
	return int(gb), nil
}
