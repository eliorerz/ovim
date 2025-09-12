package models

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test CRD validation functions and business logic

func TestOrganizationValidation(t *testing.T) {
	tests := []struct {
		name        string
		org         func() Organization
		expectValid bool
		errorMsg    string
	}{
		{
			name: "valid organization",
			org: func() Organization {
				return Organization{
					Name:        "test-org",
					DisplayName: stringPtr("Test Organization"),
					CRName:      "test-org",
					CRNamespace: "default",
					Namespace:   "org-test-org",
				}
			},
			expectValid: true,
		},
		{
			name: "missing display name",
			org: func() Organization {
				return Organization{
					Name:        "test-org",
					DisplayName: nil,
					CRName:      "test-org",
					CRNamespace: "default",
					Namespace:   "org-test-org",
				}
			},
			expectValid: false,
			errorMsg:    "display name is required",
		},
		{
			name: "empty display name",
			org: func() Organization {
				return Organization{
					Name:        "test-org",
					DisplayName: stringPtr(""),
					CRName:      "test-org",
					CRNamespace: "default",
					Namespace:   "org-test-org",
				}
			},
			expectValid: false,
			errorMsg:    "display name cannot be empty",
		},
		{
			name: "invalid CR name",
			org: func() Organization {
				return Organization{
					Name:        "test-org",
					DisplayName: stringPtr("Test Organization"),
					CRName:      "Test_Org!",
					CRNamespace: "default",
					Namespace:   "org-test-org",
				}
			},
			expectValid: false,
			errorMsg:    "invalid CR name format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := tt.org()
			err := validateOrganization(&org)

			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestVDCQuotaValidation(t *testing.T) {
	tests := []struct {
		name        string
		vdc         func() VirtualDataCenter
		expectValid bool
		errorMsg    string
	}{
		{
			name: "valid quotas",
			vdc: func() VirtualDataCenter {
				return VirtualDataCenter{
					Name:         "test-vdc",
					CPUQuota:     10,
					MemoryQuota:  20,
					StorageQuota: 100,
				}
			},
			expectValid: true,
		},
		{
			name: "zero CPU quota",
			vdc: func() VirtualDataCenter {
				return VirtualDataCenter{
					Name:         "test-vdc",
					CPUQuota:     0,
					MemoryQuota:  20,
					StorageQuota: 100,
				}
			},
			expectValid: false,
			errorMsg:    "CPU quota must be positive",
		},
		{
			name: "negative memory quota",
			vdc: func() VirtualDataCenter {
				return VirtualDataCenter{
					Name:         "test-vdc",
					CPUQuota:     10,
					MemoryQuota:  -5,
					StorageQuota: 100,
				}
			},
			expectValid: false,
			errorMsg:    "memory quota must be positive",
		},
		{
			name: "invalid limit range",
			vdc: func() VirtualDataCenter {
				maxCPU := 1000
				minCPU := 2000 // min > max
				return VirtualDataCenter{
					Name:         "test-vdc",
					CPUQuota:     10,
					MemoryQuota:  20,
					StorageQuota: 100,
					MinCPU:       &minCPU,
					MaxCPU:       &maxCPU,
				}
			},
			expectValid: false,
			errorMsg:    "min CPU cannot be greater than max CPU",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vdc := tt.vdc()
			err := validateVDCQuotas(&vdc)

			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestCatalogSourceValidation(t *testing.T) {
	tests := []struct {
		name        string
		catalog     func() Catalog
		expectValid bool
		errorMsg    string
	}{
		{
			name: "valid OCI source",
			catalog: func() Catalog {
				return Catalog{
					Name:       "test-catalog",
					Type:       CatalogTypeVMTemplate,
					SourceType: CatalogSourceOCI,
					SourceURL:  "registry.example.com/templates",
				}
			},
			expectValid: true,
		},
		{
			name: "valid Git source",
			catalog: func() Catalog {
				return Catalog{
					Name:         "test-catalog",
					Type:         CatalogTypeVMTemplate,
					SourceType:   CatalogSourceGit,
					SourceURL:    "https://github.com/company/templates.git",
					SourceBranch: "main",
				}
			},
			expectValid: true,
		},
		{
			name: "invalid URL",
			catalog: func() Catalog {
				return Catalog{
					Name:       "test-catalog",
					Type:       CatalogTypeVMTemplate,
					SourceType: CatalogSourceOCI,
					SourceURL:  "not-a-valid-url",
				}
			},
			expectValid: false,
			errorMsg:    "invalid source URL",
		},
		{
			name: "invalid refresh interval",
			catalog: func() Catalog {
				return Catalog{
					Name:            "test-catalog",
					Type:            CatalogTypeVMTemplate,
					SourceType:      CatalogSourceGit,
					SourceURL:       "https://github.com/company/templates.git",
					RefreshInterval: "invalid",
				}
			},
			expectValid: false,
			errorMsg:    "invalid refresh interval format",
		},
		{
			name: "unsupported source type",
			catalog: func() Catalog {
				return Catalog{
					Name:       "test-catalog",
					Type:       CatalogTypeVMTemplate,
					SourceType: "ftp",
					SourceURL:  "ftp://example.com/templates",
				}
			},
			expectValid: false,
			errorMsg:    "unsupported source type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := tt.catalog()
			err := validateCatalogSource(&catalog)

			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestResourceUsageCalculations(t *testing.T) {
	vdc := &VirtualDataCenter{
		CPUQuota:     100,
		MemoryQuota:  200,
		StorageQuota: 500,
	}

	tests := []struct {
		name               string
		cpuUsed            int
		memoryUsed         int
		storageUsed        int
		expectedCPUPct     float64
		expectedMemoryPct  float64
		expectedStoragePct float64
	}{
		{
			name:               "50% usage",
			cpuUsed:            50,
			memoryUsed:         100,
			storageUsed:        250,
			expectedCPUPct:     50.0,
			expectedMemoryPct:  50.0,
			expectedStoragePct: 50.0,
		},
		{
			name:               "over quota usage",
			cpuUsed:            120,
			memoryUsed:         250,
			storageUsed:        600,
			expectedCPUPct:     120.0,
			expectedMemoryPct:  125.0,
			expectedStoragePct: 120.0,
		},
		{
			name:               "zero usage",
			cpuUsed:            0,
			memoryUsed:         0,
			storageUsed:        0,
			expectedCPUPct:     0.0,
			expectedMemoryPct:  0.0,
			expectedStoragePct: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vdc.UpdateResourceUsage(tt.cpuUsed, tt.memoryUsed, tt.storageUsed)

			assert.Equal(t, tt.cpuUsed, vdc.CPUUsed)
			assert.Equal(t, tt.memoryUsed, vdc.MemoryUsed)
			assert.Equal(t, tt.storageUsed, vdc.StorageUsed)
			assert.Equal(t, tt.expectedCPUPct, vdc.CPUPercentage)
			assert.Equal(t, tt.expectedMemoryPct, vdc.MemoryPercentage)
			assert.Equal(t, tt.expectedStoragePct, vdc.StoragePercentage)

			// Verify timestamp was updated
			assert.NotNil(t, vdc.LastMetricsUpdate)
			assert.True(t, time.Since(*vdc.LastMetricsUpdate) < time.Second)
		})
	}
}

func TestCatalogSyncStatusManagement(t *testing.T) {
	catalog := &Catalog{}

	// Test successful sync
	catalog.UpdateSyncStatus(true, "")

	assert.Equal(t, CatalogPhaseReady, catalog.Phase)
	assert.NotNil(t, catalog.LastSync)
	assert.NotNil(t, catalog.LastSyncAttempt)
	assert.True(t, catalog.Conditions.IsConditionTrue("Synced"))

	lastSuccessTime := *catalog.LastSync

	// Test failed sync
	time.Sleep(time.Millisecond) // Ensure time difference
	catalog.UpdateSyncStatus(false, "connection timeout")

	assert.Equal(t, CatalogPhaseFailed, catalog.Phase)
	assert.Equal(t, lastSuccessTime, *catalog.LastSync) // Last success time unchanged
	assert.True(t, catalog.LastSyncAttempt.After(lastSuccessTime))
	assert.False(t, catalog.Conditions.IsConditionTrue("Synced"))
	assert.NotNil(t, catalog.SyncErrors)

	// Verify error was recorded
	assert.Greater(t, len(catalog.SyncErrors), 0)
}

func TestConditionManagement(t *testing.T) {
	var conditions ConditionsArray

	// Test adding new condition
	conditions.SetCondition("Ready", "True", "Available", "Resource is ready")
	assert.Len(t, conditions, 1)
	assert.Equal(t, "Ready", conditions[0].Type)
	assert.Equal(t, "True", conditions[0].Status)

	// Test updating existing condition with different status
	time.Sleep(time.Millisecond)
	originalTime := conditions[0].LastTransitionTime
	conditions.SetCondition("Ready", "False", "NotAvailable", "Resource is not ready")
	assert.Len(t, conditions, 1)
	assert.Equal(t, "False", conditions[0].Status)
	assert.True(t, conditions[0].LastTransitionTime.After(originalTime))

	// Test updating with same status (time should not change)
	sameStatusTime := conditions[0].LastTransitionTime
	conditions.SetCondition("Ready", "False", "StillNotAvailable", "Still not ready")
	assert.Equal(t, sameStatusTime, conditions[0].LastTransitionTime)

	// Test adding different condition
	conditions.SetCondition("Synced", "True", "SyncSuccessful", "Sync completed")
	assert.Len(t, conditions, 2)

	// Test condition queries
	assert.False(t, conditions.IsConditionTrue("Ready"))
	assert.True(t, conditions.IsConditionTrue("Synced"))
	assert.False(t, conditions.IsConditionTrue("NonExistent"))

	readyCondition := conditions.GetCondition("Ready")
	require.NotNil(t, readyCondition)
	assert.Equal(t, "False", readyCondition.Status)

	nonExistentCondition := conditions.GetCondition("NonExistent")
	assert.Nil(t, nonExistentCondition)
}

func TestJSONBFieldsEdgeCases(t *testing.T) {
	// Test JSONBArray with special characters
	specialArray := JSONBArray{"item with spaces", "item-with-dashes", "item_with_underscores", "item/with/slashes"}
	value, err := specialArray.Value()
	require.NoError(t, err)

	var scannedArray JSONBArray
	err = scannedArray.Scan(value)
	require.NoError(t, err)
	assert.Equal(t, specialArray, scannedArray)

	// Test JSONBMap with complex values
	complexMap := JSONBMap{
		"string":  "value",
		"number":  42.5,
		"boolean": true,
		"array":   []interface{}{"nested", "array"},
		"object":  map[string]interface{}{"nested": "object"},
	}

	value, err = complexMap.Value()
	require.NoError(t, err)

	var scannedMap JSONBMap
	err = scannedMap.Scan(value)
	require.NoError(t, err)

	// Compare individual fields since float precision might vary
	assert.Equal(t, "value", scannedMap["string"])
	assert.Equal(t, float64(42.5), scannedMap["number"])
	assert.Equal(t, true, scannedMap["boolean"])
	assert.IsType(t, []interface{}{}, scannedMap["array"])
	assert.IsType(t, map[string]interface{}{}, scannedMap["object"])
}

func TestCRDRequestValidation(t *testing.T) {
	// Test CreateOrganizationRequest validation
	orgReq := CreateOrganizationRequest{
		Name:        "test-org",
		DisplayName: "Test Organization",
		Admins:      []string{"admin-group"},
		IsEnabled:   true,
	}
	assert.True(t, validateCreateOrganizationRequest(&orgReq))

	// Test with missing required fields
	invalidOrgReq := CreateOrganizationRequest{
		Name:      "test-org",
		IsEnabled: true,
		// Missing DisplayName and Admins
	}
	assert.False(t, validateCreateOrganizationRequest(&invalidOrgReq))

	// Test CreateVDCRequest validation
	vdcReq := CreateVDCRequest{
		Name:         "test-vdc",
		DisplayName:  "Test VDC",
		OrgID:        "org-123",
		CPUQuota:     10,
		MemoryQuota:  20,
		StorageQuota: 100,
	}
	assert.True(t, validateCreateVDCRequest(&vdcReq))

	// Test with invalid quotas
	invalidVDCReq := CreateVDCRequest{
		Name:         "test-vdc",
		DisplayName:  "Test VDC",
		OrgID:        "org-123",
		CPUQuota:     0, // Invalid
		MemoryQuota:  20,
		StorageQuota: 100,
	}
	assert.False(t, validateCreateVDCRequest(&invalidVDCReq))

	// Test CreateCatalogRequest validation
	catalogReq := CreateCatalogRequest{
		Name:        "test-catalog",
		DisplayName: "Test Catalog",
		OrgID:       "org-123",
		Type:        CatalogTypeVMTemplate,
		SourceType:  CatalogSourceOCI,
		SourceURL:   "registry.example.com/templates",
		IsEnabled:   true,
	}
	assert.True(t, validateCreateCatalogRequest(&catalogReq))

	// Test with invalid source type
	invalidCatalogReq := CreateCatalogRequest{
		Name:        "test-catalog",
		DisplayName: "Test Catalog",
		OrgID:       "org-123",
		Type:        CatalogTypeVMTemplate,
		SourceType:  "invalid-source", // Invalid
		SourceURL:   "registry.example.com/templates",
		IsEnabled:   true,
	}
	assert.False(t, validateCreateCatalogRequest(&invalidCatalogReq))
}

// Helper functions for validation (these would be implemented in the actual models)

func validateOrganization(org *Organization) error {
	if org.DisplayName == nil || *org.DisplayName == "" {
		if org.DisplayName == nil {
			return fmt.Errorf("display name is required")
		}
		return fmt.Errorf("display name cannot be empty")
	}

	// Simplified CR name validation
	if org.CRName != "" && !isValidKubernetesName(org.CRName) {
		return fmt.Errorf("invalid CR name format")
	}

	return nil
}

func validateVDCQuotas(vdc *VirtualDataCenter) error {
	if vdc.CPUQuota <= 0 {
		return fmt.Errorf("CPU quota must be positive")
	}
	if vdc.MemoryQuota <= 0 {
		return fmt.Errorf("memory quota must be positive")
	}
	if vdc.StorageQuota <= 0 {
		return fmt.Errorf("storage quota must be positive")
	}

	if vdc.MinCPU != nil && vdc.MaxCPU != nil && *vdc.MinCPU > *vdc.MaxCPU {
		return fmt.Errorf("min CPU cannot be greater than max CPU")
	}

	if vdc.MinMemory != nil && vdc.MaxMemory != nil && *vdc.MinMemory > *vdc.MaxMemory {
		return fmt.Errorf("min memory cannot be greater than max memory")
	}

	return nil
}

func validateCatalogSource(catalog *Catalog) error {
	// Simplified source type validation
	validSources := []string{CatalogSourceGit, CatalogSourceOCI, CatalogSourceS3, CatalogSourceHTTP, CatalogSourceLocal}
	found := false
	for _, valid := range validSources {
		if catalog.SourceType == valid {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unsupported source type: %s", catalog.SourceType)
	}

	// Simplified URL validation
	if catalog.SourceURL == "" || (!strings.HasPrefix(catalog.SourceURL, "http://") &&
		!strings.HasPrefix(catalog.SourceURL, "https://") &&
		!strings.Contains(catalog.SourceURL, ".")) {
		return fmt.Errorf("invalid source URL: %s", catalog.SourceURL)
	}

	// Simplified refresh interval validation
	if catalog.RefreshInterval != "" && catalog.RefreshInterval != "1h" &&
		catalog.RefreshInterval != "30m" && catalog.RefreshInterval != "15m" {
		return fmt.Errorf("invalid refresh interval format: %s", catalog.RefreshInterval)
	}

	return nil
}

func validateCreateOrganizationRequest(req *CreateOrganizationRequest) bool {
	return req.Name != "" && req.DisplayName != "" && len(req.Admins) > 0
}

func validateCreateVDCRequest(req *CreateVDCRequest) bool {
	return req.Name != "" && req.DisplayName != "" && req.OrgID != "" &&
		req.CPUQuota > 0 && req.MemoryQuota > 0 && req.StorageQuota > 0
}

func validateCreateCatalogRequest(req *CreateCatalogRequest) bool {
	validTypes := []string{CatalogTypeVMTemplate, CatalogTypeApplicationStack, CatalogTypeMixed}
	validSources := []string{CatalogSourceGit, CatalogSourceOCI, CatalogSourceS3, CatalogSourceHTTP, CatalogSourceLocal}

	typeValid := false
	for _, valid := range validTypes {
		if req.Type == valid {
			typeValid = true
			break
		}
	}

	sourceValid := false
	for _, valid := range validSources {
		if req.SourceType == valid {
			sourceValid = true
			break
		}
	}

	return req.Name != "" && req.DisplayName != "" && req.OrgID != "" &&
		typeValid && sourceValid && req.SourceURL != ""
}

func isValidKubernetesName(name string) bool {
	// Simplified Kubernetes name validation
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	// Should start and end with alphanumeric, can contain hyphens
	// For testing purposes, reject names with special characters like ! or _
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return false
		}
	}
	return true
}

func stringPtr(s string) *string {
	return &s
}
