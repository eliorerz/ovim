package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AlertsHandlers handles alerts-related API endpoints
type AlertsHandlers struct {
	// storage storage.Storage // TODO: Add when actual alerting system is implemented
}

// NewAlertsHandlers creates a new alerts handlers instance
func NewAlertsHandlers() *AlertsHandlers {
	return &AlertsHandlers{}
}

// AlertSummary represents the alerts summary response
type AlertSummary struct {
	TotalAlerts          int                 `json:"total_alerts"`
	UnacknowledgedAlerts int                 `json:"unacknowledged_alerts"`
	CriticalAlerts       int                 `json:"critical_alerts"`
	WarningAlerts        int                 `json:"warning_alerts"`
	InfoAlerts           int                 `json:"info_alerts"`
	RecentAlerts         []AlertNotification `json:"recent_alerts"`
}

// AlertNotification represents an individual alert notification
type AlertNotification struct {
	ID                  string     `json:"id"`
	AlertThresholdID    string     `json:"alert_threshold_id"`
	ThresholdName       string     `json:"threshold_name"`
	ResourceType        string     `json:"resource_type"`
	CurrentPercentage   float64    `json:"current_percentage"`
	ThresholdPercentage float64    `json:"threshold_percentage"`
	Severity            string     `json:"severity"`
	Scope               string     `json:"scope"`
	ScopeID             *string    `json:"scope_id,omitempty"`
	ScopeName           string     `json:"scope_name"`
	TriggeredAt         time.Time  `json:"triggered_at"`
	Acknowledged        bool       `json:"acknowledged"`
	AcknowledgedBy      *string    `json:"acknowledged_by,omitempty"`
	AcknowledgedAt      *time.Time `json:"acknowledged_at,omitempty"`
	Resolved            bool       `json:"resolved"`
	ResolvedAt          *time.Time `json:"resolved_at,omitempty"`
}

// GetAlertSummary handles GET /alerts/summary
func (h *AlertsHandlers) GetAlertSummary(c *gin.Context) {
	// TODO: Implement actual alerting system
	// For now, return empty/mock data since no alerting system is implemented yet
	summary := &AlertSummary{
		TotalAlerts:          0,
		UnacknowledgedAlerts: 0,
		CriticalAlerts:       0,
		WarningAlerts:        0,
		InfoAlerts:           0,
		RecentAlerts:         []AlertNotification{},
	}

	c.JSON(http.StatusOK, summary)
}
