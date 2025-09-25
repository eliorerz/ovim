package api

import (
	"fmt"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/config"
	"k8s.io/klog/v2"
)

// SpokeURLBuilder builds URLs for spoke agent communication using FQDNs
// Deprecated: Use SpokeClient instead for enhanced functionality
type SpokeURLBuilder struct {
	spokeClient *SpokeClient
}

// NewSpokeURLBuilder creates a new spoke URL builder with configuration
// Deprecated: Use NewSpokeClient instead
func NewSpokeURLBuilder(cfg *config.Config) *SpokeURLBuilder {
	spokeClient := NewSpokeClient(&cfg.Spoke, nil)
	return &SpokeURLBuilder{
		spokeClient: spokeClient,
	}
}

// BuildSpokeAgentURL builds the FQDN URL for a spoke agent given a cluster ID
func (b *SpokeURLBuilder) BuildSpokeAgentURL(clusterID string) string {
	fqdn, err := b.spokeClient.generateFQDN(clusterID)
	if err != nil {
		klog.Errorf("Failed to generate FQDN for cluster %s: %v", clusterID, err)
		return ""
	}
	return b.spokeClient.buildSpokeURL(fqdn, "")
}

// BuildSpokeHealthURL builds the health check URL for a spoke agent
func (b *SpokeURLBuilder) BuildSpokeHealthURL(clusterID string) string {
	fqdn, err := b.spokeClient.generateFQDN(clusterID)
	if err != nil {
		klog.Errorf("Failed to generate FQDN for cluster %s: %v", clusterID, err)
		return ""
	}
	return b.spokeClient.buildSpokeURL(fqdn, "/health")
}

// BuildSpokeOperationsURL builds the operations endpoint URL for a spoke agent
func (b *SpokeURLBuilder) BuildSpokeOperationsURL(clusterID string) string {
	fqdn, err := b.spokeClient.generateFQDN(clusterID)
	if err != nil {
		klog.Errorf("Failed to generate FQDN for cluster %s: %v", clusterID, err)
		return ""
	}
	return b.spokeClient.buildSpokeURL(fqdn, "/operations")
}

// CheckSpokeHealth performs a health check against a spoke agent
func (b *SpokeURLBuilder) CheckSpokeHealth(clusterID string) (*SpokeHealthResponse, error) {
	return b.spokeClient.CheckSpokeHealth(clusterID)
}

// LegacySpokeHealthResponse represents the response from a spoke health check (legacy format)
type LegacySpokeHealthResponse struct {
	Status    string    `json:"status"`
	ClusterID string    `json:"cluster_id"`
	URL       string    `json:"url"`
	Timestamp time.Time `json:"timestamp"`
}

// ConvertToLegacyFormat converts the new SpokeHealthResponse to the legacy format
func ConvertToLegacyFormat(response *SpokeHealthResponse) *LegacySpokeHealthResponse {
	return &LegacySpokeHealthResponse{
		Status:    response.Status,
		ClusterID: response.ClusterID,
		URL:       fmt.Sprintf("https://%s/health", response.FQDN),
		Timestamp: response.Timestamp,
	}
}