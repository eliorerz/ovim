package v1

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOrganizationSpec(t *testing.T) {
	tests := []struct {
		name string
		spec OrganizationSpec
	}{
		{
			name: "complete organization spec",
			spec: OrganizationSpec{
				DisplayName: "Test Organization",
				Description: "A test organization for unit testing",
				Admins:      []string{"admin1", "admin2"},
				IsEnabled:   true,
				Catalogs: []CatalogReference{
					{
						Name:      "vm-catalog",
						Namespace: "ovim-catalogs",
						Type:      "vm-template",
					},
				},
			},
		},
		{
			name: "minimal organization spec",
			spec: OrganizationSpec{
				DisplayName: "Minimal Org",
				Admins:      []string{"admin"},
				IsEnabled:   false,
			},
		},
		{
			name: "empty organization spec",
			spec: OrganizationSpec{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.spec)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled OrganizationSpec
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.spec, unmarshaled)
		})
	}
}

func TestOrganizationStatus(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name   string
		status OrganizationStatus
	}{
		{
			name: "complete organization status",
			status: OrganizationStatus{
				Namespace: "org-test",
				Phase:     OrganizationPhaseActive,
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						LastTransitionTime: now,
						Reason:             "OrganizationReady",
						Message:            "Organization is ready and active",
					},
				},
				VDCCount:     5,
				LastRBACSync: &now,
			},
		},
		{
			name: "pending organization status",
			status: OrganizationStatus{
				Namespace: "org-pending",
				Phase:     OrganizationPhasePending,
				VDCCount:  0,
			},
		},
		{
			name: "failed organization status",
			status: OrganizationStatus{
				Namespace: "org-failed",
				Phase:     OrganizationPhaseFailed,
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						LastTransitionTime: now,
						Reason:             "NamespaceCreationFailed",
						Message:            "Failed to create organization namespace",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.status)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled OrganizationStatus
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			// Compare fields that don't involve time precision issues
			assert.Equal(t, tt.status.Namespace, unmarshaled.Namespace)
			assert.Equal(t, tt.status.Phase, unmarshaled.Phase)
			assert.Equal(t, tt.status.VDCCount, unmarshaled.VDCCount)

			// For conditions, compare all fields except precise timestamps
			assert.Equal(t, len(tt.status.Conditions), len(unmarshaled.Conditions))
			for i, cond := range tt.status.Conditions {
				if i < len(unmarshaled.Conditions) {
					assert.Equal(t, cond.Type, unmarshaled.Conditions[i].Type)
					assert.Equal(t, cond.Status, unmarshaled.Conditions[i].Status)
					assert.Equal(t, cond.Reason, unmarshaled.Conditions[i].Reason)
					assert.Equal(t, cond.Message, unmarshaled.Conditions[i].Message)
					// Time should be close but precision may differ due to JSON marshaling
					assert.WithinDuration(t, cond.LastTransitionTime.Time, unmarshaled.Conditions[i].LastTransitionTime.Time, time.Second)
				}
			}

			// Compare LastRBACSync if present (handling time precision)
			if tt.status.LastRBACSync != nil {
				require.NotNil(t, unmarshaled.LastRBACSync)
				assert.WithinDuration(t, tt.status.LastRBACSync.Time, unmarshaled.LastRBACSync.Time, time.Second)
			} else {
				assert.Nil(t, unmarshaled.LastRBACSync)
			}
		})
	}
}

func TestOrganizationPhases(t *testing.T) {
	tests := []struct {
		name  string
		phase OrganizationPhase
	}{
		{"pending phase", OrganizationPhasePending},
		{"active phase", OrganizationPhaseActive},
		{"failed phase", OrganizationPhaseFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.phase))

			// Test JSON marshaling
			data, err := json.Marshal(tt.phase)
			require.NoError(t, err)

			// Test JSON unmarshaling
			var unmarshaled OrganizationPhase
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.phase, unmarshaled)
		})
	}
}

func TestOrganization(t *testing.T) {
	org := Organization{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ovim.io/v1",
			Kind:       "Organization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
			Labels: map[string]string{
				"app.kubernetes.io/name": "ovim",
			},
		},
		Spec: OrganizationSpec{
			DisplayName: "Test Organization",
			Description: "Test description",
			Admins:      []string{"admin1"},
			IsEnabled:   true,
		},
		Status: OrganizationStatus{
			Namespace: "org-test-org",
			Phase:     OrganizationPhaseActive,
			VDCCount:  3,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(org)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test JSON unmarshaling
	var unmarshaled Organization
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, org, unmarshaled)

	// Test that required fields are present
	assert.Equal(t, "ovim.io/v1", org.APIVersion)
	assert.Equal(t, "Organization", org.Kind)
	assert.Equal(t, "test-org", org.Name)
}

func TestVirtualDataCenterSpec(t *testing.T) {
	tests := []struct {
		name string
		spec VirtualDataCenterSpec
	}{
		{
			name: "complete VDC spec",
			spec: VirtualDataCenterSpec{
				OrganizationRef: "test-org",
				DisplayName:     "Test VDC",
				Description:     "A test virtual data center",
				Quota: ResourceQuota{
					CPU:     "20",
					Memory:  "64Gi",
					Storage: "1Ti",
				},
				LimitRange: &LimitRange{
					MinCpu:    1,
					MaxCpu:    16,
					MinMemory: 2,
					MaxMemory: 32,
				},
				NetworkPolicy: "isolated",
			},
		},
		{
			name: "minimal VDC spec",
			spec: VirtualDataCenterSpec{
				OrganizationRef: "org1",
				DisplayName:     "Minimal VDC",
				Quota: ResourceQuota{
					CPU:     "10",
					Memory:  "32Gi",
					Storage: "500Gi",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.spec)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled VirtualDataCenterSpec
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.spec, unmarshaled)
		})
	}
}

func TestResourceQuota(t *testing.T) {
	tests := []struct {
		name  string
		quota ResourceQuota
	}{
		{
			name: "typical resource quota",
			quota: ResourceQuota{
				CPU:     "20",
				Memory:  "64Gi",
				Storage: "1Ti",
			},
		},
		{
			name: "minimal resource quota",
			quota: ResourceQuota{
				CPU:     "1",
				Memory:  "1Gi",
				Storage: "10Gi",
			},
		},
		{
			name: "large resource quota",
			quota: ResourceQuota{
				CPU:     "100",
				Memory:  "256Gi",
				Storage: "10Ti",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.quota)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled ResourceQuota
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.quota, unmarshaled)

			// Validate that all fields are strings (Kubernetes resource format)
			assert.IsType(t, "", tt.quota.CPU)
			assert.IsType(t, "", tt.quota.Memory)
			assert.IsType(t, "", tt.quota.Storage)
		})
	}
}

func TestLimitRange(t *testing.T) {
	tests := []struct {
		name       string
		limitRange LimitRange
		valid      bool
	}{
		{
			name: "valid limit range",
			limitRange: LimitRange{
				MinCpu:    1,
				MaxCpu:    16,
				MinMemory: 2,
				MaxMemory: 32,
			},
			valid: true,
		},
		{
			name: "equal min/max values",
			limitRange: LimitRange{
				MinCpu:    4,
				MaxCpu:    4,
				MinMemory: 8,
				MaxMemory: 8,
			},
			valid: true,
		},
		{
			name: "invalid CPU range",
			limitRange: LimitRange{
				MinCpu:    16,
				MaxCpu:    8,
				MinMemory: 2,
				MaxMemory: 32,
			},
			valid: false,
		},
		{
			name: "invalid memory range",
			limitRange: LimitRange{
				MinCpu:    1,
				MaxCpu:    16,
				MinMemory: 32,
				MaxMemory: 8,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.limitRange)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled LimitRange
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.limitRange, unmarshaled)

			// Validate logical constraints
			if tt.valid {
				assert.LessOrEqual(t, tt.limitRange.MinCpu, tt.limitRange.MaxCpu, "MinCpu should be <= MaxCpu")
				assert.LessOrEqual(t, tt.limitRange.MinMemory, tt.limitRange.MaxMemory, "MinMemory should be <= MaxMemory")
			} else {
				assert.True(t,
					tt.limitRange.MinCpu > tt.limitRange.MaxCpu || tt.limitRange.MinMemory > tt.limitRange.MaxMemory,
					"Should have invalid range constraints")
			}
		})
	}
}

func TestVirtualDataCenterStatus(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name   string
		status VirtualDataCenterStatus
	}{
		{
			name: "active VDC status with usage",
			status: VirtualDataCenterStatus{
				Namespace: "vdc-test-org-test-vdc",
				Phase:     VirtualDataCenterPhaseActive,
				ResourceUsage: &ResourceUsage{
					CPUUsed:     "5",
					MemoryUsed:  "16Gi",
					StorageUsed: "100Gi",
				},
				LastMetricsUpdate: &now,
				TotalPods:         10,
				TotalVMs:          3,
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						LastTransitionTime: now,
						Reason:             "VDCReady",
						Message:            "VDC is ready and active",
					},
				},
			},
		},
		{
			name: "pending VDC status",
			status: VirtualDataCenterStatus{
				Namespace: "vdc-pending",
				Phase:     VirtualDataCenterPhasePending,
				TotalPods: 0,
				TotalVMs:  0,
			},
		},
		{
			name: "suspended VDC status",
			status: VirtualDataCenterStatus{
				Namespace: "vdc-suspended",
				Phase:     VirtualDataCenterPhaseSuspended,
				TotalPods: 0,
				TotalVMs:  2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.status)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled VirtualDataCenterStatus
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			// Compare fields that don't involve time precision issues
			assert.Equal(t, tt.status.Namespace, unmarshaled.Namespace)
			assert.Equal(t, tt.status.Phase, unmarshaled.Phase)
			assert.Equal(t, tt.status.TotalPods, unmarshaled.TotalPods)
			assert.Equal(t, tt.status.TotalVMs, unmarshaled.TotalVMs)

			// Compare ResourceUsage
			if tt.status.ResourceUsage != nil {
				require.NotNil(t, unmarshaled.ResourceUsage)
				assert.Equal(t, tt.status.ResourceUsage.CPUUsed, unmarshaled.ResourceUsage.CPUUsed)
				assert.Equal(t, tt.status.ResourceUsage.MemoryUsed, unmarshaled.ResourceUsage.MemoryUsed)
				assert.Equal(t, tt.status.ResourceUsage.StorageUsed, unmarshaled.ResourceUsage.StorageUsed)
			} else {
				assert.Nil(t, unmarshaled.ResourceUsage)
			}

			// Compare LastMetricsUpdate (handling time precision)
			if tt.status.LastMetricsUpdate != nil {
				require.NotNil(t, unmarshaled.LastMetricsUpdate)
				assert.WithinDuration(t, tt.status.LastMetricsUpdate.Time, unmarshaled.LastMetricsUpdate.Time, time.Second)
			} else {
				assert.Nil(t, unmarshaled.LastMetricsUpdate)
			}

			// For conditions, compare all fields except precise timestamps
			assert.Equal(t, len(tt.status.Conditions), len(unmarshaled.Conditions))
			for i, cond := range tt.status.Conditions {
				if i < len(unmarshaled.Conditions) {
					assert.Equal(t, cond.Type, unmarshaled.Conditions[i].Type)
					assert.Equal(t, cond.Status, unmarshaled.Conditions[i].Status)
					assert.Equal(t, cond.Reason, unmarshaled.Conditions[i].Reason)
					assert.Equal(t, cond.Message, unmarshaled.Conditions[i].Message)
					assert.WithinDuration(t, cond.LastTransitionTime.Time, unmarshaled.Conditions[i].LastTransitionTime.Time, time.Second)
				}
			}
		})
	}
}

func TestResourceUsage(t *testing.T) {
	tests := []struct {
		name  string
		usage ResourceUsage
	}{
		{
			name: "typical resource usage",
			usage: ResourceUsage{
				CPUUsed:     "10",
				MemoryUsed:  "32Gi",
				StorageUsed: "500Gi",
			},
		},
		{
			name: "zero resource usage",
			usage: ResourceUsage{
				CPUUsed:     "0",
				MemoryUsed:  "0Gi",
				StorageUsed: "0Gi",
			},
		},
		{
			name: "fractional CPU usage",
			usage: ResourceUsage{
				CPUUsed:     "2.5",
				MemoryUsed:  "8Gi",
				StorageUsed: "100Gi",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.usage)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled ResourceUsage
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.usage, unmarshaled)

			// Validate that all fields are strings (Kubernetes resource format)
			assert.IsType(t, "", tt.usage.CPUUsed)
			assert.IsType(t, "", tt.usage.MemoryUsed)
			assert.IsType(t, "", tt.usage.StorageUsed)
		})
	}
}

func TestVirtualDataCenterPhases(t *testing.T) {
	tests := []struct {
		name  string
		phase VirtualDataCenterPhase
	}{
		{"pending phase", VirtualDataCenterPhasePending},
		{"active phase", VirtualDataCenterPhaseActive},
		{"failed phase", VirtualDataCenterPhaseFailed},
		{"suspended phase", VirtualDataCenterPhaseSuspended},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.phase))

			// Test JSON marshaling
			data, err := json.Marshal(tt.phase)
			require.NoError(t, err)

			// Test JSON unmarshaling
			var unmarshaled VirtualDataCenterPhase
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.phase, unmarshaled)
		})
	}
}

func TestVirtualDataCenter(t *testing.T) {
	vdc := VirtualDataCenter{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ovim.io/v1",
			Kind:       "VirtualDataCenter",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vdc",
			Namespace: "org-test",
			Labels: map[string]string{
				"ovim.io/organization": "test-org",
			},
		},
		Spec: VirtualDataCenterSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test VDC",
			Description:     "Test VDC description",
			Quota: ResourceQuota{
				CPU:     "20",
				Memory:  "64Gi",
				Storage: "1Ti",
			},
		},
		Status: VirtualDataCenterStatus{
			Namespace: "vdc-test-org-test-vdc",
			Phase:     VirtualDataCenterPhaseActive,
			TotalPods: 5,
			TotalVMs:  2,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(vdc)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test JSON unmarshaling
	var unmarshaled VirtualDataCenter
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, vdc, unmarshaled)

	// Test that required fields are present
	assert.Equal(t, "ovim.io/v1", vdc.APIVersion)
	assert.Equal(t, "VirtualDataCenter", vdc.Kind)
	assert.Equal(t, "test-vdc", vdc.Name)
	assert.Equal(t, "org-test", vdc.Namespace)
}

func TestCatalogReference(t *testing.T) {
	tests := []struct {
		name string
		ref  CatalogReference
	}{
		{
			name: "vm template catalog reference",
			ref: CatalogReference{
				Name:      "vm-templates",
				Namespace: "ovim-catalogs",
				Type:      "vm-template",
			},
		},
		{
			name: "application stack catalog reference",
			ref: CatalogReference{
				Name:      "app-stacks",
				Namespace: "ovim-catalogs",
				Type:      "application-stack",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.ref)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled CatalogReference
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.ref, unmarshaled)

			// Validate required fields
			assert.NotEmpty(t, tt.ref.Name, "Name should not be empty")
			assert.NotEmpty(t, tt.ref.Namespace, "Namespace should not be empty")
			assert.NotEmpty(t, tt.ref.Type, "Type should not be empty")
		})
	}
}

func TestCatalogSpec(t *testing.T) {
	tests := []struct {
		name string
		spec CatalogSpec
	}{
		{
			name: "git catalog spec",
			spec: CatalogSpec{
				OrganizationRef: "test-org",
				DisplayName:     "VM Templates Catalog",
				Description:     "Catalog containing VM templates",
				Type:            "vm-template",
				Source: CatalogSource{
					Type:        "git",
					URL:         "https://github.com/example/vm-templates.git",
					Credentials: "git-credentials-secret",
				},
			},
		},
		{
			name: "oci catalog spec",
			spec: CatalogSpec{
				OrganizationRef: "test-org",
				DisplayName:     "OCI Catalog",
				Type:            "application-stack",
				Source: CatalogSource{
					Type: "oci",
					URL:  "registry.example.com/app-stacks",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.spec)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled CatalogSpec
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.spec, unmarshaled)

			// Validate required fields
			assert.NotEmpty(t, tt.spec.OrganizationRef, "OrganizationRef should not be empty")
			assert.NotEmpty(t, tt.spec.DisplayName, "DisplayName should not be empty")
			assert.NotEmpty(t, tt.spec.Type, "Type should not be empty")
		})
	}
}

func TestCatalogSource(t *testing.T) {
	tests := []struct {
		name   string
		source CatalogSource
	}{
		{
			name: "git source with credentials",
			source: CatalogSource{
				Type:        "git",
				URL:         "https://github.com/example/catalog.git",
				Credentials: "git-secret",
			},
		},
		{
			name: "oci source without credentials",
			source: CatalogSource{
				Type: "oci",
				URL:  "registry.example.com/catalog",
			},
		},
		{
			name: "s3 source",
			source: CatalogSource{
				Type:        "s3",
				URL:         "s3://bucket/catalog",
				Credentials: "s3-credentials",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.source)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled CatalogSource
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.source, unmarshaled)

			// Validate required fields
			assert.NotEmpty(t, tt.source.Type, "Type should not be empty")
			assert.NotEmpty(t, tt.source.URL, "URL should not be empty")
		})
	}
}

func TestCatalogStatus(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name   string
		status CatalogStatus
	}{
		{
			name: "ready catalog status",
			status: CatalogStatus{
				Phase:     CatalogPhaseReady,
				ItemCount: 25,
				LastSync:  &now,
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						LastTransitionTime: now,
						Reason:             "CatalogSynced",
						Message:            "Catalog synchronized successfully",
					},
				},
			},
		},
		{
			name: "failed catalog status",
			status: CatalogStatus{
				Phase:     CatalogPhaseFailed,
				ItemCount: 0,
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionFalse,
						LastTransitionTime: now,
						Reason:             "SyncFailed",
						Message:            "Failed to sync catalog from source",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.status)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var unmarshaled CatalogStatus
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			// Compare fields that don't involve time precision issues
			assert.Equal(t, tt.status.Phase, unmarshaled.Phase)
			assert.Equal(t, tt.status.ItemCount, unmarshaled.ItemCount)

			// Compare LastSync (handling time precision)
			if tt.status.LastSync != nil {
				require.NotNil(t, unmarshaled.LastSync)
				assert.WithinDuration(t, tt.status.LastSync.Time, unmarshaled.LastSync.Time, time.Second)
			} else {
				assert.Nil(t, unmarshaled.LastSync)
			}

			// For conditions, compare all fields except precise timestamps
			assert.Equal(t, len(tt.status.Conditions), len(unmarshaled.Conditions))
			for i, cond := range tt.status.Conditions {
				if i < len(unmarshaled.Conditions) {
					assert.Equal(t, cond.Type, unmarshaled.Conditions[i].Type)
					assert.Equal(t, cond.Status, unmarshaled.Conditions[i].Status)
					assert.Equal(t, cond.Reason, unmarshaled.Conditions[i].Reason)
					assert.Equal(t, cond.Message, unmarshaled.Conditions[i].Message)
					assert.WithinDuration(t, cond.LastTransitionTime.Time, unmarshaled.Conditions[i].LastTransitionTime.Time, time.Second)
				}
			}
		})
	}
}

func TestCatalogPhases(t *testing.T) {
	tests := []struct {
		name  string
		phase CatalogPhase
	}{
		{"pending phase", CatalogPhasePending},
		{"ready phase", CatalogPhaseReady},
		{"failed phase", CatalogPhaseFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.phase))

			// Test JSON marshaling
			data, err := json.Marshal(tt.phase)
			require.NoError(t, err)

			// Test JSON unmarshaling
			var unmarshaled CatalogPhase
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.phase, unmarshaled)
		})
	}
}

func TestCatalog(t *testing.T) {
	catalog := Catalog{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ovim.io/v1",
			Kind:       "Catalog",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-catalog",
			Namespace: "org-test",
			Labels: map[string]string{
				"ovim.io/organization": "test-org",
			},
		},
		Spec: CatalogSpec{
			OrganizationRef: "test-org",
			DisplayName:     "Test Catalog",
			Description:     "Test catalog description",
			Type:            "vm-template",
			Source: CatalogSource{
				Type: "git",
				URL:  "https://github.com/example/catalog.git",
			},
		},
		Status: CatalogStatus{
			Phase:     CatalogPhaseReady,
			ItemCount: 10,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(catalog)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test JSON unmarshaling
	var unmarshaled Catalog
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, catalog, unmarshaled)

	// Test that required fields are present
	assert.Equal(t, "ovim.io/v1", catalog.APIVersion)
	assert.Equal(t, "Catalog", catalog.Kind)
	assert.Equal(t, "test-catalog", catalog.Name)
	assert.Equal(t, "org-test", catalog.Namespace)
}
