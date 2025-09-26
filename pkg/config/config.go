package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	// Default configuration values
	DefaultPort            = "8080"
	DefaultTLSPort         = "8443"
	DefaultJWTSecret       = "ovim-default-secret-change-in-production"
	DefaultEnvironment     = "development"
	DefaultDatabaseURL     = "postgres://ovim:ovim@localhost/ovim?sslmode=disable"
	DefaultShutdownTimeout = 30 * time.Second
	DefaultReadTimeout     = 10 * time.Second
	DefaultWriteTimeout    = 10 * time.Second
	DefaultIdleTimeout     = 60 * time.Second

	// Environment variable names
	EnvPort                = "OVIM_PORT"
	EnvTLSEnabled          = "OVIM_TLS_ENABLED"
	EnvTLSPort             = "OVIM_TLS_PORT"
	EnvTLSCertFile         = "OVIM_TLS_CERT_FILE"
	EnvTLSKeyFile          = "OVIM_TLS_KEY_FILE"
	EnvTLSAutoGenerateCert = "OVIM_TLS_AUTO_GENERATE_CERT"
	EnvDatabaseURL         = "OVIM_DATABASE_URL"
	EnvKubernetesConfig    = "OVIM_KUBECONFIG"
	EnvKubernetesInCluster = "OVIM_KUBERNETES_IN_CLUSTER"
	EnvKubevirtEnabled     = "OVIM_KUBEVIRT_ENABLED"
	EnvKubevirtNamespace   = "OVIM_KUBEVIRT_NAMESPACE"
	EnvJWTSecret           = "OVIM_JWT_SECRET"
	EnvEnvironment         = "OVIM_ENVIRONMENT"
	EnvLogLevel            = "OVIM_LOG_LEVEL"

	// OIDC Environment variables
	EnvOIDCEnabled      = "OVIM_OIDC_ENABLED"
	EnvOIDCIssuerURL    = "OVIM_OIDC_ISSUER_URL"
	EnvOIDCClientID     = "OVIM_OIDC_CLIENT_ID"
	EnvOIDCClientSecret = "OVIM_OIDC_CLIENT_SECRET"
	EnvOIDCRedirectURL  = "OVIM_OIDC_REDIRECT_URL"

	// OpenShift Environment variables
	EnvOpenShiftEnabled           = "OVIM_OPENSHIFT_ENABLED"
	EnvOpenShiftConfig            = "OVIM_OPENSHIFT_KUBECONFIG"
	EnvOpenShiftInCluster         = "OVIM_OPENSHIFT_IN_CLUSTER"
	EnvOpenShiftTemplateNamespace = "OVIM_OPENSHIFT_TEMPLATE_NAMESPACE"

	// Spoke Agent Environment variables
	EnvSpokeDomainSuffix      = "OVIM_SPOKE_DOMAIN_SUFFIX"
	EnvSpokeHostPattern       = "OVIM_SPOKE_HOST_PATTERN"
	EnvSpokeFQDNTemplate      = "OVIM_SPOKE_FQDN_TEMPLATE"
	EnvSpokeCustomFQDNs       = "OVIM_SPOKE_CUSTOM_FQDNS"
	EnvSpokeProtocol          = "OVIM_SPOKE_PROTOCOL"
	EnvSpokeTLSSkipVerify     = "OVIM_SPOKE_TLS_SKIP_VERIFY"
	EnvSpokeTimeout           = "OVIM_SPOKE_TIMEOUT"
	EnvSpokeRetryEnabled      = "OVIM_SPOKE_RETRY_ENABLED"
	EnvSpokeMaxRetries        = "OVIM_SPOKE_MAX_RETRIES"
	EnvSpokeInitialDelay      = "OVIM_SPOKE_INITIAL_DELAY"
	EnvSpokeMaxDelay          = "OVIM_SPOKE_MAX_DELAY"
	EnvSpokeBackoffMultiplier = "OVIM_SPOKE_BACKOFF_MULTIPLIER"
	EnvSpokeHealthEnabled     = "OVIM_SPOKE_HEALTH_ENABLED"
	EnvSpokeHealthInterval    = "OVIM_SPOKE_HEALTH_INTERVAL"
	EnvSpokeHealthTimeout     = "OVIM_SPOKE_HEALTH_TIMEOUT"
	EnvSpokeDiscoverySource   = "OVIM_SPOKE_DISCOVERY_SOURCE"
	EnvSpokeListEnv           = "OVIM_SPOKE_LIST"
	EnvSpokeRefreshInterval   = "OVIM_SPOKE_REFRESH_INTERVAL"
)

// Config holds all configuration for the OVIM backend
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	OpenShift  OpenShiftConfig  `yaml:"openshift"`
	Auth       AuthConfig       `yaml:"auth"`
	Logging    LoggingConfig    `yaml:"logging"`
	Spoke      SpokeConfig      `yaml:"spoke"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string        `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"readTimeout"`
	WriteTimeout time.Duration `yaml:"writeTimeout"`
	IdleTimeout  time.Duration `yaml:"idleTimeout"`
	Environment  string        `yaml:"environment"`
	TLS          TLSConfig     `yaml:"tls"`
}

// TLSConfig holds TLS/HTTPS configuration
type TLSConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Port             string `yaml:"port"`
	CertFile         string `yaml:"certFile"`
	KeyFile          string `yaml:"keyFile"`
	AutoGenerateCert bool   `yaml:"autoGenerateCert"`
	SkipVerify       bool   `yaml:"skipVerify"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL             string        `yaml:"url"`
	MaxOpenConns    int           `yaml:"maxOpenConns"`
	MaxIdleConns    int           `yaml:"maxIdleConns"`
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime"`
}

// KubernetesConfig holds Kubernetes client configuration
type KubernetesConfig struct {
	ConfigPath string         `yaml:"configPath"`
	InCluster  bool           `yaml:"inCluster"`
	KubeVirt   KubeVirtConfig `yaml:"kubevirt"`
}

// KubeVirtConfig holds KubeVirt-specific configuration
type KubeVirtConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Namespace string `yaml:"namespace"`
	UseMock   bool   `yaml:"useMock"`
}

// OpenShiftConfig holds OpenShift client configuration
type OpenShiftConfig struct {
	Enabled           bool   `yaml:"enabled"`
	ConfigPath        string `yaml:"configPath"`
	InCluster         bool   `yaml:"inCluster"`
	TemplateNamespace string `yaml:"templateNamespace"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret     string        `yaml:"jwtSecret"`
	TokenDuration time.Duration `yaml:"tokenDuration"`
	OIDC          OIDCConfig    `yaml:"oidc"`
}

// OIDCConfig holds OpenID Connect configuration
type OIDCConfig struct {
	Enabled      bool     `yaml:"enabled"`
	IssuerURL    string   `yaml:"issuerUrl"`
	ClientID     string   `yaml:"clientId"`
	ClientSecret string   `yaml:"clientSecret"`
	RedirectURL  string   `yaml:"redirectUrl"`
	Scopes       []string `yaml:"scopes"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// SpokeConfig holds spoke agent communication configuration
type SpokeConfig struct {
	// FQDN discovery configuration
	DomainSuffix string            `yaml:"domain_suffix"`
	HostPattern  string            `yaml:"host_pattern"`
	FQDNTemplate string            `yaml:"fqdn_template"`
	CustomFQDNs  map[string]string `yaml:"custom_fqdns"`

	// Communication settings
	Protocol string        `yaml:"protocol"`
	TLS      TLSConfig     `yaml:"tls"`
	Timeout  time.Duration `yaml:"timeout"`

	// Retry configuration
	Retry RetryConfig `yaml:"retry"`

	// Health checking
	HealthCheck HealthCheckConfig `yaml:"health_check"`

	// Discovery configuration
	Discovery DiscoveryConfig `yaml:"discovery"`
}

// RetryConfig represents retry configuration for spoke communication
type RetryConfig struct {
	Enabled           bool          `yaml:"enabled"`
	MaxRetries        int           `yaml:"max_retries"`
	InitialDelay      time.Duration `yaml:"initial_delay"`
	MaxDelay          time.Duration `yaml:"max_delay"`
	BackoffMultiplier float64       `yaml:"backoff_multiplier"`
	JitterEnabled     bool          `yaml:"jitter_enabled"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Path     string        `yaml:"path"`
	Port     int           `yaml:"port"`
}

// DiscoveryConfig represents spoke discovery configuration
type DiscoveryConfig struct {
	// Source of spoke agent information
	Source string `yaml:"source"` // "config", "database", "crd", "environment"

	// Environment variable names for static spoke list
	SpokeListEnv string `yaml:"spoke_list_env"`

	// Database query configuration
	DatabaseQuery string `yaml:"database_query"`

	// CRD selector for spoke discovery
	CRDSelector map[string]string `yaml:"crd_selector"`

	// Refresh interval for dynamic discovery
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

// Load loads configuration from environment variables and config file
func Load(configPath string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnvString(EnvPort, DefaultPort),
			ReadTimeout:  DefaultReadTimeout,
			WriteTimeout: DefaultWriteTimeout,
			IdleTimeout:  DefaultIdleTimeout,
			Environment:  getEnvString(EnvEnvironment, DefaultEnvironment),
			TLS: TLSConfig{
				Enabled:          getEnvBool(EnvTLSEnabled, true),
				Port:             getEnvString(EnvTLSPort, DefaultTLSPort),
				CertFile:         getEnvString(EnvTLSCertFile, ""),
				KeyFile:          getEnvString(EnvTLSKeyFile, ""),
				AutoGenerateCert: getEnvBool(EnvTLSAutoGenerateCert, true),
			},
		},
		Database: DatabaseConfig{
			URL:             getEnvString(EnvDatabaseURL, DefaultDatabaseURL),
			MaxOpenConns:    25,
			MaxIdleConns:    25,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Kubernetes: KubernetesConfig{
			ConfigPath: getEnvString(EnvKubernetesConfig, ""),
			InCluster:  getEnvBool(EnvKubernetesInCluster, false),
			KubeVirt: KubeVirtConfig{
				Enabled:   getEnvBool(EnvKubevirtEnabled, true),
				Namespace: getEnvString(EnvKubevirtNamespace, "default"),
				UseMock:   getEnvString(EnvEnvironment, DefaultEnvironment) == "development",
			},
		},
		OpenShift: OpenShiftConfig{
			Enabled:           getEnvBool(EnvOpenShiftEnabled, false),
			ConfigPath:        getEnvString(EnvOpenShiftConfig, ""),
			InCluster:         getEnvBool(EnvOpenShiftInCluster, false),
			TemplateNamespace: getEnvString(EnvOpenShiftTemplateNamespace, "openshift"),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnvString(EnvJWTSecret, DefaultJWTSecret),
			TokenDuration: 24 * time.Hour,
			OIDC: OIDCConfig{
				Enabled:      getEnvBool(EnvOIDCEnabled, false),
				IssuerURL:    getEnvString(EnvOIDCIssuerURL, ""),
				ClientID:     getEnvString(EnvOIDCClientID, ""),
				ClientSecret: getEnvString(EnvOIDCClientSecret, ""),
				RedirectURL:  getEnvString(EnvOIDCRedirectURL, ""),
				Scopes:       []string{"openid", "profile", "email"},
			},
		},
		Logging: LoggingConfig{
			Level:  getEnvString(EnvLogLevel, "info"),
			Format: "json",
		},
		Spoke: SpokeConfig{
			// Default FQDN configuration
			DomainSuffix: getEnvString(EnvSpokeDomainSuffix, ""),
			HostPattern:  getEnvString(EnvSpokeHostPattern, "ovim-spoke-agent"),
			FQDNTemplate: getEnvString(EnvSpokeFQDNTemplate, "{{.HostPattern}}-{{.ClusterID}}.{{.DomainSuffix}}"),
			CustomFQDNs:  parseEnvJSON(EnvSpokeCustomFQDNs, make(map[string]string)),

			// Default communication settings
			Protocol: getEnvString(EnvSpokeProtocol, "https"),
			TLS: TLSConfig{
				Enabled:          true,
				AutoGenerateCert: false,
				SkipVerify:       getEnvBool(EnvSpokeTLSSkipVerify, false),
			},
			Timeout: parseDurationEnv(EnvSpokeTimeout, 30*time.Second),

			// Default retry configuration
			Retry: RetryConfig{
				Enabled:           getEnvBool(EnvSpokeRetryEnabled, true),
				MaxRetries:        getEnvInt(EnvSpokeMaxRetries, 3),
				InitialDelay:      parseDurationEnv(EnvSpokeInitialDelay, 1*time.Second),
				MaxDelay:          parseDurationEnv(EnvSpokeMaxDelay, 30*time.Second),
				BackoffMultiplier: parseFloatEnv(EnvSpokeBackoffMultiplier, 2.0),
				JitterEnabled:     true,
			},

			// Default health check configuration
			HealthCheck: HealthCheckConfig{
				Enabled:  getEnvBool(EnvSpokeHealthEnabled, true),
				Interval: parseDurationEnv(EnvSpokeHealthInterval, 60*time.Second),
				Timeout:  parseDurationEnv(EnvSpokeHealthTimeout, 10*time.Second),
				Path:     "/health",
				Port:     8080,
			},

			// Default discovery configuration
			Discovery: DiscoveryConfig{
				Source:          getEnvString(EnvSpokeDiscoverySource, "environment"),
				SpokeListEnv:    getEnvString(EnvSpokeListEnv, ""),
				RefreshInterval: parseDurationEnv(EnvSpokeRefreshInterval, 5*time.Minute),
			},
		},
	}

	// Load from config file if provided
	if configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// validate ensures the configuration is valid
func (c *Config) validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT secret cannot be empty")
	}
	if c.Auth.JWTSecret == DefaultJWTSecret && c.Server.Environment == "production" {
		return fmt.Errorf("default JWT secret cannot be used in production")
	}
	return nil
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(cfg *Config, path string) error {
	// TODO: Implement YAML config file loading
	// For now, we only use environment variables
	return nil
}

// getEnvString gets a string environment variable with a default value
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// parseDurationEnv parses a duration environment variable with a default value
func parseDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// parseFloatEnv parses a float environment variable with a default value
func parseFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// parseEnvJSON parses a JSON environment variable with a default value
func parseEnvJSON[T any](key string, defaultValue T) T {
	if value := os.Getenv(key); value != "" {
		var result T
		if err := json.Unmarshal([]byte(value), &result); err == nil {
			return result
		}
	}
	return defaultValue
}
