package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	"k8s.io/klog/v2"
)

// SpokeInfo represents information about a spoke agent
type SpokeInfo struct {
	ClusterID string    `json:"cluster_id"`
	ZoneID    string    `json:"zone_id"`
	FQDN      string    `json:"fqdn"`
	Status    string    `json:"status"`
	LastSeen  time.Time `json:"last_seen"`
	Enabled   bool      `json:"enabled"`
}

// SpokeHealthResponse represents the response from a spoke health check
type SpokeHealthResponse struct {
	Status    string    `json:"status"`
	ClusterID string    `json:"cluster_id"`
	FQDN      string    `json:"fqdn"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// SpokeClient manages communication with spoke agents using dynamic FQDN discovery
type SpokeClient struct {
	config     *config.SpokeConfig
	storage    storage.Storage
	httpClient *http.Client

	// Spoke discovery and state
	spokes    map[string]*SpokeInfo
	spokesMux sync.RWMutex

	// Health monitoring
	healthStatus map[string]*SpokeHealthResponse
	healthMux    sync.RWMutex

	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSpokeClient creates a new spoke client with the given configuration
func NewSpokeClient(cfg *config.SpokeConfig, storage storage.Storage) *SpokeClient {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure HTTP client with TLS settings
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.TLS.SkipVerify,
	}

	client := &SpokeClient{
		config:       cfg,
		storage:      storage,
		spokes:       make(map[string]*SpokeInfo),
		healthStatus: make(map[string]*SpokeHealthResponse),
		ctx:          ctx,
		cancel:       cancel,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				TLSClientConfig:       tlsConfig,
				DisableKeepAlives:     false,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
	}

	return client
}

// Start starts the spoke client with periodic discovery and health checking
func (c *SpokeClient) Start() error {
	klog.Infof("Starting spoke client with discovery source: %s", c.config.Discovery.Source)

	// Initial spoke discovery
	if err := c.discoverSpokes(); err != nil {
		klog.Errorf("Initial spoke discovery failed: %v", err)
		// Continue starting even if discovery fails
	}

	// Start periodic discovery refresh
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.periodicDiscovery()
	}()

	// Start health checking if enabled
	if c.config.HealthCheck.Enabled {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.periodicHealthCheck()
		}()
	}

	klog.Info("Spoke client started successfully")
	return nil
}

// Stop gracefully stops the spoke client
func (c *SpokeClient) Stop() error {
	klog.Info("Stopping spoke client")

	c.cancel()

	// Wait for all goroutines to complete with timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		klog.Info("Spoke client stopped gracefully")
	case <-time.After(30 * time.Second):
		klog.Warningf("Timeout waiting for spoke client to stop")
	}

	return nil
}

// GetSpokes returns all discovered spokes
func (c *SpokeClient) GetSpokes() map[string]*SpokeInfo {
	c.spokesMux.RLock()
	defer c.spokesMux.RUnlock()

	result := make(map[string]*SpokeInfo)
	for k, v := range c.spokes {
		result[k] = v
	}
	return result
}

// GetSpokeByClusterID returns spoke info for a specific cluster
func (c *SpokeClient) GetSpokeByClusterID(clusterID string) (*SpokeInfo, bool) {
	c.spokesMux.RLock()
	defer c.spokesMux.RUnlock()

	spoke, exists := c.spokes[clusterID]
	return spoke, exists
}

// GetHealthStatus returns health status for all spokes
func (c *SpokeClient) GetHealthStatus() map[string]*SpokeHealthResponse {
	c.healthMux.RLock()
	defer c.healthMux.RUnlock()

	result := make(map[string]*SpokeHealthResponse)
	for k, v := range c.healthStatus {
		result[k] = v
	}
	return result
}

// CheckSpokeHealth performs health check for a specific spoke
func (c *SpokeClient) CheckSpokeHealth(clusterID string) (*SpokeHealthResponse, error) {
	spoke, exists := c.GetSpokeByClusterID(clusterID)
	if !exists {
		return nil, fmt.Errorf("spoke not found for cluster %s", clusterID)
	}

	return c.performHealthCheck(spoke)
}

// SendToSpoke sends a request to a specific spoke agent
func (c *SpokeClient) SendToSpoke(clusterID, path string, method string, payload interface{}) (*http.Response, error) {
	spoke, exists := c.GetSpokeByClusterID(clusterID)
	if !exists {
		return nil, fmt.Errorf("spoke not found for cluster %s", clusterID)
	}

	url := c.buildSpokeURL(spoke.FQDN, path)

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(c.ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.doRequestWithRetry(req)
}

// discoverSpokes discovers spoke agents based on configuration
func (c *SpokeClient) discoverSpokes() error {
	switch c.config.Discovery.Source {
	case "environment":
		return c.discoverFromEnvironment()
	case "database":
		return c.discoverFromDatabase()
	case "crd":
		return c.discoverFromCRD()
	case "config":
		return c.discoverFromConfig()
	default:
		return fmt.Errorf("unsupported discovery source: %s", c.config.Discovery.Source)
	}
}

// discoverFromEnvironment discovers spokes from environment variables
func (c *SpokeClient) discoverFromEnvironment() error {
	spokes, err := c.getSpokeListFromEnv()
	if err != nil {
		return fmt.Errorf("failed to get spoke list from environment: %w", err)
	}

	c.spokesMux.Lock()
	defer c.spokesMux.Unlock()

	// Clear existing spokes
	c.spokes = make(map[string]*SpokeInfo)

	// Add discovered spokes
	for _, spoke := range spokes {
		c.spokes[spoke.ClusterID] = &spoke
		klog.V(4).Infof("Discovered spoke: cluster=%s, zone=%s, fqdn=%s",
			spoke.ClusterID, spoke.ZoneID, spoke.FQDN)
	}

	klog.Infof("Discovered %d spokes from environment", len(spokes))
	return nil
}

// discoverFromDatabase discovers spokes from database
func (c *SpokeClient) discoverFromDatabase() error {
	// TODO: Implement database-based discovery
	// This would query a spokes table or similar
	klog.V(2).Info("Database-based spoke discovery not yet implemented")
	return nil
}

// discoverFromCRD discovers spokes from Kubernetes CRDs
func (c *SpokeClient) discoverFromCRD() error {
	// TODO: Implement CRD-based discovery
	// This would query for spoke agent CRDs
	klog.V(2).Info("CRD-based spoke discovery not yet implemented")
	return nil
}

// discoverFromConfig discovers spokes from static configuration
func (c *SpokeClient) discoverFromConfig() error {
	// TODO: Implement config file-based discovery
	klog.V(2).Info("Config-based spoke discovery not yet implemented")
	return nil
}

// getSpokeListFromEnv parses spoke list from environment variable
func (c *SpokeClient) getSpokeListFromEnv() ([]SpokeInfo, error) {
	if c.config.Discovery.SpokeListEnv == "" {
		return nil, fmt.Errorf("spoke list environment variable not configured")
	}

	spokeListStr := strings.TrimSpace(c.config.Discovery.SpokeListEnv)
	if spokeListStr == "" {
		return []SpokeInfo{}, nil // Empty list is valid
	}

	var spokes []SpokeInfo
	entries := strings.Split(spokeListStr, ",")

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.Split(entry, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid spoke entry format '%s', expected 'cluster:zone[:fqdn]'", entry)
		}

		clusterID := strings.TrimSpace(parts[0])
		zoneID := strings.TrimSpace(parts[1])

		if clusterID == "" || zoneID == "" {
			return nil, fmt.Errorf("cluster ID and zone ID cannot be empty in entry '%s'", entry)
		}

		var fqdn string
		if len(parts) >= 3 && strings.TrimSpace(parts[2]) != "" {
			fqdn = strings.TrimSpace(parts[2])
		} else {
			// Generate FQDN using template
			var err error
			fqdn, err = c.generateFQDN(clusterID)
			if err != nil {
				return nil, fmt.Errorf("failed to generate FQDN for cluster '%s': %w", clusterID, err)
			}
		}

		spokes = append(spokes, SpokeInfo{
			ClusterID: clusterID,
			ZoneID:    zoneID,
			FQDN:      fqdn,
			Status:    "unknown",
			Enabled:   true,
		})
	}

	return spokes, nil
}

// generateFQDN generates FQDN for a cluster using the configured template
func (c *SpokeClient) generateFQDN(clusterID string) (string, error) {
	// Check for custom FQDN first
	if customFQDN, exists := c.config.CustomFQDNs[clusterID]; exists {
		return customFQDN, nil
	}

	// Validate required configuration for template generation
	if c.config.DomainSuffix == "" {
		return "", fmt.Errorf("domain suffix is required for FQDN generation")
	}

	// Simple template replacement
	fqdn := c.config.FQDNTemplate
	fqdn = strings.ReplaceAll(fqdn, "{{.HostPattern}}", c.config.HostPattern)
	fqdn = strings.ReplaceAll(fqdn, "{{.ClusterID}}", clusterID)
	fqdn = strings.ReplaceAll(fqdn, "{{.DomainSuffix}}", c.config.DomainSuffix)

	if strings.Contains(fqdn, "{{") {
		return "", fmt.Errorf("unresolved template variables in FQDN template")
	}

	return fqdn, nil
}

// buildSpokeURL builds the full URL for a spoke agent endpoint
func (c *SpokeClient) buildSpokeURL(fqdn, path string) string {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return fmt.Sprintf("%s://%s%s", c.config.Protocol, fqdn, path)
}

// periodicDiscovery runs periodic spoke discovery
func (c *SpokeClient) periodicDiscovery() {
	ticker := time.NewTicker(c.config.Discovery.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.discoverSpokes(); err != nil {
				klog.Errorf("Periodic spoke discovery failed: %v", err)
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// periodicHealthCheck runs periodic health checks on all spokes
func (c *SpokeClient) periodicHealthCheck() {
	ticker := time.NewTicker(c.config.HealthCheck.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.performAllHealthChecks()
		case <-c.ctx.Done():
			return
		}
	}
}

// performAllHealthChecks checks health of all discovered spokes
func (c *SpokeClient) performAllHealthChecks() {
	spokes := c.GetSpokes()

	for clusterID, spoke := range spokes {
		if !spoke.Enabled {
			continue
		}

		go func(cid string, s *SpokeInfo) {
			health, err := c.performHealthCheck(s)
			if err != nil {
				health = &SpokeHealthResponse{
					Status:    "unhealthy",
					ClusterID: cid,
					FQDN:      s.FQDN,
					Timestamp: time.Now(),
					Error:     err.Error(),
				}
			}

			c.healthMux.Lock()
			c.healthStatus[cid] = health
			c.healthMux.Unlock()
		}(clusterID, spoke)
	}
}

// performHealthCheck performs a health check on a specific spoke
func (c *SpokeClient) performHealthCheck(spoke *SpokeInfo) (*SpokeHealthResponse, error) {
	url := c.buildSpokeURL(spoke.FQDN, c.config.HealthCheck.Path)

	ctx, cancel := context.WithTimeout(c.ctx, c.config.HealthCheck.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return &SpokeHealthResponse{
		Status:    "healthy",
		ClusterID: spoke.ClusterID,
		FQDN:      spoke.FQDN,
		Timestamp: time.Now(),
	}, nil
}

// doRequestWithRetry performs an HTTP request with exponential backoff retry logic
func (c *SpokeClient) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	if !c.config.Retry.Enabled {
		return c.httpClient.Do(req)
	}

	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= c.config.Retry.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateRetryDelay(attempt - 1)
			klog.V(4).Infof("Retrying spoke request (attempt %d) after %v: %s %s",
				attempt, delay, req.Method, req.URL.String())

			select {
			case <-time.After(delay):
			case <-c.ctx.Done():
				return nil, c.ctx.Err()
			}
		}

		// Clone the request for retry attempts
		reqClone := req.Clone(req.Context())

		resp, lastErr = c.httpClient.Do(reqClone)
		if lastErr != nil {
			if !c.isRetryableError(lastErr) {
				break
			}
			klog.V(4).Infof("Retryable error on spoke request: %v", lastErr)
			continue
		}

		// Check if the status code is retryable
		if c.isRetryableStatusCode(resp.StatusCode) {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("received retryable status code %d: %s", resp.StatusCode, string(body))
			continue
		}

		// Success or non-retryable error
		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.config.Retry.MaxRetries+1, lastErr)
	}

	return resp, nil
}

// calculateRetryDelay calculates the delay for the next retry attempt with exponential backoff
func (c *SpokeClient) calculateRetryDelay(attempt int) time.Duration {
	delay := time.Duration(float64(c.config.Retry.InitialDelay) *
		math.Pow(c.config.Retry.BackoffMultiplier, float64(attempt)))

	if delay > c.config.Retry.MaxDelay {
		delay = c.config.Retry.MaxDelay
	}

	// Add jitter if enabled
	if c.config.Retry.JitterEnabled {
		jitter := time.Duration(float64(delay) * 0.1 * (2.0*rand.Float64() - 1.0))
		delay += jitter
	}

	return delay
}

// isRetryableError determines if an error is worth retrying
func (c *SpokeClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are generally retryable
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Context cancellation should not be retried
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	return true
}

// isRetryableStatusCode determines if an HTTP status code is worth retrying
func (c *SpokeClient) isRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}
