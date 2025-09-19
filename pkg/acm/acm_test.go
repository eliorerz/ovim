package acm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Simple integration tests for the ACM package

func TestDefaultSyncConfig(t *testing.T) {
	config := DefaultSyncConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, 10*time.Minute, config.Interval)
	assert.Equal(t, "open-cluster-management", config.Namespace)
	assert.True(t, config.AutoCreateZones)
	assert.Equal(t, "", config.ZonePrefix)
	assert.Equal(t, 80, config.DefaultQuotaPercentage)
	assert.Empty(t, config.ExcludedClusters)
	assert.Empty(t, config.RequiredLabels)
}

func TestEnabledSyncConfig(t *testing.T) {
	kubeconfig := "/path/to/kubeconfig"
	config := EnabledSyncConfig(kubeconfig)

	assert.True(t, config.Enabled)
	assert.Equal(t, kubeconfig, config.HubKubeconfig)
	assert.Equal(t, 10*time.Minute, config.Interval)
	assert.Equal(t, "open-cluster-management", config.Namespace)
	assert.True(t, config.AutoCreateZones)
	assert.Equal(t, 80, config.DefaultQuotaPercentage)
}

func TestValidateSyncConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  SyncConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid enabled config",
			config: SyncConfig{
				Enabled:                true,
				Interval:               5 * time.Minute,
				DefaultQuotaPercentage: 80,
			},
			wantErr: false,
		},
		{
			name: "Valid disabled config",
			config: SyncConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "Zero interval",
			config: SyncConfig{
				Enabled:  true,
				Interval: 0,
			},
			wantErr: true,
			errMsg:  "sync interval must be positive",
		},
		{
			name: "Interval too short",
			config: SyncConfig{
				Enabled:  true,
				Interval: 30 * time.Second,
			},
			wantErr: true,
			errMsg:  "sync interval must be at least 1 minute",
		},
		{
			name: "Quota percentage too high",
			config: SyncConfig{
				Enabled:                true,
				Interval:               5 * time.Minute,
				DefaultQuotaPercentage: 150,
			},
			wantErr: true,
			errMsg:  "quota percentage must be between 0 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSyncConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSyncResultStructure(t *testing.T) {
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
