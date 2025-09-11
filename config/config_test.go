package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"OVIM_PORT",
		"OVIM_DATABASE_URL",
		"OVIM_JWT_SECRET",
		"OVIM_ENVIRONMENT",
		"OVIM_KUBECONFIG",
	}

	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}

	// Restore environment after test
	defer func() {
		for env, value := range originalEnv {
			if value != "" {
				os.Setenv(env, value)
			} else {
				os.Unsetenv(env)
			}
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name:    "Default configuration",
			envVars: map[string]string{},
			expected: &Config{
				Port:             "8080",
				DatabaseURL:      "postgres://ovim:ovim@localhost/ovim?sslmode=disable",
				JWTSecret:        "ovim-default-secret-change-in-production",
				Environment:      "development",
				KubernetesConfig: "",
			},
		},
		{
			name: "Custom configuration",
			envVars: map[string]string{
				"OVIM_PORT":         "9090",
				"OVIM_DATABASE_URL": "postgres://custom:custom@db:5432/ovim",
				"OVIM_JWT_SECRET":   "custom-secret-key",
				"OVIM_ENVIRONMENT":  "production",
				"OVIM_KUBECONFIG":   "/path/to/kubeconfig",
			},
			expected: &Config{
				Port:             "9090",
				DatabaseURL:      "postgres://custom:custom@db:5432/ovim",
				JWTSecret:        "custom-secret-key",
				Environment:      "production",
				KubernetesConfig: "/path/to/kubeconfig",
			},
		},
		{
			name: "Partial configuration override",
			envVars: map[string]string{
				"OVIM_PORT":        "3000",
				"OVIM_ENVIRONMENT": "staging",
			},
			expected: &Config{
				Port:             "3000",
				DatabaseURL:      "postgres://ovim:ovim@localhost/ovim?sslmode=disable",
				JWTSecret:        "ovim-default-secret-change-in-production",
				Environment:      "staging",
				KubernetesConfig: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for this test
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			config, err := Load()

			assert.NoError(t, err)
			assert.NotNil(t, config)
			assert.Equal(t, tt.expected.Port, config.Port)
			assert.Equal(t, tt.expected.DatabaseURL, config.DatabaseURL)
			assert.Equal(t, tt.expected.JWTSecret, config.JWTSecret)
			assert.Equal(t, tt.expected.Environment, config.Environment)
			assert.Equal(t, tt.expected.KubernetesConfig, config.KubernetesConfig)

			// Clean up environment variables
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "Environment variable exists",
			envKey:       "TEST_VAR",
			envValue:     "test_value",
			defaultValue: "default_value",
			expected:     "test_value",
		},
		{
			name:         "Environment variable doesn't exist",
			envKey:       "NONEXISTENT_VAR",
			envValue:     "",
			defaultValue: "default_value",
			expected:     "default_value",
		},
		{
			name:         "Environment variable is empty",
			envKey:       "EMPTY_VAR",
			envValue:     "",
			defaultValue: "default_value",
			expected:     "default_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before test
			os.Unsetenv(tt.envKey)

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
			}

			result := getEnv(tt.envKey, tt.defaultValue)
			assert.Equal(t, tt.expected, result)

			// Clean up after test
			os.Unsetenv(tt.envKey)
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "Valid integer environment variable",
			envKey:       "TEST_INT_VAR",
			envValue:     "123",
			defaultValue: 456,
			expected:     123,
		},
		{
			name:         "Invalid integer environment variable",
			envKey:       "TEST_INVALID_INT_VAR",
			envValue:     "not_a_number",
			defaultValue: 456,
			expected:     456,
		},
		{
			name:         "Environment variable doesn't exist",
			envKey:       "NONEXISTENT_INT_VAR",
			envValue:     "",
			defaultValue: 456,
			expected:     456,
		},
		{
			name:         "Zero value",
			envKey:       "TEST_ZERO_VAR",
			envValue:     "0",
			defaultValue: 456,
			expected:     0,
		},
		{
			name:         "Negative value",
			envKey:       "TEST_NEGATIVE_VAR",
			envValue:     "-123",
			defaultValue: 456,
			expected:     -123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before test
			os.Unsetenv(tt.envKey)

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
			}

			result := getEnvInt(tt.envKey, tt.defaultValue)
			assert.Equal(t, tt.expected, result)

			// Clean up after test
			os.Unsetenv(tt.envKey)
		})
	}
}

func TestConfigStructure(t *testing.T) {
	config := &Config{
		DatabaseURL:      "postgres://test:test@localhost/test",
		KubernetesConfig: "/test/kubeconfig",
		JWTSecret:        "test-secret",
		Port:             "8080",
		Environment:      "test",
	}

	assert.Equal(t, "postgres://test:test@localhost/test", config.DatabaseURL)
	assert.Equal(t, "/test/kubeconfig", config.KubernetesConfig)
	assert.Equal(t, "test-secret", config.JWTSecret)
	assert.Equal(t, "8080", config.Port)
	assert.Equal(t, "test", config.Environment)
}

func TestEnvironmentHelpers(t *testing.T) {
	// Test environment-specific behavior if needed
	tests := []struct {
		name        string
		environment string
		isDev       bool
		isProd      bool
	}{
		{
			name:        "Development environment",
			environment: "development",
			isDev:       true,
			isProd:      false,
		},
		{
			name:        "Production environment",
			environment: "production",
			isDev:       false,
			isProd:      true,
		},
		{
			name:        "Staging environment",
			environment: "staging",
			isDev:       false,
			isProd:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Environment: tt.environment}

			// These would be helper methods on Config if they existed
			isDev := config.Environment == "development"
			isProd := config.Environment == "production"

			assert.Equal(t, tt.isDev, isDev)
			assert.Equal(t, tt.isProd, isProd)
		})
	}
}
