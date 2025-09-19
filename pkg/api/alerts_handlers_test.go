package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertsHandlers_GetAlertSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		validateResp   func(*testing.T, *AlertSummary)
	}{
		{
			name:           "successful get alert summary",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, summary *AlertSummary) {
				// Since this is a mock implementation, verify the structure
				assert.NotNil(t, summary)
				assert.Equal(t, 0, summary.TotalAlerts)
				assert.Equal(t, 0, summary.UnacknowledgedAlerts)
				assert.Equal(t, 0, summary.CriticalAlerts)
				assert.Equal(t, 0, summary.WarningAlerts)
				assert.Equal(t, 0, summary.InfoAlerts)
				assert.NotNil(t, summary.RecentAlerts)
				assert.Len(t, summary.RecentAlerts, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := NewAlertsHandlers()

			req := httptest.NewRequest(http.MethodGet, "/alerts/summary", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			handlers.GetAlertSummary(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateResp != nil {
				var summary AlertSummary
				err := json.Unmarshal(w.Body.Bytes(), &summary)
				require.NoError(t, err)
				tt.validateResp(t, &summary)
			}
		})
	}
}

func TestNewAlertsHandlers(t *testing.T) {
	handlers := NewAlertsHandlers()

	assert.NotNil(t, handlers)
}

func TestAlertSummary_Structure(t *testing.T) {
	summary := AlertSummary{
		TotalAlerts:          5,
		UnacknowledgedAlerts: 3,
		CriticalAlerts:       1,
		WarningAlerts:        2,
		InfoAlerts:           2,
		RecentAlerts: []AlertNotification{
			{
				ID:                  "alert1",
				AlertThresholdID:    "threshold1",
				ThresholdName:       "CPU Usage High",
				ResourceType:        "cpu",
				CurrentPercentage:   85.5,
				ThresholdPercentage: 80.0,
				Severity:            "warning",
				Scope:               "vm",
				ScopeID:             &[]string{"vm-123"}[0],
				ScopeName:           "web-server-01",
				Acknowledged:        false,
				Resolved:            false,
			},
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(summary)
	require.NoError(t, err)

	var unmarshaled AlertSummary
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, summary.TotalAlerts, unmarshaled.TotalAlerts)
	assert.Equal(t, summary.UnacknowledgedAlerts, unmarshaled.UnacknowledgedAlerts)
	assert.Equal(t, summary.CriticalAlerts, unmarshaled.CriticalAlerts)
	assert.Equal(t, summary.WarningAlerts, unmarshaled.WarningAlerts)
	assert.Equal(t, summary.InfoAlerts, unmarshaled.InfoAlerts)
	assert.Len(t, unmarshaled.RecentAlerts, 1)
	assert.Equal(t, "alert1", unmarshaled.RecentAlerts[0].ID)
	assert.Equal(t, "CPU Usage High", unmarshaled.RecentAlerts[0].ThresholdName)
	assert.Equal(t, 85.5, unmarshaled.RecentAlerts[0].CurrentPercentage)
}

func TestAlertNotification_Structure(t *testing.T) {
	notification := AlertNotification{
		ID:                  "alert1",
		AlertThresholdID:    "threshold1",
		ThresholdName:       "Memory Usage Critical",
		ResourceType:        "memory",
		CurrentPercentage:   95.0,
		ThresholdPercentage: 90.0,
		Severity:            "critical",
		Scope:               "organization",
		ScopeID:             &[]string{"org-456"}[0],
		ScopeName:           "ACME Corp",
		Acknowledged:        true,
		AcknowledgedBy:      &[]string{"admin@example.com"}[0],
		Resolved:            false,
	}

	// Test JSON serialization
	data, err := json.Marshal(notification)
	require.NoError(t, err)

	var unmarshaled AlertNotification
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, notification.ID, unmarshaled.ID)
	assert.Equal(t, notification.AlertThresholdID, unmarshaled.AlertThresholdID)
	assert.Equal(t, notification.ThresholdName, unmarshaled.ThresholdName)
	assert.Equal(t, notification.ResourceType, unmarshaled.ResourceType)
	assert.Equal(t, notification.CurrentPercentage, unmarshaled.CurrentPercentage)
	assert.Equal(t, notification.ThresholdPercentage, unmarshaled.ThresholdPercentage)
	assert.Equal(t, notification.Severity, unmarshaled.Severity)
	assert.Equal(t, notification.Scope, unmarshaled.Scope)
	assert.NotNil(t, unmarshaled.ScopeID)
	assert.Equal(t, *notification.ScopeID, *unmarshaled.ScopeID)
	assert.Equal(t, notification.ScopeName, unmarshaled.ScopeName)
	assert.Equal(t, notification.Acknowledged, unmarshaled.Acknowledged)
	assert.NotNil(t, unmarshaled.AcknowledgedBy)
	assert.Equal(t, *notification.AcknowledgedBy, *unmarshaled.AcknowledgedBy)
	assert.Equal(t, notification.Resolved, unmarshaled.Resolved)
}

func TestAlertSummary_EmptyRecentAlerts(t *testing.T) {
	summary := AlertSummary{
		TotalAlerts:          0,
		UnacknowledgedAlerts: 0,
		CriticalAlerts:       0,
		WarningAlerts:        0,
		InfoAlerts:           0,
		RecentAlerts:         []AlertNotification{},
	}

	// Test JSON serialization with empty slice
	data, err := json.Marshal(summary)
	require.NoError(t, err)

	var unmarshaled AlertSummary
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.NotNil(t, unmarshaled.RecentAlerts)
	assert.Len(t, unmarshaled.RecentAlerts, 0)
}

func TestAlertNotification_OptionalFields(t *testing.T) {
	// Test with minimal required fields
	notification := AlertNotification{
		ID:                  "alert2",
		AlertThresholdID:    "threshold2",
		ThresholdName:       "Storage Usage",
		ResourceType:        "storage",
		CurrentPercentage:   75.0,
		ThresholdPercentage: 80.0,
		Severity:            "info",
		Scope:               "vdc",
		ScopeName:           "dev-environment",
		Acknowledged:        false,
		Resolved:            false,
		// Optional fields not set: ScopeID, AcknowledgedBy, AcknowledgedAt, ResolvedAt
	}

	// Test JSON serialization
	data, err := json.Marshal(notification)
	require.NoError(t, err)

	var unmarshaled AlertNotification
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, notification.ID, unmarshaled.ID)
	assert.Equal(t, notification.Severity, unmarshaled.Severity)
	assert.Equal(t, notification.Scope, unmarshaled.Scope)
	assert.Nil(t, unmarshaled.ScopeID)
	assert.Nil(t, unmarshaled.AcknowledgedBy)
	assert.Nil(t, unmarshaled.AcknowledgedAt)
	assert.Nil(t, unmarshaled.ResolvedAt)
}
