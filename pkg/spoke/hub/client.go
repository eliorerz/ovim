package hub

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
)

// HTTPClient implements the HubClient interface using HTTP REST API
type HTTPClient struct {
	config     *config.SpokeConfig
	httpClient *http.Client
	baseURL    string
	logger     *slog.Logger

	// Connection state
	connected   bool
	lastContact time.Time
	operations  chan *spoke.Operation
	mu          sync.RWMutex

	// Context for background operations
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Retry configuration
	maxRetries         int
	baseRetryDelay     time.Duration
	maxRetryDelay      time.Duration
	retryBackoffFactor float64
}

// NewHTTPClient creates a new HTTP-based hub client
func NewHTTPClient(cfg *config.SpokeConfig, logger *slog.Logger) *HTTPClient {
	ctx, cancel := context.WithCancel(context.Background())

	// Configure HTTP client with TLS if enabled
	var tlsConfig *tls.Config
	if cfg.Hub.TLSEnabled {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: cfg.Hub.TLSSkipVerify,
		}

		// Load client certificates if provided
		if cfg.Hub.CertificatePath != "" && cfg.Hub.PrivateKeyPath != "" {
			cert, err := tls.LoadX509KeyPair(cfg.Hub.CertificatePath, cfg.Hub.PrivateKeyPath)
			if err != nil {
				logger.Error("Failed to load client certificates", "error", err)
			} else {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}
	}

	httpClient := &http.Client{
		Timeout: cfg.Hub.Timeout,
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
	}

	return &HTTPClient{
		config:             cfg,
		httpClient:         httpClient,
		baseURL:            cfg.Hub.Endpoint,
		logger:             logger,
		operations:         make(chan *spoke.Operation, 100),
		ctx:                ctx,
		cancel:             cancel,
		maxRetries:         5,
		baseRetryDelay:     1 * time.Second,
		maxRetryDelay:      60 * time.Second,
		retryBackoffFactor: 2.0,
	}
}

// Connect establishes connection to the hub
func (c *HTTPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("Connecting to hub", "endpoint", c.baseURL)

	// Test connection with a simple health check
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	c.addAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to hub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hub health check failed with status: %d", resp.StatusCode)
	}

	c.connected = true
	c.lastContact = time.Now()

	// Operation polling removed - using push-based messaging

	c.logger.Info("Successfully connected to hub")
	return nil
}

// Disconnect closes the connection to the hub
func (c *HTTPClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.logger.Info("Disconnecting from hub")

	// Cancel background operations
	c.cancel()

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("All background operations stopped")
	case <-time.After(10 * time.Second):
		c.logger.Warn("Timeout waiting for background operations to stop")
	}

	c.connected = false
	close(c.operations)

	c.logger.Info("Disconnected from hub")
	return nil
}

// SendStatusReport sends a status report to the hub
func (c *HTTPClient) SendStatusReport(ctx context.Context, report *spoke.StatusReport) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected to hub")
	}
	c.mu.RUnlock()

	// Add callback URL for push notifications if local API is enabled
	if c.config.API.Enabled {
		var callbackURL string
		if c.config.API.CallbackURL != "" {
			// Use explicitly configured callback URL (e.g., external FQDN)
			callbackURL = c.config.API.CallbackURL
		} else {
			// Fall back to constructed URL from address and port
			callbackURL = fmt.Sprintf("http://%s:%d", c.config.API.Address, c.config.API.Port)
			if c.config.API.Address == "0.0.0.0" {
				// Use localhost for callback when binding to all interfaces
				callbackURL = fmt.Sprintf("http://localhost:%d", c.config.API.Port)
			}
		}
		report.CallbackURL = callbackURL
	}

	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal status report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/spoke/status", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create status report request: %w", err)
	}

	c.addAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send status report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status report failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.updateLastContact()
	return nil
}

// ReceiveOperations returns a channel for receiving operations from the hub
func (c *HTTPClient) ReceiveOperations() <-chan *spoke.Operation {
	return c.operations
}

// SendOperationResult sends an operation result back to the hub
func (c *HTTPClient) SendOperationResult(ctx context.Context, result *spoke.OperationResult) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected to hub")
	}
	c.mu.RUnlock()

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal operation result: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/spoke/operations/%s/result", c.baseURL, result.OperationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create operation result request: %w", err)
	}

	c.addAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send operation result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("operation result failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.updateLastContact()
	return nil
}

// IsConnected returns true if connected to the hub
func (c *HTTPClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetLastContact returns the time of last successful contact with hub
func (c *HTTPClient) GetLastContact() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastContact
}

// ReceiveOperation receives a single operation via push notification (public method)
func (c *HTTPClient) ReceiveOperation(operation *spoke.Operation) {
	select {
	case c.operations <- operation:
		c.logger.Info("Received operation via push notification", "operation_id", operation.ID, "type", operation.Type)
	case <-c.ctx.Done():
		return
	default:
		c.logger.Warn("Operations channel full, dropping operation", "operation_id", operation.ID)
	}
}

// addAuthHeaders adds authentication headers to the request
func (c *HTTPClient) addAuthHeaders(req *http.Request) {
	// Add agent identification headers
	req.Header.Set("X-Agent-ID", c.config.AgentID)
	req.Header.Set("X-Cluster-ID", c.config.ClusterID)
	req.Header.Set("X-Zone-ID", c.config.ZoneID)
	req.Header.Set("X-Agent-Version", c.config.Version)

	// TODO: Add proper authentication (JWT token, client certificates, etc.)
	// For now, using simple header-based auth
	req.Header.Set("Authorization", "Bearer spoke-agent-token")
}

// updateLastContact updates the last contact time
func (c *HTTPClient) updateLastContact() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastContact = time.Now()
}

// isRetryableError determines if an error is worth retrying
func (c *HTTPClient) isRetryableError(err error) bool {
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
func (c *HTTPClient) isRetryableStatusCode(statusCode int) bool {
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

// calculateRetryDelay calculates the delay for the next retry attempt
func (c *HTTPClient) calculateRetryDelay(attempt int) time.Duration {
	delay := time.Duration(float64(c.baseRetryDelay) * math.Pow(c.retryBackoffFactor, float64(attempt)))
	if delay > c.maxRetryDelay {
		delay = c.maxRetryDelay
	}

	// Add some jitter to avoid thundering herd
	jitter := time.Duration(float64(delay) * 0.1 * (2.0*rand.Float64() - 1.0))
	return delay + jitter
}

// doRequestWithRetry performs an HTTP request with retry logic
func (c *HTTPClient) doRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateRetryDelay(attempt - 1)
			c.logger.Debug("Retrying request",
				"attempt", attempt,
				"delay", delay,
				"method", req.Method,
				"url", req.URL.String())

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Clone the request for retry attempts
		reqClone := req.Clone(ctx)

		resp, lastErr = c.httpClient.Do(reqClone)
		if lastErr != nil {
			if !c.isRetryableError(lastErr) {
				c.logger.Debug("Non-retryable error", "error", lastErr)
				break
			}
			c.logger.Warn("Request failed, will retry",
				"attempt", attempt+1,
				"error", lastErr,
				"method", req.Method,
				"url", req.URL.String())
			continue
		}

		// Check if the status code is retryable
		if c.isRetryableStatusCode(resp.StatusCode) {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("received retryable status code %d: %s", resp.StatusCode, string(body))
			c.logger.Warn("Retryable status code",
				"attempt", attempt+1,
				"status_code", resp.StatusCode,
				"method", req.Method,
				"url", req.URL.String())
			continue
		}

		// Success or non-retryable error
		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
	}

	return resp, nil
}
