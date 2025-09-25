package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// SpokeConfig represents the configuration for the spoke agent
type SpokeConfig struct {
	// Agent identification
	AgentID   string `yaml:"agent_id" env:"OVIM_AGENT_ID"`
	ClusterID string `yaml:"cluster_id" env:"OVIM_CLUSTER_ID"`
	ZoneID    string `yaml:"zone_id" env:"OVIM_ZONE_ID"`
	Version   string `yaml:"version" env:"OVIM_VERSION"`

	// Hub connection
	Hub HubConfig `yaml:"hub"`

	// Local API server
	API APIConfig `yaml:"api"`

	// Kubernetes configuration
	Kubernetes KubernetesConfig `yaml:"kubernetes"`

	// Metrics and monitoring
	Metrics MetricsConfig `yaml:"metrics"`

	// Health checking
	Health HealthConfig `yaml:"health"`

	// Logging
	Logging LoggingConfig `yaml:"logging"`

	// Features
	Features FeatureConfig `yaml:"features"`
}

// HubConfig represents hub connection configuration
type HubConfig struct {
	Endpoint        string        `yaml:"endpoint" env:"OVIM_HUB_ENDPOINT"`
	Protocol        string        `yaml:"protocol" env:"OVIM_HUB_PROTOCOL"` // "https"
	TLSEnabled      bool          `yaml:"tls_enabled" env:"OVIM_HUB_TLS_ENABLED"`
	TLSSkipVerify   bool          `yaml:"tls_skip_verify" env:"OVIM_HUB_TLS_SKIP_VERIFY"`
	CertificatePath string        `yaml:"certificate_path" env:"OVIM_HUB_CERT_PATH"`
	PrivateKeyPath  string        `yaml:"private_key_path" env:"OVIM_HUB_KEY_PATH"`
	CAPath          string        `yaml:"ca_path" env:"OVIM_HUB_CA_PATH"`
	Timeout         time.Duration `yaml:"timeout" env:"OVIM_HUB_TIMEOUT"`
	RetryInterval   time.Duration `yaml:"retry_interval" env:"OVIM_HUB_RETRY_INTERVAL"`
	MaxRetries      int           `yaml:"max_retries" env:"OVIM_HUB_MAX_RETRIES"`
	KeepAlive       time.Duration `yaml:"keep_alive" env:"OVIM_HUB_KEEP_ALIVE"`
}

// APIConfig represents local API server configuration
type APIConfig struct {
	Enabled  bool   `yaml:"enabled" env:"OVIM_API_ENABLED"`
	Address  string `yaml:"address" env:"OVIM_API_ADDRESS"`
	Port     int    `yaml:"port" env:"OVIM_API_PORT"`
	TLS      bool   `yaml:"tls" env:"OVIM_API_TLS"`
	CertPath string `yaml:"cert_path" env:"OVIM_API_CERT_PATH"`
	KeyPath  string `yaml:"key_path" env:"OVIM_API_KEY_PATH"`
}

// KubernetesConfig represents Kubernetes client configuration
type KubernetesConfig struct {
	ConfigPath     string        `yaml:"config_path" env:"KUBECONFIG"`
	InCluster      bool          `yaml:"in_cluster" env:"OVIM_K8S_IN_CLUSTER"`
	QPS            float32       `yaml:"qps" env:"OVIM_K8S_QPS"`
	Burst          int           `yaml:"burst" env:"OVIM_K8S_BURST"`
	Timeout        time.Duration `yaml:"timeout" env:"OVIM_K8S_TIMEOUT"`
	ResyncInterval time.Duration `yaml:"resync_interval" env:"OVIM_K8S_RESYNC_INTERVAL"`
}

// MetricsConfig represents metrics collection configuration
type MetricsConfig struct {
	Enabled            bool          `yaml:"enabled" env:"OVIM_METRICS_ENABLED"`
	CollectionInterval time.Duration `yaml:"collection_interval" env:"OVIM_METRICS_COLLECTION_INTERVAL"`
	ReportingInterval  time.Duration `yaml:"reporting_interval" env:"OVIM_METRICS_REPORTING_INTERVAL"`
	RetentionPeriod    time.Duration `yaml:"retention_period" env:"OVIM_METRICS_RETENTION_PERIOD"`
	IncludeNodeMetrics bool          `yaml:"include_node_metrics" env:"OVIM_METRICS_INCLUDE_NODES"`
}

// HealthConfig represents health checking configuration
type HealthConfig struct {
	Enabled          bool          `yaml:"enabled" env:"OVIM_HEALTH_ENABLED"`
	CheckInterval    time.Duration `yaml:"check_interval" env:"OVIM_HEALTH_CHECK_INTERVAL"`
	ReportInterval   time.Duration `yaml:"report_interval" env:"OVIM_HEALTH_REPORT_INTERVAL"`
	Timeout          time.Duration `yaml:"timeout" env:"OVIM_HEALTH_TIMEOUT"`
	FailureThreshold int           `yaml:"failure_threshold" env:"OVIM_HEALTH_FAILURE_THRESHOLD"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level" env:"OVIM_LOG_LEVEL"`
	Format     string `yaml:"format" env:"OVIM_LOG_FORMAT"` // "json", "text"
	Output     string `yaml:"output" env:"OVIM_LOG_OUTPUT"` // "stdout", "stderr", "/path/to/file"
	MaxSize    int    `yaml:"max_size" env:"OVIM_LOG_MAX_SIZE"`
	MaxBackups int    `yaml:"max_backups" env:"OVIM_LOG_MAX_BACKUPS"`
	MaxAge     int    `yaml:"max_age" env:"OVIM_LOG_MAX_AGE"`
	Compress   bool   `yaml:"compress" env:"OVIM_LOG_COMPRESS"`
}

// FeatureConfig represents feature flags
type FeatureConfig struct {
	VDCManagement   bool `yaml:"vdc_management" env:"OVIM_FEATURE_VDC_MANAGEMENT"`
	TemplateSync    bool `yaml:"template_sync" env:"OVIM_FEATURE_TEMPLATE_SYNC"`
	NetworkPolicies bool `yaml:"network_policies" env:"OVIM_FEATURE_NETWORK_POLICIES"`
	LocalAPI        bool `yaml:"local_api" env:"OVIM_FEATURE_LOCAL_API"`
	EventRecording  bool `yaml:"event_recording" env:"OVIM_FEATURE_EVENT_RECORDING"`
}

// Default configuration values
var DefaultConfig = &SpokeConfig{
	AgentID:   generateAgentID(),
	ClusterID: getEnvOrDefault("CLUSTER_NAME", "unknown-cluster"),
	ZoneID:    getEnvOrDefault("ZONE_NAME", "default-zone"),
	Version:   "v1.0.0",

	Hub: HubConfig{
		Endpoint:      "https://ovim-hub:8443",
		Protocol:      "https",
		TLSEnabled:    true,
		TLSSkipVerify: false,
		Timeout:       30 * time.Second,
		RetryInterval: 5 * time.Second,
		MaxRetries:    3,
		KeepAlive:     60 * time.Second,
	},

	API: APIConfig{
		Enabled: true,
		Address: "0.0.0.0",
		Port:    8080,
		TLS:     false,
	},

	Kubernetes: KubernetesConfig{
		InCluster:      true,
		QPS:            20,
		Burst:          30,
		Timeout:        30 * time.Second,
		ResyncInterval: 10 * time.Minute,
	},

	Metrics: MetricsConfig{
		Enabled:            true,
		CollectionInterval: 30 * time.Second,
		ReportingInterval:  60 * time.Second,
		RetentionPeriod:    24 * time.Hour,
		IncludeNodeMetrics: true,
	},

	Health: HealthConfig{
		Enabled:          true,
		CheckInterval:    30 * time.Second,
		ReportInterval:   60 * time.Second,
		Timeout:          10 * time.Second,
		FailureThreshold: 3,
	},

	Logging: LoggingConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		MaxSize:    100, // MB
		MaxBackups: 3,
		MaxAge:     7, // days
		Compress:   true,
	},

	Features: FeatureConfig{
		VDCManagement:   true,
		TemplateSync:    true,
		NetworkPolicies: true,
		LocalAPI:        true,
		EventRecording:  true,
	},
}

// LoadConfig loads configuration from environment variables and default values
func LoadConfig() (*SpokeConfig, error) {
	config := &SpokeConfig{}
	*config = *DefaultConfig // Copy defaults

	// Load from environment variables
	if err := loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadFromEnv loads configuration values from environment variables
func loadFromEnv(config *SpokeConfig) error {
	// Agent identification
	if val := os.Getenv("OVIM_AGENT_ID"); val != "" {
		config.AgentID = val
	}
	if val := os.Getenv("OVIM_CLUSTER_ID"); val != "" {
		config.ClusterID = val
	}
	if val := os.Getenv("OVIM_ZONE_ID"); val != "" {
		config.ZoneID = val
	}
	if val := os.Getenv("OVIM_VERSION"); val != "" {
		config.Version = val
	}

	// Hub configuration
	if val := os.Getenv("OVIM_HUB_ENDPOINT"); val != "" {
		config.Hub.Endpoint = val
	}
	if val := os.Getenv("OVIM_HUB_PROTOCOL"); val != "" {
		config.Hub.Protocol = val
	}
	if val := os.Getenv("OVIM_HUB_TLS_ENABLED"); val != "" {
		config.Hub.TLSEnabled = val == "true"
	}
	if val := os.Getenv("OVIM_HUB_TLS_SKIP_VERIFY"); val != "" {
		config.Hub.TLSSkipVerify = val == "true"
	}
	if val := os.Getenv("OVIM_HUB_CERT_PATH"); val != "" {
		config.Hub.CertificatePath = val
	}
	if val := os.Getenv("OVIM_HUB_KEY_PATH"); val != "" {
		config.Hub.PrivateKeyPath = val
	}
	if val := os.Getenv("OVIM_HUB_CA_PATH"); val != "" {
		config.Hub.CAPath = val
	}

	// Parse duration values
	if val := os.Getenv("OVIM_HUB_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.Hub.Timeout = d
		}
	}

	// API configuration
	if val := os.Getenv("OVIM_API_ENABLED"); val != "" {
		config.API.Enabled = val == "true"
	}
	if val := os.Getenv("OVIM_API_ADDRESS"); val != "" {
		config.API.Address = val
	}
	if val := os.Getenv("OVIM_API_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.API.Port = port
		}
	}

	// Kubernetes configuration
	if val := os.Getenv("KUBECONFIG"); val != "" {
		config.Kubernetes.ConfigPath = val
		config.Kubernetes.InCluster = false
	}
	if val := os.Getenv("OVIM_K8S_IN_CLUSTER"); val != "" {
		config.Kubernetes.InCluster = val == "true"
	}

	// Feature flags
	if val := os.Getenv("OVIM_FEATURE_VDC_MANAGEMENT"); val != "" {
		config.Features.VDCManagement = val == "true"
	}
	if val := os.Getenv("OVIM_FEATURE_TEMPLATE_SYNC"); val != "" {
		config.Features.TemplateSync = val == "true"
	}
	if val := os.Getenv("OVIM_FEATURE_LOCAL_API"); val != "" {
		config.Features.LocalAPI = val == "true"
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig(config *SpokeConfig) error {
	if config.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if config.ClusterID == "" {
		return fmt.Errorf("cluster_id is required")
	}
	if config.ZoneID == "" {
		return fmt.Errorf("zone_id is required")
	}
	if config.Hub.Endpoint == "" {
		return fmt.Errorf("hub endpoint is required")
	}
	if config.Hub.Protocol != "https" {
		return fmt.Errorf("hub protocol must be 'https'")
	}
	if config.API.Port <= 0 || config.API.Port > 65535 {
		return fmt.Errorf("api port must be between 1 and 65535")
	}
	return nil
}

// getEnvOrDefault returns the environment variable value or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// generateAgentID generates a unique agent ID
func generateAgentID() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return fmt.Sprintf("ovim-spoke-%s", hostname)
}
