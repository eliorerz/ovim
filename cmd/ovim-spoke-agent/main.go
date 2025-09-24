package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/spoke/agent"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
	"github.com/eliorerz/ovim-updated/pkg/spoke/hub"
	"github.com/eliorerz/ovim-updated/pkg/spoke/processor"
	"github.com/eliorerz/ovim-updated/pkg/spoke/vdc"
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

	// Initialize Kubernetes clients
	k8sConfig, err := buildKubernetesConfig(cfg)
	if err != nil {
		logger.Error("Failed to build Kubernetes config", "error", err)
		os.Exit(1)
	}

	k8sClient, err := buildControllerRuntimeClient(k8sConfig)
	if err != nil {
		logger.Error("Failed to build controller-runtime client", "error", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		logger.Error("Failed to build Kubernetes clientset", "error", err)
		os.Exit(1)
	}

	// Initialize VM manager
	vmManager, err := vm.NewManager(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize VM manager", "error", err)
		os.Exit(1)
	}
	spokeAgent.SetVMManager(vmManager)

	// Initialize operation processor
	opProcessor := processor.NewProcessor(cfg, logger)
	opProcessor.SetVMManager(vmManager)
	spokeAgent.SetOperationProcessor(opProcessor)

	// Initialize VDC Manager (namespace and quota management) if enabled
	if cfg.Features.VDCManagement {
		vdcManager := vdc.NewManager(cfg, k8sClient, clientset, logger)
		opProcessor.SetVDCManager(vdcManager)
		spokeAgent.SetVDCManager(vdcManager)
		logger.Info("VDC Manager initialized")
	}

	// TODO: Initialize other components
	// - Metrics Collector (cluster resource monitoring)
	// - Health Reporter (health checking)
	// - Template Manager (VM template caching)
	// - Local API Server (debugging endpoints)

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

// buildKubernetesConfig builds Kubernetes client configuration
func buildKubernetesConfig(cfg *config.SpokeConfig) (*rest.Config, error) {
	var k8sConfig *rest.Config
	var err error

	if cfg.Kubernetes.InCluster {
		// Use in-cluster configuration
		k8sConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
	} else {
		// Use kubeconfig file
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubernetes.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create config from kubeconfig: %w", err)
		}
	}

	// Apply configuration overrides
	k8sConfig.QPS = cfg.Kubernetes.QPS
	k8sConfig.Burst = cfg.Kubernetes.Burst
	k8sConfig.Timeout = cfg.Kubernetes.Timeout

	return k8sConfig, nil
}

// buildControllerRuntimeClient builds a controller-runtime client with OVIM CRD support
func buildControllerRuntimeClient(k8sConfig *rest.Config) (client.Client, error) {
	// Create scheme with OVIM CRDs
	scheme := runtime.NewScheme()

	// Add standard Kubernetes types
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add Kubernetes types to scheme: %w", err)
	}

	// Add OVIM CRDs
	if err := ovimv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add OVIM CRDs to scheme: %w", err)
	}

	// Create client
	k8sClient, err := client.New(k8sConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create controller-runtime client: %w", err)
	}

	return k8sClient, nil
}
