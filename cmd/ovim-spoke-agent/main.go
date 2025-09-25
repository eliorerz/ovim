package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/agent"
	"github.com/eliorerz/ovim-updated/pkg/spoke/api"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
	"github.com/eliorerz/ovim-updated/pkg/spoke/hub"
	"github.com/eliorerz/ovim-updated/pkg/spoke/processor"
	"github.com/eliorerz/ovim-updated/pkg/spoke/vm"
)

const (
	// Version of the spoke agent
	Version = "v1.0.0"
)

func main() {
	// Parse command line flags
	var (
		_        = flag.String("config", "", "Path to configuration file") // TODO: implement config file loading
		logLevel = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		version  = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	// Print version and exit if requested
	if *version {
		println("OVIM Spoke Agent", Version)
		os.Exit(0)
	}

	// Setup logging
	logger := setupLogging(*logLevel)
	logger.Info("Starting OVIM Spoke Agent", "version", Version)

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Override version from build
	cfg.Version = Version

	logger.Info("Configuration loaded successfully",
		"agent_id", cfg.AgentID,
		"cluster_id", cfg.ClusterID,
		"zone_id", cfg.ZoneID,
		"hub_endpoint", cfg.Hub.Endpoint)

	// Create the main agent
	spokeAgent := agent.NewAgent(cfg, logger)

	// Initialize hub client
	hubClient := hub.NewHTTPClient(cfg, logger)
	spokeAgent.SetHubClient(hubClient)

	// Initialize VM manager (skip if Kubernetes not available)
	var vmManager spoke.VMManager
	if cfg.Kubernetes.InCluster || cfg.Kubernetes.ConfigPath != "" {
		vmManager, err = vm.NewManager(cfg, logger)
		if err != nil {
			logger.Warn("Failed to initialize VM manager, continuing without it", "error", err)
			vmManager = nil
		}
	} else {
		logger.Info("VM manager disabled - no Kubernetes configuration")
		vmManager = nil
	}
	if vmManager != nil {
		spokeAgent.SetVMManager(vmManager)
	}

	// Initialize operation processor
	opProcessor := processor.NewProcessor(cfg, logger)
	opProcessor.SetVMManager(vmManager)
	spokeAgent.SetOperationProcessor(opProcessor)

	// Initialize local API server
	localAPIServer := api.NewServer(cfg, logger)
	localAPIServer.SetHubClient(hubClient)
	localAPIServer.SetAgent(spokeAgent)
	spokeAgent.SetLocalAPIServer(localAPIServer)

	// TODO: Initialize other components
	// - VDC Manager (namespace and quota management)
	// - Metrics Collector (cluster resource monitoring)
	// - Health Reporter (health checking)
	// - Template Manager (VM template caching)

	logger.Info("Components initialized, starting agent")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the agent in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := spokeAgent.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", "signal", sig.String())
	case err := <-errChan:
		logger.Error("Agent startup failed", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown
	logger.Info("Shutting down agent...")
	cancel()

	if err := spokeAgent.Stop(); err != nil {
		logger.Error("Error during shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Agent shutdown completed")
}

// setupLogging configures structured logging
func setupLogging(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: level == "debug",
	}

	// Use JSON handler for structured logging
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}
