package config

import (
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
)

// Config holds all configuration for the OVIM backend
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Auth       AuthConfig       `yaml:"auth"`
	Logging    LoggingConfig    `yaml:"logging"`
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

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret     string        `yaml:"jwtSecret"`
	TokenDuration time.Duration `yaml:"tokenDuration"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
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
				Enabled:          getEnvBool(EnvTLSEnabled, false),
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
		Auth: AuthConfig{
			JWTSecret:     getEnvString(EnvJWTSecret, DefaultJWTSecret),
			TokenDuration: 24 * time.Hour,
		},
		Logging: LoggingConfig{
			Level:  getEnvString(EnvLogLevel, "info"),
			Format: "json",
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
