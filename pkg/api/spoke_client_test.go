package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpokeClient_NewSpokeClient(t *testing.T) {
	cfg := &config.SpokeConfig{
		Protocol: "https",
		Timeout:  30 * time.Second,
		TLS: config.TLSConfig{
			Enabled:    true,
			SkipVerify: false,
		},
		Retry: config.RetryConfig{
			Enabled:    true,
			MaxRetries: 3,
		},
	}

	client := NewSpokeClient(cfg, nil)

	assert.NotNil(t, client)
	assert.Equal(t, cfg, client.config)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.spokes)
	assert.NotNil(t, client.healthStatus)
}

func TestSpokeClient_GenerateFQDN(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.SpokeConfig
		clusterID   string
		expected    string
		shouldError bool
	}{
		{
			name: "basic template generation",
			config: &config.SpokeConfig{
				HostPattern:  "ovim-spoke-agent",
				DomainSuffix: "example.com",
				FQDNTemplate: "{{.HostPattern}}-{{.ClusterID}}.{{.DomainSuffix}}",
				CustomFQDNs:  make(map[string]string),
			},
			clusterID: "cluster1",
			expected:  "ovim-spoke-agent-cluster1.example.com",
		},
		{
			name: "custom FQDN override",
			config: &config.SpokeConfig{
				HostPattern:  "ovim-spoke-agent",
				DomainSuffix: "example.com",
				FQDNTemplate: "{{.HostPattern}}-{{.ClusterID}}.{{.DomainSuffix}}",
				CustomFQDNs: map[string]string{
					"cluster1": "custom-spoke.custom.com",
				},
			},
			clusterID: "cluster1",
			expected:  "custom-spoke.custom.com",
		},
		{
			name: "missing domain suffix",
			config: &config.SpokeConfig{
				HostPattern:  "ovim-spoke-agent",
				DomainSuffix: "",
				FQDNTemplate: "{{.HostPattern}}-{{.ClusterID}}.{{.DomainSuffix}}",
				CustomFQDNs:  make(map[string]string),
			},
			clusterID:   "cluster1",
			shouldError: true,
		},
		{
			name: "unresolved template variables",
			config: &config.SpokeConfig{
				HostPattern:  "ovim-spoke-agent",
				DomainSuffix: "example.com",
				FQDNTemplate: "{{.HostPattern}}-{{.ClusterID}}.{{.UnknownVar}}",
				CustomFQDNs:  make(map[string]string),
			},
			clusterID:   "cluster1",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSpokeClient(tt.config, nil)
			result, err := client.generateFQDN(tt.clusterID)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSpokeClient_GetSpokeListFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.SpokeConfig
		envValue    string
		expected    []SpokeInfo
		shouldError bool
	}{
		{
			name: "valid spoke list with FQDNs",
			config: &config.SpokeConfig{
				Discovery: config.DiscoveryConfig{
					SpokeListEnv: "TEST_SPOKE_LIST",
				},
			},
			envValue: "cluster1:zone1:spoke1.example.com,cluster2:zone2:spoke2.example.com",
			expected: []SpokeInfo{
				{
					ClusterID: "cluster1",
					ZoneID:    "zone1",
					FQDN:      "spoke1.example.com",
					Status:    "unknown",
					Enabled:   true,
				},
				{
					ClusterID: "cluster2",
					ZoneID:    "zone2",
					FQDN:      "spoke2.example.com",
					Status:    "unknown",
					Enabled:   true,
				},
			},
		},
		{
			name: "spoke list without FQDNs (auto-generated)",
			config: &config.SpokeConfig{
				HostPattern:  "ovim-spoke-agent",
				DomainSuffix: "example.com",
				FQDNTemplate: "{{.HostPattern}}-{{.ClusterID}}.{{.DomainSuffix}}",
				CustomFQDNs:  make(map[string]string),
				Discovery: config.DiscoveryConfig{
					SpokeListEnv: "TEST_SPOKE_LIST",
				},
			},
			envValue: "cluster1:zone1,cluster2:zone2",
			expected: []SpokeInfo{
				{
					ClusterID: "cluster1",
					ZoneID:    "zone1",
					FQDN:      "ovim-spoke-agent-cluster1.example.com",
					Status:    "unknown",
					Enabled:   true,
				},
				{
					ClusterID: "cluster2",
					ZoneID:    "zone2",
					FQDN:      "ovim-spoke-agent-cluster2.example.com",
					Status:    "unknown",
					Enabled:   true,
				},
			},
		},
		{
			name: "empty environment variable",
			config: &config.SpokeConfig{
				Discovery: config.DiscoveryConfig{
					SpokeListEnv: "TEST_SPOKE_LIST",
				},
			},
			envValue: "",
			expected: []SpokeInfo{},
		},
		{
			name: "invalid format",
			config: &config.SpokeConfig{
				Discovery: config.DiscoveryConfig{
					SpokeListEnv: "TEST_SPOKE_LIST",
				},
			},
			envValue:    "invalid_format",
			shouldError: true,
		},
		{
			name: "empty cluster ID",
			config: &config.SpokeConfig{
				Discovery: config.DiscoveryConfig{
					SpokeListEnv: "TEST_SPOKE_LIST",
				},
			},
			envValue:    ":zone1:fqdn1",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			tt.config.Discovery.SpokeListEnv = tt.envValue

			client := NewSpokeClient(tt.config, nil)
			result, err := client.getSpokeListFromEnv()

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSpokeClient_HealthCheck(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		case "/unhealthy":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Extract host from server URL for FQDN
	serverURL := strings.TrimPrefix(server.URL, "https://")

	cfg := &config.SpokeConfig{
		Protocol: "https",
		Timeout:  10 * time.Second,
		TLS: config.TLSConfig{
			Enabled:    true,
			SkipVerify: true, // Skip verification for test server
		},
		HealthCheck: config.HealthCheckConfig{
			Enabled: true,
			Timeout: 5 * time.Second,
			Path:    "/health",
		},
		Retry: config.RetryConfig{
			Enabled: false, // Disable retry for faster tests
		},
	}

	client := NewSpokeClient(cfg, nil)

	t.Run("successful health check", func(t *testing.T) {
		spoke := &SpokeInfo{
			ClusterID: "test-cluster",
			ZoneID:    "test-zone",
			FQDN:      serverURL,
			Enabled:   true,
		}

		response, err := client.performHealthCheck(spoke)
		require.NoError(t, err)
		assert.Equal(t, "healthy", response.Status)
		assert.Equal(t, "test-cluster", response.ClusterID)
		assert.Equal(t, serverURL, response.FQDN)
	})

	t.Run("unhealthy spoke", func(t *testing.T) {
		spoke := &SpokeInfo{
			ClusterID: "test-cluster",
			ZoneID:    "test-zone",
			FQDN:      serverURL,
			Enabled:   true,
		}

		// Override path to trigger unhealthy response
		originalPath := client.config.HealthCheck.Path
		client.config.HealthCheck.Path = "/unhealthy"
		defer func() { client.config.HealthCheck.Path = originalPath }()

		_, err := client.performHealthCheck(spoke)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check failed with status 503")
	})

	t.Run("unreachable spoke", func(t *testing.T) {
		spoke := &SpokeInfo{
			ClusterID: "test-cluster",
			ZoneID:    "test-zone",
			FQDN:      "nonexistent.example.com",
			Enabled:   true,
		}

		_, err := client.performHealthCheck(spoke)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check request failed")
	})
}

func TestSpokeClient_RetryLogic(t *testing.T) {
	retryCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		retryCount++
		if retryCount < 3 {
			// Fail first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Succeed on third attempt
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer server.Close()

	cfg := &config.SpokeConfig{
		Protocol: "https",
		Timeout:  10 * time.Second,
		TLS: config.TLSConfig{
			Enabled:    true,
			SkipVerify: true,
		},
		Retry: config.RetryConfig{
			Enabled:           true,
			MaxRetries:        3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			JitterEnabled:     false, // Disable jitter for predictable timing
		},
	}

	client := NewSpokeClient(cfg, nil)

	// Create a request
	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	// Perform request with retry
	start := time.Now()
	resp, err := client.doRequestWithRetry(req)
	duration := time.Since(start)

	// Should succeed after retries
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Should have made 3 attempts (initial + 2 retries)
	assert.Equal(t, 3, retryCount)

	// Should have taken some time due to retry delays
	assert.True(t, duration > 20*time.Millisecond, "Expected some delay from retries")
}

func TestSpokeClient_BuildSpokeURL(t *testing.T) {
	cfg := &config.SpokeConfig{
		Protocol: "https",
	}

	client := NewSpokeClient(cfg, nil)

	tests := []struct {
		fqdn     string
		path     string
		expected string
	}{
		{
			fqdn:     "spoke.example.com",
			path:     "/health",
			expected: "https://spoke.example.com/health",
		},
		{
			fqdn:     "spoke.example.com",
			path:     "health", // without leading slash
			expected: "https://spoke.example.com/health",
		},
		{
			fqdn:     "spoke.example.com",
			path:     "",
			expected: "https://spoke.example.com/",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("fqdn=%s,path=%s", tt.fqdn, tt.path), func(t *testing.T) {
			result := client.buildSpokeURL(tt.fqdn, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSpokeClient_DiscoveryIntegration(t *testing.T) {
	// Test environment-based discovery
	cfg := &config.SpokeConfig{
		HostPattern:  "ovim-spoke-agent",
		DomainSuffix: "example.com",
		FQDNTemplate: "{{.HostPattern}}-{{.ClusterID}}.{{.DomainSuffix}}",
		CustomFQDNs:  make(map[string]string),
		Discovery: config.DiscoveryConfig{
			Source:          "environment",
			SpokeListEnv:    "cluster1:zone1,cluster2:zone2:custom.example.com",
			RefreshInterval: 1 * time.Minute,
		},
		HealthCheck: config.HealthCheckConfig{
			Enabled: false, // Disable health checking for this test
		},
	}

	client := NewSpokeClient(cfg, nil)

	// Test discovery
	err := client.discoverFromEnvironment()
	require.NoError(t, err)

	spokes := client.GetSpokes()
	require.Len(t, spokes, 2)

	// Check first spoke (auto-generated FQDN)
	spoke1, exists := spokes["cluster1"]
	require.True(t, exists)
	assert.Equal(t, "cluster1", spoke1.ClusterID)
	assert.Equal(t, "zone1", spoke1.ZoneID)
	assert.Equal(t, "ovim-spoke-agent-cluster1.example.com", spoke1.FQDN)
	assert.True(t, spoke1.Enabled)

	// Check second spoke (custom FQDN)
	spoke2, exists := spokes["cluster2"]
	require.True(t, exists)
	assert.Equal(t, "cluster2", spoke2.ClusterID)
	assert.Equal(t, "zone2", spoke2.ZoneID)
	assert.Equal(t, "custom.example.com", spoke2.FQDN)
	assert.True(t, spoke2.Enabled)
}

func TestSpokeClient_StartStop(t *testing.T) {
	cfg := &config.SpokeConfig{
		Discovery: config.DiscoveryConfig{
			Source:          "environment",
			SpokeListEnv:    "", // Empty list
			RefreshInterval: 100 * time.Millisecond,
		},
		HealthCheck: config.HealthCheckConfig{
			Enabled:  true,
			Interval: 100 * time.Millisecond,
			Timeout:  1 * time.Second,
		},
	}

	client := NewSpokeClient(cfg, nil)

	// Start the client
	err := client.Start()
	require.NoError(t, err)

	// Let it run briefly
	time.Sleep(150 * time.Millisecond)

	// Stop the client
	err = client.Stop()
	assert.NoError(t, err)
}

// Integration test with a real mock spoke agent
func TestSpokeClient_RealWorldIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a mock spoke agent server
	spokeServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			response := map[string]interface{}{
				"status":     "healthy",
				"cluster_id": "test-cluster",
				"agent_id":   "spoke-agent-test",
				"timestamp":  time.Now().Unix(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		case "/operations":
			// Mock operations endpoint
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer spokeServer.Close()

	// Extract FQDN from server URL
	serverFQDN := strings.TrimPrefix(spokeServer.URL, "https://")

	// Configure spoke client
	cfg := &config.SpokeConfig{
		Protocol: "https",
		Timeout:  10 * time.Second,
		TLS: config.TLSConfig{
			Enabled:    true,
			SkipVerify: true,
		},
		Discovery: config.DiscoveryConfig{
			Source:          "environment",
			SpokeListEnv:    fmt.Sprintf("test-cluster:test-zone:%s", serverFQDN),
			RefreshInterval: 1 * time.Minute,
		},
		HealthCheck: config.HealthCheckConfig{
			Enabled:  true,
			Interval: 5 * time.Second,
			Timeout:  2 * time.Second,
			Path:     "/health",
		},
		Retry: config.RetryConfig{
			Enabled:           true,
			MaxRetries:        3,
			InitialDelay:      100 * time.Millisecond,
			MaxDelay:          1 * time.Second,
			BackoffMultiplier: 2.0,
		},
	}

	client := NewSpokeClient(cfg, nil)

	// Start the client
	err := client.Start()
	require.NoError(t, err)
	defer client.Stop()

	// Wait for discovery to complete
	time.Sleep(100 * time.Millisecond)

	// Test spoke discovery
	spokes := client.GetSpokes()
	require.Len(t, spokes, 1)

	spoke, exists := spokes["test-cluster"]
	require.True(t, exists)
	assert.Equal(t, "test-cluster", spoke.ClusterID)
	assert.Equal(t, "test-zone", spoke.ZoneID)
	assert.Equal(t, serverFQDN, spoke.FQDN)

	// Test health check
	health, err := client.CheckSpokeHealth("test-cluster")
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "test-cluster", health.ClusterID)

	// Test sending a request to spoke
	resp, err := client.SendToSpoke("test-cluster", "/operations", "GET", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Wait for health check to run automatically
	time.Sleep(6 * time.Second)

	// Verify health status was updated
	healthStatuses := client.GetHealthStatus()
	require.Len(t, healthStatuses, 1)

	healthStatus, exists := healthStatuses["test-cluster"]
	require.True(t, exists)
	assert.Equal(t, "healthy", healthStatus.Status)
}