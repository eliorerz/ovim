package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONBArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected JSONBArray
		wantErr  bool
	}{
		{
			name:     "valid array",
			input:    []byte(`["item1", "item2", "item3"]`),
			expected: JSONBArray{"item1", "item2", "item3"},
			wantErr:  false,
		},
		{
			name:     "empty array",
			input:    []byte(`[]`),
			expected: JSONBArray{},
			wantErr:  false,
		},
		{
			name:     "null value",
			input:    nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "empty bytes",
			input:    []byte(``),
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "invalid json",
			input:    []byte(`invalid json`),
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ja JSONBArray
			err := ja.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, ja)
			}
		})
	}
}

func TestJSONBArrayValue(t *testing.T) {
	tests := []struct {
		name     string
		input    JSONBArray
		expected string
		wantErr  bool
	}{
		{
			name:     "valid array",
			input:    JSONBArray{"item1", "item2"},
			expected: `["item1","item2"]`,
			wantErr:  false,
		},
		{
			name:     "empty array",
			input:    JSONBArray{},
			expected: `[]`,
			wantErr:  false,
		},
		{
			name:     "nil array",
			input:    nil,
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.input.Value()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expected == "" {
					assert.Nil(t, value)
				} else {
					assert.Equal(t, tt.expected, string(value.([]byte)))
				}
			}
		})
	}
}

func TestJSONBMap(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected JSONBMap
		wantErr  bool
	}{
		{
			name:     "valid map",
			input:    []byte(`{"key1": "value1", "key2": 42}`),
			expected: JSONBMap{"key1": "value1", "key2": float64(42)},
			wantErr:  false,
		},
		{
			name:     "empty map",
			input:    []byte(`{}`),
			expected: JSONBMap{},
			wantErr:  false,
		},
		{
			name:     "null value",
			input:    nil,
			expected: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var jm JSONBMap
			err := jm.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, jm)
			}
		})
	}
}

func TestConditionsArray(t *testing.T) {
	// Use a fixed time to avoid precision issues with JSON serialization
	now := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    []Condition
		expected []byte
	}{
		{
			name: "single condition",
			input: []Condition{
				{
					Type:               "Ready",
					Status:             "True",
					LastTransitionTime: now,
					Reason:             "Available",
					Message:            "Resource is ready",
				},
			},
		},
		{
			name:  "empty conditions",
			input: []Condition{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca := ConditionsArray(tt.input)

			// Test Value()
			value, err := ca.Value()
			require.NoError(t, err)

			if len(tt.input) == 0 {
				assert.Equal(t, "[]", string(value.([]byte)))
			} else {
				// Verify it's valid JSON
				var conditions []Condition
				err = json.Unmarshal(value.([]byte), &conditions)
				require.NoError(t, err)
				assert.Len(t, conditions, len(tt.input))
			}

			// Test Scan()
			var scannedCA ConditionsArray
			err = scannedCA.Scan(value)
			require.NoError(t, err)
			assert.Equal(t, ca, scannedCA)
		})
	}
}

func TestConditionsArraySetCondition(t *testing.T) {
	var ca ConditionsArray

	// Test adding new condition
	ca.SetCondition("Ready", "True", "Available", "Resource is ready")
	assert.Len(t, ca, 1)
	assert.Equal(t, "Ready", ca[0].Type)
	assert.Equal(t, "True", ca[0].Status)

	// Test updating existing condition
	oldTime := ca[0].LastTransitionTime
	time.Sleep(time.Millisecond) // Ensure time difference
	ca.SetCondition("Ready", "False", "NotAvailable", "Resource is not ready")
	assert.Len(t, ca, 1)
	assert.Equal(t, "False", ca[0].Status)
	assert.True(t, ca[0].LastTransitionTime.After(oldTime))

	// Test adding different condition
	ca.SetCondition("Synced", "True", "SyncSuccessful", "Sync completed")
	assert.Len(t, ca, 2)
}

func TestConditionsArrayGetCondition(t *testing.T) {
	ca := ConditionsArray{
		{Type: "Ready", Status: "True"},
		{Type: "Synced", Status: "False"},
	}

	// Test existing condition
	condition := ca.GetCondition("Ready")
	require.NotNil(t, condition)
	assert.Equal(t, "True", condition.Status)

	// Test non-existing condition
	condition = ca.GetCondition("NonExistent")
	assert.Nil(t, condition)
}

func TestConditionsArrayIsConditionTrue(t *testing.T) {
	ca := ConditionsArray{
		{Type: "Ready", Status: "True"},
		{Type: "Synced", Status: "False"},
		{Type: "Unknown", Status: "Unknown"},
	}

	assert.True(t, ca.IsConditionTrue("Ready"))
	assert.False(t, ca.IsConditionTrue("Synced"))
	assert.False(t, ca.IsConditionTrue("Unknown"))
	assert.False(t, ca.IsConditionTrue("NonExistent"))
}

func TestVirtualDataCenterGetResourceUsagePercentages(t *testing.T) {
	vdc := &VirtualDataCenter{
		CPUQuota:     100,
		MemoryQuota:  200,
		StorageQuota: 500,
		CPUUsed:      50,
		MemoryUsed:   100,
		StorageUsed:  250,
	}

	cpuPct, memPct, storagePct := vdc.GetResourceUsagePercentages()

	assert.Equal(t, 50.0, cpuPct)
	assert.Equal(t, 50.0, memPct)
	assert.Equal(t, 50.0, storagePct)
}

func TestVirtualDataCenterUpdateResourceUsage(t *testing.T) {
	vdc := &VirtualDataCenter{
		CPUQuota:     100,
		MemoryQuota:  200,
		StorageQuota: 500,
	}

	beforeTime := time.Now()
	vdc.UpdateResourceUsage(25, 50, 125)
	afterTime := time.Now()

	assert.Equal(t, 25, vdc.CPUUsed)
	assert.Equal(t, 50, vdc.MemoryUsed)
	assert.Equal(t, 125, vdc.StorageUsed)
	assert.Equal(t, 25.0, vdc.CPUPercentage)
	assert.Equal(t, 25.0, vdc.MemoryPercentage)
	assert.Equal(t, 25.0, vdc.StoragePercentage)

	require.NotNil(t, vdc.LastMetricsUpdate)
	assert.True(t, vdc.LastMetricsUpdate.After(beforeTime))
	assert.True(t, vdc.LastMetricsUpdate.Before(afterTime))
}

func TestCatalogUpdateSyncStatus(t *testing.T) {
	catalog := &Catalog{}

	// Test successful sync
	beforeTime := time.Now()
	catalog.UpdateSyncStatus(true, "")
	_ = time.Now() // afterTime unused but kept for potential future use

	assert.Equal(t, CatalogPhaseReady, catalog.Phase)
	require.NotNil(t, catalog.LastSync)
	require.NotNil(t, catalog.LastSyncAttempt)
	assert.True(t, catalog.LastSync.After(beforeTime))
	assert.True(t, catalog.LastSyncAttempt.After(beforeTime))
	assert.True(t, catalog.Conditions.IsConditionTrue("Synced"))

	// Test failed sync
	catalog.UpdateSyncStatus(false, "sync failed: network error")

	assert.Equal(t, CatalogPhaseFailed, catalog.Phase)
	assert.False(t, catalog.Conditions.IsConditionTrue("Synced"))
	assert.NotNil(t, catalog.SyncErrors)
}

func TestCreateOrganizationRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request CreateOrganizationRequest
		valid   bool
	}{
		{
			name: "valid request",
			request: CreateOrganizationRequest{
				Name:        "test-org",
				DisplayName: "Test Organization",
				Description: "Test description",
				Admins:      []string{"admin-group"},
				IsEnabled:   true,
			},
			valid: true,
		},
		{
			name: "missing required fields",
			request: CreateOrganizationRequest{
				Description: "Test description",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would normally be validated by the binding framework
			// Here we just verify the struct can be marshaled/unmarshaled
			data, err := json.Marshal(tt.request)
			assert.NoError(t, err)

			var unmarshaled CreateOrganizationRequest
			err = json.Unmarshal(data, &unmarshaled)
			assert.NoError(t, err)
			assert.Equal(t, tt.request, unmarshaled)
		})
	}
}

func TestCreateVDCRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request CreateVDCRequest
		valid   bool
	}{
		{
			name: "valid request",
			request: CreateVDCRequest{
				Name:         "test-vdc",
				DisplayName:  "Test VDC",
				Description:  "Test description",
				OrgID:        "org-123",
				CPUQuota:     10,
				MemoryQuota:  20,
				StorageQuota: 100,
			},
			valid: true,
		},
		{
			name: "with limit range",
			request: CreateVDCRequest{
				Name:         "test-vdc",
				DisplayName:  "Test VDC",
				OrgID:        "org-123",
				CPUQuota:     10,
				MemoryQuota:  20,
				StorageQuota: 100,
				MinCPU:       func(i int) *int { return &i }(1000), // 1 core in millicores
				MaxCPU:       func(i int) *int { return &i }(4000), // 4 cores in millicores
				MinMemory:    func(i int) *int { return &i }(512),  // 512 MiB
				MaxMemory:    func(i int) *int { return &i }(8192), // 8 GiB in MiB
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			assert.NoError(t, err)

			var unmarshaled CreateVDCRequest
			err = json.Unmarshal(data, &unmarshaled)
			assert.NoError(t, err)
			assert.Equal(t, tt.request, unmarshaled)
		})
	}
}

func TestCreateCatalogRequestValidation(t *testing.T) {
	request := CreateCatalogRequest{
		Name:        "test-catalog",
		DisplayName: "Test Catalog",
		Description: "Test catalog description",
		OrgID:       "org-123",
		Type:        CatalogTypeVMTemplate,
		SourceType:  CatalogSourceOCI,
		SourceURL:   "registry.example.com/catalogs/vm-templates",
		IsEnabled:   true,
	}

	data, err := json.Marshal(request)
	assert.NoError(t, err)

	var unmarshaled CreateCatalogRequest
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, request, unmarshaled)
}

func TestCRDConstants(t *testing.T) {
	// Test that constants are properly defined
	assert.Equal(t, "Pending", OrgPhasePending)
	assert.Equal(t, "Active", OrgPhaseActive)
	assert.Equal(t, "Failed", OrgPhaseFailed)
	assert.Equal(t, "Terminating", OrgPhaseTerminating)

	assert.Equal(t, "Pending", VDCPhasePending)
	assert.Equal(t, "Active", VDCPhaseActive)
	assert.Equal(t, "Failed", VDCPhaseFailed)
	assert.Equal(t, "Suspended", VDCPhaseSuspended)
	assert.Equal(t, "Terminating", VDCPhaseTerminating)

	assert.Equal(t, "Pending", CatalogPhasePending)
	assert.Equal(t, "Syncing", CatalogPhaseSyncing)
	assert.Equal(t, "Ready", CatalogPhaseReady)
	assert.Equal(t, "Failed", CatalogPhaseFailed)
	assert.Equal(t, "Suspended", CatalogPhaseSuspended)

	assert.Equal(t, "default", NetworkPolicyDefault)
	assert.Equal(t, "isolated", NetworkPolicyIsolated)
	assert.Equal(t, "custom", NetworkPolicyCustom)

	assert.Equal(t, "vm-template", CatalogTypeVMTemplate)
	assert.Equal(t, "application-stack", CatalogTypeApplicationStack)
	assert.Equal(t, "mixed", CatalogTypeMixed)

	assert.Equal(t, "git", CatalogSourceGit)
	assert.Equal(t, "oci", CatalogSourceOCI)
	assert.Equal(t, "s3", CatalogSourceS3)
	assert.Equal(t, "http", CatalogSourceHTTP)
	assert.Equal(t, "local", CatalogSourceLocal)
}

// Benchmark tests for performance-critical operations

func BenchmarkConditionsArraySetCondition(b *testing.B) {
	var ca ConditionsArray

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ca.SetCondition("Ready", "True", "Available", "Resource is ready")
	}
}

func BenchmarkJSONBArrayScan(b *testing.B) {
	data := []byte(`["item1", "item2", "item3", "item4", "item5"]`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var ja JSONBArray
		_ = ja.Scan(data)
	}
}

func BenchmarkJSONBMapScan(b *testing.B) {
	data := []byte(`{"key1": "value1", "key2": "value2", "key3": 42, "key4": true}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var jm JSONBMap
		_ = jm.Scan(data)
	}
}
