package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.AgentID == "" {
		t.Error("AgentID should not be empty")
	}

	if cfg.Hub.Endpoint == "" {
		t.Error("Hub endpoint should not be empty")
	}

	if cfg.Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name         string
		modifyConfig func(*config.SpokeConfig)
		expectError  bool
	}{
		{
			name: "valid config",
			modifyConfig: func(cfg *config.SpokeConfig) {
				// Keep defaults
			},
			expectError: false,
		},
		{
			name: "empty agent ID",
			modifyConfig: func(cfg *config.SpokeConfig) {
				cfg.AgentID = ""
			},
			expectError: true,
		},
		{
			name: "empty cluster ID",
			modifyConfig: func(cfg *config.SpokeConfig) {
				cfg.ClusterID = ""
			},
			expectError: true,
		},
		{
			name: "invalid protocol",
			modifyConfig: func(cfg *config.SpokeConfig) {
				cfg.Hub.Protocol = "invalid"
			},
			expectError: true,
		},
		{
			name: "invalid port",
			modifyConfig: func(cfg *config.SpokeConfig) {
				cfg.API.Port = -1
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := config.LoadConfig()
			tt.modifyConfig(cfg)

			err := validateTestConfig(cfg)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestSetupLogging(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "invalid"}

	for _, level := range levels {
		logger := setupLogging(level)
		if logger == nil {
			t.Errorf("Logger should not be nil for level: %s", level)
		}
	}
}

func TestMainComponents(t *testing.T) {
	// Test that main components can be created without panicking
	_, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	logger := setupLogging("info")
	if logger == nil {
		t.Fatal("Logger should not be nil")
	}

	// Test context creation and cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		// Expected timeout
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have timed out")
	}
}

// validateTestConfig is a simplified version of config validation for testing
func validateTestConfig(cfg *config.SpokeConfig) error {
	if cfg.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if cfg.ClusterID == "" {
		return fmt.Errorf("cluster_id is required")
	}
	if cfg.Hub.Protocol != "http" && cfg.Hub.Protocol != "" {
		return fmt.Errorf("invalid protocol")
	}
	if cfg.API.Port <= 0 || cfg.API.Port > 65535 {
		return fmt.Errorf("invalid port")
	}
	return nil
}

func BenchmarkConfigLoad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := config.LoadConfig()
		if err != nil {
			b.Fatalf("Config load failed: %v", err)
		}
	}
}

func BenchmarkLoggingSetup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		logger := setupLogging("info")
		if logger == nil {
			b.Fatal("Logger should not be nil")
		}
	}
}
