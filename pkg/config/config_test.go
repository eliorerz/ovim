package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("DefaultConfiguration", func(t *testing.T) {
		// Clear environment variables
		clearEnvVars()

		cfg, err := Load("")
		require.NoError(t, err)

		// Test default values
		assert.Equal(t, DefaultPort, cfg.Server.Port)
		assert.Equal(t, DefaultEnvironment, cfg.Server.Environment)
		assert.Equal(t, DefaultReadTimeout, cfg.Server.ReadTimeout)
		assert.Equal(t, DefaultWriteTimeout, cfg.Server.WriteTimeout)
		assert.Equal(t, DefaultIdleTimeout, cfg.Server.IdleTimeout)

		// Test TLS defaults
		assert.True(t, cfg.Server.TLS.Enabled)
		assert.Equal(t, DefaultTLSPort, cfg.Server.TLS.Port)
		assert.True(t, cfg.Server.TLS.AutoGenerateCert)
		assert.Empty(t, cfg.Server.TLS.CertFile)
		assert.Empty(t, cfg.Server.TLS.KeyFile)

		// Test database defaults
		assert.Equal(t, DefaultDatabaseURL, cfg.Database.URL)
		assert.Equal(t, 25, cfg.Database.MaxOpenConns)
		assert.Equal(t, 25, cfg.Database.MaxIdleConns)
		assert.Equal(t, 5*time.Minute, cfg.Database.ConnMaxLifetime)

		// Test Kubernetes defaults
		assert.Empty(t, cfg.Kubernetes.ConfigPath)
		assert.False(t, cfg.Kubernetes.InCluster)
		assert.True(t, cfg.Kubernetes.KubeVirt.Enabled)
		assert.Equal(t, "default", cfg.Kubernetes.KubeVirt.Namespace)
		assert.True(t, cfg.Kubernetes.KubeVirt.UseMock)

		// Test OpenShift defaults
		assert.False(t, cfg.OpenShift.Enabled)
		assert.Empty(t, cfg.OpenShift.ConfigPath)
		assert.False(t, cfg.OpenShift.InCluster)
		assert.Equal(t, "openshift", cfg.OpenShift.TemplateNamespace)

		// Test Auth defaults
		assert.Equal(t, DefaultJWTSecret, cfg.Auth.JWTSecret)
		assert.Equal(t, 24*time.Hour, cfg.Auth.TokenDuration)
		assert.False(t, cfg.Auth.OIDC.Enabled)
		assert.Empty(t, cfg.Auth.OIDC.IssuerURL)
		assert.Empty(t, cfg.Auth.OIDC.ClientID)
		assert.Empty(t, cfg.Auth.OIDC.ClientSecret)
		assert.Empty(t, cfg.Auth.OIDC.RedirectURL)
		assert.Equal(t, []string{"openid", "profile", "email"}, cfg.Auth.OIDC.Scopes)

		// Test Logging defaults
		assert.Equal(t, "info", cfg.Logging.Level)
		assert.Equal(t, "json", cfg.Logging.Format)
	})

	t.Run("EnvironmentVariables", func(t *testing.T) {
		// Clear environment variables first
		clearEnvVars()

		// Set test environment variables
		os.Setenv(EnvPort, "9000")
		os.Setenv(EnvTLSEnabled, "false")
		os.Setenv(EnvTLSPort, "9443")
		os.Setenv(EnvTLSCertFile, "/custom/cert.pem")
		os.Setenv(EnvTLSKeyFile, "/custom/key.pem")
		os.Setenv(EnvTLSAutoGenerateCert, "false")
		os.Setenv(EnvDatabaseURL, "postgres://test:test@localhost/test")
		os.Setenv(EnvKubernetesConfig, "/custom/kubeconfig")
		os.Setenv(EnvKubernetesInCluster, "true")
		os.Setenv(EnvKubevirtEnabled, "false")
		os.Setenv(EnvKubevirtNamespace, "kubevirt-ns")
		os.Setenv(EnvJWTSecret, "custom-jwt-secret")
		os.Setenv(EnvEnvironment, "production")
		os.Setenv(EnvLogLevel, "debug")
		os.Setenv(EnvOIDCEnabled, "true")
		os.Setenv(EnvOIDCIssuerURL, "https://auth.example.com")
		os.Setenv(EnvOIDCClientID, "test-client")
		os.Setenv(EnvOIDCClientSecret, "test-secret")
		os.Setenv(EnvOIDCRedirectURL, "https://app.example.com/callback")
		os.Setenv(EnvOpenShiftEnabled, "true")
		os.Setenv(EnvOpenShiftConfig, "/custom/openshift-config")
		os.Setenv(EnvOpenShiftInCluster, "true")
		os.Setenv(EnvOpenShiftTemplateNamespace, "custom-templates")

		defer clearEnvVars()

		cfg, err := Load("")
		require.NoError(t, err)

		// Test server config
		assert.Equal(t, "9000", cfg.Server.Port)
		assert.Equal(t, "production", cfg.Server.Environment)

		// Test TLS config
		assert.False(t, cfg.Server.TLS.Enabled)
		assert.Equal(t, "9443", cfg.Server.TLS.Port)
		assert.Equal(t, "/custom/cert.pem", cfg.Server.TLS.CertFile)
		assert.Equal(t, "/custom/key.pem", cfg.Server.TLS.KeyFile)
		assert.False(t, cfg.Server.TLS.AutoGenerateCert)

		// Test database config
		assert.Equal(t, "postgres://test:test@localhost/test", cfg.Database.URL)

		// Test Kubernetes config
		assert.Equal(t, "/custom/kubeconfig", cfg.Kubernetes.ConfigPath)
		assert.True(t, cfg.Kubernetes.InCluster)
		assert.False(t, cfg.Kubernetes.KubeVirt.Enabled)
		assert.Equal(t, "kubevirt-ns", cfg.Kubernetes.KubeVirt.Namespace)
		assert.False(t, cfg.Kubernetes.KubeVirt.UseMock) // production environment

		// Test OpenShift config
		assert.True(t, cfg.OpenShift.Enabled)
		assert.Equal(t, "/custom/openshift-config", cfg.OpenShift.ConfigPath)
		assert.True(t, cfg.OpenShift.InCluster)
		assert.Equal(t, "custom-templates", cfg.OpenShift.TemplateNamespace)

		// Test Auth config
		assert.Equal(t, "custom-jwt-secret", cfg.Auth.JWTSecret)
		assert.True(t, cfg.Auth.OIDC.Enabled)
		assert.Equal(t, "https://auth.example.com", cfg.Auth.OIDC.IssuerURL)
		assert.Equal(t, "test-client", cfg.Auth.OIDC.ClientID)
		assert.Equal(t, "test-secret", cfg.Auth.OIDC.ClientSecret)
		assert.Equal(t, "https://app.example.com/callback", cfg.Auth.OIDC.RedirectURL)

		// Test Logging config
		assert.Equal(t, "debug", cfg.Logging.Level)
	})

	t.Run("ConfigFileLoading", func(t *testing.T) {
		clearEnvVars()

		// Test config file loading (currently returns nil as it's not implemented)
		cfg, err := Load("/nonexistent/config.yaml")
		require.NoError(t, err)
		assert.NotNil(t, cfg)
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("ValidConfiguration", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Port:        "8080",
				Environment: "development",
			},
			Auth: AuthConfig{
				JWTSecret: "valid-secret",
			},
		}

		err := cfg.validate()
		assert.NoError(t, err)
	})

	t.Run("EmptyPort", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Port: "",
			},
			Auth: AuthConfig{
				JWTSecret: "valid-secret",
			},
		}

		err := cfg.validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server port cannot be empty")
	})

	t.Run("EmptyJWTSecret", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Port: "8080",
			},
			Auth: AuthConfig{
				JWTSecret: "",
			},
		}

		err := cfg.validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "JWT secret cannot be empty")
	})

	t.Run("DefaultJWTSecretInProduction", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Port:        "8080",
				Environment: "production",
			},
			Auth: AuthConfig{
				JWTSecret: DefaultJWTSecret,
			},
		}

		err := cfg.validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "default JWT secret cannot be used in production")
	})

	t.Run("DefaultJWTSecretInDevelopment", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Port:        "8080",
				Environment: "development",
			},
			Auth: AuthConfig{
				JWTSecret: DefaultJWTSecret,
			},
		}

		err := cfg.validate()
		assert.NoError(t, err)
	})
}

func TestGetEnvString(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "EnvironmentVariableSet",
			key:          "TEST_STRING_VAR",
			defaultValue: "default",
			envValue:     "custom-value",
			expected:     "custom-value",
		},
		{
			name:         "EnvironmentVariableEmpty",
			key:          "TEST_STRING_VAR_EMPTY",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "EnvironmentVariableNotSet",
			key:          "TEST_STRING_VAR_NOT_SET",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvString(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		expected     int
	}{
		{
			name:         "ValidInteger",
			key:          "TEST_INT_VAR",
			defaultValue: 100,
			envValue:     "200",
			expected:     200,
		},
		{
			name:         "InvalidInteger",
			key:          "TEST_INT_VAR_INVALID",
			defaultValue: 100,
			envValue:     "not-a-number",
			expected:     100,
		},
		{
			name:         "EmptyValue",
			key:          "TEST_INT_VAR_EMPTY",
			defaultValue: 100,
			envValue:     "",
			expected:     100,
		},
		{
			name:         "ZeroValue",
			key:          "TEST_INT_VAR_ZERO",
			defaultValue: 100,
			envValue:     "0",
			expected:     0,
		},
		{
			name:         "NegativeValue",
			key:          "TEST_INT_VAR_NEGATIVE",
			defaultValue: 100,
			envValue:     "-50",
			expected:     -50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvInt(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue bool
		envValue     string
		expected     bool
	}{
		{
			name:         "TrueValue",
			key:          "TEST_BOOL_VAR_TRUE",
			defaultValue: false,
			envValue:     "true",
			expected:     true,
		},
		{
			name:         "FalseValue",
			key:          "TEST_BOOL_VAR_FALSE",
			defaultValue: true,
			envValue:     "false",
			expected:     false,
		},
		{
			name:         "OneValue",
			key:          "TEST_BOOL_VAR_ONE",
			defaultValue: false,
			envValue:     "1",
			expected:     true,
		},
		{
			name:         "ZeroValue",
			key:          "TEST_BOOL_VAR_ZERO",
			defaultValue: true,
			envValue:     "0",
			expected:     false,
		},
		{
			name:         "InvalidValue",
			key:          "TEST_BOOL_VAR_INVALID",
			defaultValue: true,
			envValue:     "maybe",
			expected:     true,
		},
		{
			name:         "EmptyValue",
			key:          "TEST_BOOL_VAR_EMPTY",
			defaultValue: true,
			envValue:     "",
			expected:     true,
		},
		{
			name:         "CaseInsensitive",
			key:          "TEST_BOOL_VAR_CASE",
			defaultValue: false,
			envValue:     "TRUE",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvBool(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	t.Run("NotImplemented", func(t *testing.T) {
		cfg := &Config{}
		err := loadFromFile(cfg, "/some/path.yaml")
		assert.NoError(t, err) // Currently returns nil as it's not implemented
	})
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "8080", DefaultPort)
	assert.Equal(t, "8443", DefaultTLSPort)
	assert.Equal(t, "ovim-default-secret-change-in-production", DefaultJWTSecret)
	assert.Equal(t, "development", DefaultEnvironment)
	assert.Equal(t, "postgres://ovim:ovim@localhost/ovim?sslmode=disable", DefaultDatabaseURL)
	assert.Equal(t, 30*time.Second, DefaultShutdownTimeout)
	assert.Equal(t, 10*time.Second, DefaultReadTimeout)
	assert.Equal(t, 10*time.Second, DefaultWriteTimeout)
	assert.Equal(t, 60*time.Second, DefaultIdleTimeout)

	// Test environment variable names
	assert.Equal(t, "OVIM_PORT", EnvPort)
	assert.Equal(t, "OVIM_TLS_ENABLED", EnvTLSEnabled)
	assert.Equal(t, "OVIM_DATABASE_URL", EnvDatabaseURL)
	assert.Equal(t, "OVIM_JWT_SECRET", EnvJWTSecret)
	assert.Equal(t, "OVIM_ENVIRONMENT", EnvEnvironment)
	assert.Equal(t, "OVIM_LOG_LEVEL", EnvLogLevel)
	assert.Equal(t, "OVIM_OIDC_ENABLED", EnvOIDCEnabled)
	assert.Equal(t, "OVIM_OPENSHIFT_ENABLED", EnvOpenShiftEnabled)
}

func TestConfigStructs(t *testing.T) {
	t.Run("ConfigStruct", func(t *testing.T) {
		cfg := Config{
			Server: ServerConfig{
				Port:        "8080",
				Environment: "test",
			},
			Database: DatabaseConfig{
				URL: "postgres://test",
			},
			Kubernetes: KubernetesConfig{
				InCluster: true,
			},
			OpenShift: OpenShiftConfig{
				Enabled: true,
			},
			Auth: AuthConfig{
				JWTSecret: "secret",
			},
			Logging: LoggingConfig{
				Level: "debug",
			},
		}

		assert.Equal(t, "8080", cfg.Server.Port)
		assert.Equal(t, "test", cfg.Server.Environment)
		assert.Equal(t, "postgres://test", cfg.Database.URL)
		assert.True(t, cfg.Kubernetes.InCluster)
		assert.True(t, cfg.OpenShift.Enabled)
		assert.Equal(t, "secret", cfg.Auth.JWTSecret)
		assert.Equal(t, "debug", cfg.Logging.Level)
	})

	t.Run("TLSConfig", func(t *testing.T) {
		tls := TLSConfig{
			Enabled:          true,
			Port:             "8443",
			CertFile:         "/cert.pem",
			KeyFile:          "/key.pem",
			AutoGenerateCert: false,
		}

		assert.True(t, tls.Enabled)
		assert.Equal(t, "8443", tls.Port)
		assert.Equal(t, "/cert.pem", tls.CertFile)
		assert.Equal(t, "/key.pem", tls.KeyFile)
		assert.False(t, tls.AutoGenerateCert)
	})

	t.Run("KubeVirtConfig", func(t *testing.T) {
		kv := KubeVirtConfig{
			Enabled:   true,
			Namespace: "kubevirt",
			UseMock:   false,
		}

		assert.True(t, kv.Enabled)
		assert.Equal(t, "kubevirt", kv.Namespace)
		assert.False(t, kv.UseMock)
	})

	t.Run("OIDCConfig", func(t *testing.T) {
		oidc := OIDCConfig{
			Enabled:      true,
			IssuerURL:    "https://auth.example.com",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURL:  "https://app.example.com/callback",
			Scopes:       []string{"openid", "profile"},
		}

		assert.True(t, oidc.Enabled)
		assert.Equal(t, "https://auth.example.com", oidc.IssuerURL)
		assert.Equal(t, "client-id", oidc.ClientID)
		assert.Equal(t, "client-secret", oidc.ClientSecret)
		assert.Equal(t, "https://app.example.com/callback", oidc.RedirectURL)
		assert.Equal(t, []string{"openid", "profile"}, oidc.Scopes)
	})
}

func TestFullConfigurationFlow(t *testing.T) {
	t.Run("ProductionEnvironment", func(t *testing.T) {
		clearEnvVars()

		// Set production-like environment
		os.Setenv(EnvEnvironment, "production")
		os.Setenv(EnvJWTSecret, "production-jwt-secret-very-secure")
		os.Setenv(EnvPort, "8080")
		os.Setenv(EnvDatabaseURL, "postgres://prod:secret@db.prod.com/ovim?sslmode=require")
		os.Setenv(EnvTLSEnabled, "true")
		os.Setenv(EnvTLSPort, "8443")
		os.Setenv(EnvKubernetesInCluster, "true")
		os.Setenv(EnvLogLevel, "warn")

		defer clearEnvVars()

		cfg, err := Load("")
		require.NoError(t, err)

		assert.Equal(t, "production", cfg.Server.Environment)
		assert.Equal(t, "production-jwt-secret-very-secure", cfg.Auth.JWTSecret)
		assert.Equal(t, "8080", cfg.Server.Port)
		assert.Contains(t, cfg.Database.URL, "db.prod.com")
		assert.True(t, cfg.Server.TLS.Enabled)
		assert.Equal(t, "8443", cfg.Server.TLS.Port)
		assert.True(t, cfg.Kubernetes.InCluster)
		assert.False(t, cfg.Kubernetes.KubeVirt.UseMock) // production should not use mock
		assert.Equal(t, "warn", cfg.Logging.Level)
	})

	t.Run("DevelopmentEnvironment", func(t *testing.T) {
		clearEnvVars()

		// Set development-like environment (minimal env vars)
		os.Setenv(EnvEnvironment, "development")
		os.Setenv(EnvPort, "3000")

		defer clearEnvVars()

		cfg, err := Load("")
		require.NoError(t, err)

		assert.Equal(t, "development", cfg.Server.Environment)
		assert.Equal(t, DefaultJWTSecret, cfg.Auth.JWTSecret) // Should allow default in dev
		assert.Equal(t, "3000", cfg.Server.Port)
		assert.Equal(t, DefaultDatabaseURL, cfg.Database.URL)
		assert.True(t, cfg.Kubernetes.KubeVirt.UseMock) // development should use mock
		assert.Equal(t, "info", cfg.Logging.Level)
	})
}

// clearEnvVars clears all OVIM-related environment variables
func clearEnvVars() {
	envVars := []string{
		EnvPort, EnvTLSEnabled, EnvTLSPort, EnvTLSCertFile, EnvTLSKeyFile, EnvTLSAutoGenerateCert,
		EnvDatabaseURL, EnvKubernetesConfig, EnvKubernetesInCluster, EnvKubevirtEnabled, EnvKubevirtNamespace,
		EnvJWTSecret, EnvEnvironment, EnvLogLevel, EnvOIDCEnabled, EnvOIDCIssuerURL, EnvOIDCClientID,
		EnvOIDCClientSecret, EnvOIDCRedirectURL, EnvOpenShiftEnabled, EnvOpenShiftConfig,
		EnvOpenShiftInCluster, EnvOpenShiftTemplateNamespace,
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}
}
