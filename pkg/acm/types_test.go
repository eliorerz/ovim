package acm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManagedClusterDeepCopy(t *testing.T) {
	original := &ManagedCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ManagedCluster",
			APIVersion: "cluster.open-cluster-management.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"environment": "test",
				"region":      "us-west-2",
			},
		},
		Spec: ManagedClusterSpec{
			HubAcceptsClient: true,
			ManagedClusterClientConfigs: []ClientConfig{
				{
					URL:      "https://api.test-cluster.example.com:6443",
					CABundle: []byte("test-ca-bundle"),
				},
			},
			Taints: []Taint{
				{
					Key:    "test-taint",
					Value:  "test-value",
					Effect: TaintEffectNoSelect,
				},
			},
		},
		Status: ManagedClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   string(ManagedClusterConditionAvailable),
					Status: "True",
				},
			},
			Capacity: ResourceList{
				"cpu":    "8",
				"memory": "32Gi",
			},
			Version: ManagedClusterVersion{
				Kubernetes: "v1.24.0",
			},
			ClusterClaims: []ManagedClusterClaim{
				{
					Name:  ClusterClaimProvider,
					Value: "aws",
				},
				{
					Name:  ClusterClaimRegion,
					Value: "us-west-2",
				},
			},
		},
	}

	// Test deep copy
	copied := original.DeepCopy()

	// Verify the copy is not the same object
	assert.NotSame(t, original, copied)

	// Verify all fields are copied correctly
	assert.Equal(t, original.TypeMeta, copied.TypeMeta)
	assert.Equal(t, original.ObjectMeta.Name, copied.ObjectMeta.Name)
	assert.Equal(t, original.ObjectMeta.Namespace, copied.ObjectMeta.Namespace)
	assert.Equal(t, original.Spec.HubAcceptsClient, copied.Spec.HubAcceptsClient)
	assert.Equal(t, original.Status.Version.Kubernetes, copied.Status.Version.Kubernetes)

	// Verify slices are deep copied (different memory addresses)
	assert.NotEqual(t, &original.Spec.ManagedClusterClientConfigs, &copied.Spec.ManagedClusterClientConfigs)
	assert.NotEqual(t, &original.Spec.Taints, &copied.Spec.Taints)
	assert.NotEqual(t, &original.Status.Conditions, &copied.Status.Conditions)
	assert.NotEqual(t, &original.Status.ClusterClaims, &copied.Status.ClusterClaims)

	// Verify maps are deep copied (different memory addresses)
	assert.NotEqual(t, &original.ObjectMeta.Labels, &copied.ObjectMeta.Labels)
	assert.NotEqual(t, &original.Status.Capacity, &copied.Status.Capacity)

	// Modify original and verify copy is unchanged
	original.ObjectMeta.Name = "modified-name"
	original.ObjectMeta.Labels["new-label"] = "new-value"
	original.Status.Capacity["storage"] = "100Gi"

	assert.Equal(t, "test-cluster", copied.ObjectMeta.Name)
	assert.NotContains(t, copied.ObjectMeta.Labels, "new-label")
	assert.NotContains(t, copied.Status.Capacity, "storage")
}

func TestManagedClusterListDeepCopy(t *testing.T) {
	original := &ManagedClusterList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ManagedClusterList",
			APIVersion: "cluster.open-cluster-management.io/v1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "12345",
		},
		Items: []ManagedCluster{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-2",
				},
			},
		},
	}

	// Test deep copy
	copied := original.DeepCopy()

	// Verify the copy is not the same object
	assert.NotSame(t, original, copied)

	// Verify all fields are copied correctly
	assert.Equal(t, original.TypeMeta, copied.TypeMeta)
	assert.Equal(t, original.ListMeta.ResourceVersion, copied.ListMeta.ResourceVersion)
	assert.Len(t, copied.Items, 2)

	// Verify items slice is deep copied
	assert.NotEqual(t, &original.Items, &copied.Items)
	assert.Equal(t, "cluster-1", copied.Items[0].ObjectMeta.Name)
	assert.Equal(t, "cluster-2", copied.Items[1].ObjectMeta.Name)

	// Modify original and verify copy is unchanged
	original.Items[0].ObjectMeta.Name = "modified-cluster-1"
	assert.Equal(t, "cluster-1", copied.Items[0].ObjectMeta.Name)
}

func TestTaintDeepCopy(t *testing.T) {
	now := metav1.Now()
	original := &Taint{
		Key:       "test-key",
		Value:     "test-value",
		Effect:    TaintEffectNoSelect,
		TimeAdded: &now,
	}

	// Create a copy using the struct method
	var copied Taint
	original.DeepCopyInto(&copied)

	// Verify fields are copied
	assert.Equal(t, original.Key, copied.Key)
	assert.Equal(t, original.Value, copied.Value)
	assert.Equal(t, original.Effect, copied.Effect)

	// Verify TimeAdded is deep copied
	assert.NotSame(t, original.TimeAdded, copied.TimeAdded)
	assert.Equal(t, original.TimeAdded.Time, copied.TimeAdded.Time)

	// Modify original and verify copy is unchanged
	original.Key = "modified-key"
	assert.Equal(t, "test-key", copied.Key)
}

func TestClusterInfoStructure(t *testing.T) {
	info := &ClusterInfo{
		Name:        "test-cluster",
		DisplayName: "Test Cluster",
		APIEndpoint: "https://api.test.example.com:6443",
		Status:      "available",
		KubeVersion: "v1.24.0",
		Region:      "us-west-2",
		Provider:    "aws",
		NodeCount:   3,
		CPUCores:    12,
		MemoryGB:    48,
		StorageGB:   300,
		Labels:      map[string]string{"env": "test"},
		Annotations: map[string]string{"desc": "test cluster"},
		Claims:      map[string]string{ClusterClaimProvider: "aws"},
		Available:   true,
		Accepted:    true,
		LastSeen:    time.Now(),
	}

	// Test that all fields are accessible
	assert.Equal(t, "test-cluster", info.Name)
	assert.Equal(t, "Test Cluster", info.DisplayName)
	assert.Equal(t, "https://api.test.example.com:6443", info.APIEndpoint)
	assert.Equal(t, "available", info.Status)
	assert.Equal(t, "v1.24.0", info.KubeVersion)
	assert.Equal(t, "us-west-2", info.Region)
	assert.Equal(t, "aws", info.Provider)
	assert.Equal(t, 3, info.NodeCount)
	assert.Equal(t, 12, info.CPUCores)
	assert.Equal(t, 48, info.MemoryGB)
	assert.Equal(t, 300, info.StorageGB)
	assert.True(t, info.Available)
	assert.True(t, info.Accepted)
	assert.NotZero(t, info.LastSeen)

	// Test maps
	assert.Equal(t, "test", info.Labels["env"])
	assert.Equal(t, "test cluster", info.Annotations["desc"])
	assert.Equal(t, "aws", info.Claims[ClusterClaimProvider])
}

func TestSyncConfigValidation(t *testing.T) {
	// Test valid config
	config := SyncConfig{
		Enabled:                true,
		Interval:               5 * time.Minute,
		Namespace:              "test-namespace",
		AutoCreateZones:        true,
		DefaultQuotaPercentage: 80,
		ExcludedClusters:       []string{"cluster1", "cluster2"},
		RequiredLabels:         map[string]string{"env": "prod"},
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 5*time.Minute, config.Interval)
	assert.Equal(t, "test-namespace", config.Namespace)
	assert.True(t, config.AutoCreateZones)
	assert.Equal(t, 80, config.DefaultQuotaPercentage)
	assert.Len(t, config.ExcludedClusters, 2)
	assert.Equal(t, "prod", config.RequiredLabels["env"])
}

func TestSyncResult(t *testing.T) {
	result := &SyncResult{
		Timestamp:        time.Now(),
		Success:          true,
		ClustersFound:    5,
		ZonesCreated:     2,
		ZonesUpdated:     3,
		ZonesDeleted:     1,
		ProcessingTimeMs: 1500,
	}

	assert.True(t, result.Success)
	assert.Equal(t, 5, result.ClustersFound)
	assert.Equal(t, 2, result.ZonesCreated)
	assert.Equal(t, 3, result.ZonesUpdated)
	assert.Equal(t, 1, result.ZonesDeleted)
	assert.Equal(t, int64(1500), result.ProcessingTimeMs)
	assert.Empty(t, result.ErrorMessage)
}

func TestTaintEffects(t *testing.T) {
	effects := []TaintEffect{
		TaintEffectNoSelect,
		TaintEffectPreferNoSelect,
		TaintEffectNoSelectIfNew,
	}

	assert.Equal(t, TaintEffect("NoSelect"), effects[0])
	assert.Equal(t, TaintEffect("PreferNoSelect"), effects[1])
	assert.Equal(t, TaintEffect("NoSelectIfNew"), effects[2])
}

func TestClusterConditionTypes(t *testing.T) {
	conditions := []ClusterConditionType{
		ManagedClusterConditionAvailable,
		ManagedClusterConditionHubAccepted,
		ManagedClusterConditionJoined,
	}

	assert.Equal(t, ClusterConditionType("ManagedClusterConditionAvailable"), conditions[0])
	assert.Equal(t, ClusterConditionType("HubClusterAccepted"), conditions[1])
	assert.Equal(t, ClusterConditionType("ManagedClusterJoined"), conditions[2])
}

func TestWellKnownConstants(t *testing.T) {
	// Test cluster claim constants
	assert.Equal(t, "platform.open-cluster-management.io", ClusterClaimPlatform)
	assert.Equal(t, "region.open-cluster-management.io", ClusterClaimRegion)
	assert.Equal(t, "provider.open-cluster-management.io", ClusterClaimProvider)
	assert.Equal(t, "version.open-cluster-management.io", ClusterClaimVersion)
	assert.Equal(t, "node.count.open-cluster-management.io", ClusterClaimNodeCount)
	assert.Equal(t, "cpu.cores.open-cluster-management.io", ClusterClaimCPUCores)
	assert.Equal(t, "memory.gb.open-cluster-management.io", ClusterClaimMemoryGB)
	assert.Equal(t, "storage.gb.open-cluster-management.io", ClusterClaimStorageGB)

	// Test label constants
	assert.Equal(t, "cluster.open-cluster-management.io/clustername", LabelClusterName)
	assert.Equal(t, "cluster.open-cluster-management.io/provider", LabelClusterProvider)
	assert.Equal(t, "cluster.open-cluster-management.io/region", LabelClusterRegion)
	assert.Equal(t, "environment", LabelEnvironment)
	assert.Equal(t, "managed-by", LabelManagedBy)
	assert.Equal(t, "zone.ovim.io/type", LabelZoneType)
	assert.Equal(t, "zone.ovim.io/default", LabelZoneDefault)
}
