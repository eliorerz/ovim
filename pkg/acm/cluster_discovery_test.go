package acm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/eliorerz/ovim-updated/pkg/models"
)

// Simplified tests that don't rely on complex mocks

func TestClusterDiscovery_GenerateZoneID(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		prefix      string
		expectedID  string
	}{
		{
			name:        "Simple cluster name",
			clusterName: "production-cluster",
			prefix:      "",
			expectedID:  "production-cluster",
		},
		{
			name:        "Cluster name with prefix",
			clusterName: "production-cluster",
			prefix:      "acm",
			expectedID:  "acm-production-cluster",
		},
		{
			name:        "Cluster name with spaces and underscores",
			clusterName: "My_Test Cluster",
			prefix:      "",
			expectedID:  "my-test-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SyncConfig{
				ZonePrefix: tt.prefix,
			}

			discovery := &ClusterDiscovery{
				config: config,
			}

			id := discovery.generateZoneID(tt.clusterName)
			assert.Equal(t, tt.expectedID, id)
		})
	}
}

func TestClusterDiscovery_GenerateZoneName(t *testing.T) {
	tests := []struct {
		name         string
		clusterName  string
		prefix       string
		expectedName string
	}{
		{
			name:         "Simple cluster name",
			clusterName:  "production-cluster",
			prefix:       "",
			expectedName: "production-cluster",
		},
		{
			name:         "Cluster name with prefix",
			clusterName:  "production-cluster",
			prefix:       "acm",
			expectedName: "acm-production-cluster",
		},
		{
			name:         "Cluster name with spaces",
			clusterName:  "My Test Cluster",
			prefix:       "",
			expectedName: "My Test Cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SyncConfig{
				ZonePrefix: tt.prefix,
			}

			discovery := &ClusterDiscovery{
				config: config,
			}

			name := discovery.generateZoneName(tt.clusterName)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}

func TestClusterDiscovery_MapClusterStatusToZoneStatus(t *testing.T) {
	discovery := &ClusterDiscovery{}

	tests := []struct {
		clusterStatus string
		expectedZone  string
	}{
		{"available", models.ZoneStatusAvailable},
		{"maintenance", models.ZoneStatusMaintenance},
		{"unavailable", models.ZoneStatusUnavailable},
		{"pending-acceptance", models.ZoneStatusUnavailable},
		{"joining", models.ZoneStatusUnavailable},
		{"unknown", models.ZoneStatusUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.clusterStatus, func(t *testing.T) {
			zoneStatus := discovery.mapClusterStatusToZoneStatus(tt.clusterStatus)
			assert.Equal(t, tt.expectedZone, zoneStatus)
		})
	}
}

func TestClusterDiscovery_CalculateQuota(t *testing.T) {
	config := SyncConfig{
		DefaultQuotaPercentage: 75,
	}

	discovery := &ClusterDiscovery{
		config: config,
	}

	tests := []struct {
		capacity int
		expected int
	}{
		{100, 75}, // 75% of 100
		{50, 37},  // 75% of 50 (37.5 rounded down)
		{0, 0},    // Zero capacity
		{-10, 0},  // Negative capacity
	}

	for _, tt := range tests {
		result := discovery.calculateQuota(tt.capacity)
		assert.Equal(t, tt.expected, result)
	}
}

func TestClusterDiscovery_IsClusterExcluded(t *testing.T) {
	config := SyncConfig{
		ExcludedClusters: []string{"excluded-1", "excluded-2"},
	}

	discovery := &ClusterDiscovery{
		config: config,
	}

	assert.True(t, discovery.isClusterExcluded("excluded-1"))
	assert.True(t, discovery.isClusterExcluded("excluded-2"))
	assert.False(t, discovery.isClusterExcluded("included-cluster"))
}

func TestClusterDiscovery_HasRequiredLabels(t *testing.T) {
	tests := []struct {
		name           string
		requiredLabels map[string]string
		clusterLabels  map[string]string
		expected       bool
	}{
		{
			name:           "No required labels",
			requiredLabels: map[string]string{},
			clusterLabels:  map[string]string{"any": "value"},
			expected:       true,
		},
		{
			name:           "Required labels present",
			requiredLabels: map[string]string{"env": "prod", "region": "us-west"},
			clusterLabels:  map[string]string{"env": "prod", "region": "us-west", "extra": "label"},
			expected:       true,
		},
		{
			name:           "Missing required label",
			requiredLabels: map[string]string{"env": "prod", "region": "us-west"},
			clusterLabels:  map[string]string{"env": "prod"}, // Missing region
			expected:       false,
		},
		{
			name:           "Wrong required label value",
			requiredLabels: map[string]string{"env": "prod"},
			clusterLabels:  map[string]string{"env": "dev"}, // Wrong value
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SyncConfig{
				RequiredLabels: tt.requiredLabels,
			}

			discovery := &ClusterDiscovery{
				config: config,
			}

			cluster := &ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tt.clusterLabels,
				},
			}

			result := discovery.hasRequiredLabels(cluster)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClusterDiscovery_DetectProvider(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *ManagedCluster
		expected string
	}{
		{
			name: "AWS detection from instance type",
			cluster: &ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node.kubernetes.io/instance-type": "m5.large",
					},
				},
			},
			expected: "aws",
		},
		{
			name: "Azure detection from instance type",
			cluster: &ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node.kubernetes.io/instance-type": "Standard_D2s_v3",
					},
				},
			},
			expected: "azure",
		},
		{
			name: "GCP detection from labels",
			cluster: &ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cloud.google.com/gke-nodepool": "default-pool",
					},
				},
			},
			expected: "gcp",
		},
		{
			name: "AWS detection from annotations",
			cluster: &ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"eks.amazonaws.com/nodegroup-name": "workers",
					},
				},
			},
			expected: "aws",
		},
		{
			name: "Unknown provider",
			cluster: &ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"some.other.label": "value",
					},
				},
			},
			expected: "unknown",
		},
	}

	config := SyncConfig{}
	discovery := &ClusterDiscovery{
		config: config,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := discovery.detectProvider(tt.cluster)
			assert.Equal(t, tt.expected, provider)
		})
	}
}

func TestClusterDiscovery_GenerateZoneLabels(t *testing.T) {
	cluster := &ClusterInfo{
		Name:     "test-cluster",
		Provider: "aws",
		Region:   "us-west-2",
		Labels: map[string]string{
			"environment": "production",
			"cluster.open-cluster-management.io/clustername": "test-cluster",
			"custom-label": "should-be-excluded",
		},
	}

	discovery := &ClusterDiscovery{}
	labels := discovery.generateZoneLabels(cluster)

	// Check OVIM-specific labels
	assert.Equal(t, "acm-managed", labels[LabelZoneType])
	assert.Equal(t, "ovim-acm-sync", labels[LabelManagedBy])
	assert.Equal(t, "test-cluster", labels["zone.ovim.io/cluster-name"])
	assert.Equal(t, "acm", labels["zone.ovim.io/sync-source"])
	assert.Equal(t, "aws", labels["zone.ovim.io/provider"])
	assert.Equal(t, "us-west-2", labels["zone.ovim.io/region"])

	// Check included cluster labels
	assert.Equal(t, "production", labels["environment"])
	assert.Equal(t, "test-cluster", labels["cluster.open-cluster-management.io/clustername"])

	// Check excluded label
	_, exists := labels["custom-label"]
	assert.False(t, exists)
}

func TestClusterDiscovery_GenerateZoneAnnotations(t *testing.T) {
	cluster := &ClusterInfo{
		Name:        "test-cluster",
		APIEndpoint: "https://api.test.example.com:6443",
		KubeVersion: "v1.24.0",
		Claims: map[string]string{
			ClusterClaimProvider: "aws",
			ClusterClaimRegion:   "us-west-2",
		},
		Annotations: map[string]string{
			"cluster.open-cluster-management.io/version": "v1.24.0",
			"custom-annotation":                          "should-be-excluded",
		},
	}

	discovery := &ClusterDiscovery{}
	annotations := discovery.generateZoneAnnotations(cluster)

	// Check required annotations
	assert.Contains(t, annotations["zone.ovim.io/description"], "test-cluster")
	assert.Equal(t, "https://api.test.example.com:6443", annotations["zone.ovim.io/cluster-api-endpoint"])
	assert.Equal(t, "v1.24.0", annotations["zone.ovim.io/kubernetes-version"])

	// Check claim annotations
	assert.Equal(t, "aws", annotations["zone.ovim.io/claim-provider-open-cluster-management-io"])
	assert.Equal(t, "us-west-2", annotations["zone.ovim.io/claim-region-open-cluster-management-io"])

	// Check included cluster annotations
	assert.Equal(t, "v1.24.0", annotations["cluster.open-cluster-management.io/version"])

	// Check excluded annotation
	_, exists := annotations["custom-annotation"]
	assert.False(t, exists)
}

func TestClusterDiscovery_EstimateResources(t *testing.T) {
	discovery := &ClusterDiscovery{}

	tests := []struct {
		name     string
		info     *ClusterInfo
		expected *ClusterInfo
	}{
		{
			name: "Estimate for 1 node with missing resources",
			info: &ClusterInfo{
				NodeCount: 1,
				CPUCores:  0,
				MemoryGB:  0,
				StorageGB: 0,
			},
			expected: &ClusterInfo{
				NodeCount: 1,
				CPUCores:  4,   // 1 * 4
				MemoryGB:  16,  // 1 * 16
				StorageGB: 100, // 1 * 100
			},
		},
		{
			name: "Estimate for 3 nodes with missing resources",
			info: &ClusterInfo{
				NodeCount: 3,
				CPUCores:  0,
				MemoryGB:  0,
				StorageGB: 0,
			},
			expected: &ClusterInfo{
				NodeCount: 3,
				CPUCores:  12,  // 3 * 4
				MemoryGB:  48,  // 3 * 16
				StorageGB: 300, // 3 * 100
			},
		},
		{
			name: "Don't override existing values",
			info: &ClusterInfo{
				NodeCount: 2,
				CPUCores:  8,   // Already set
				MemoryGB:  0,   // Missing
				StorageGB: 200, // Already set
			},
			expected: &ClusterInfo{
				NodeCount: 2,
				CPUCores:  8,   // Unchanged
				MemoryGB:  32,  // 2 * 16
				StorageGB: 200, // Unchanged
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery.estimateResources(tt.info)
			assert.Equal(t, tt.expected.NodeCount, tt.info.NodeCount)
			assert.Equal(t, tt.expected.CPUCores, tt.info.CPUCores)
			assert.Equal(t, tt.expected.MemoryGB, tt.info.MemoryGB)
			assert.Equal(t, tt.expected.StorageGB, tt.info.StorageGB)
		})
	}
}
