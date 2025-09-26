package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/spoke"
	"github.com/eliorerz/ovim-updated/pkg/spoke/agent"
	"github.com/eliorerz/ovim-updated/pkg/spoke/api"
	"github.com/eliorerz/ovim-updated/pkg/spoke/config"
	"github.com/eliorerz/ovim-updated/pkg/spoke/controller"
	"github.com/eliorerz/ovim-updated/pkg/spoke/hub"
	"github.com/eliorerz/ovim-updated/pkg/spoke/processor"
	"github.com/eliorerz/ovim-updated/pkg/spoke/vdc"
	"github.com/eliorerz/ovim-updated/pkg/spoke/vm"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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

	// Initialize VDC manager and controller
	var vdcManager spoke.VDCManager
	var mgr ctrl.Manager
	var k8sClient client.Client
	if cfg.Kubernetes.InCluster || cfg.Kubernetes.ConfigPath != "" {
		logger.Info("Initializing VDC manager with Kubernetes client")

		// Create Kubernetes client configuration
		restConfig, err := rest.InClusterConfig()
		if err != nil {
			logger.Warn("Failed to create Kubernetes config for VDC manager", "error", err)
			vdcManager = nil
		} else {
			// Create standard Kubernetes client
			k8sClientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				logger.Warn("Failed to create Kubernetes clientset for VDC manager", "error", err)
				vdcManager = nil
			} else {
				// Create scheme with core Kubernetes and OVIM types
				scheme := runtime.NewScheme()
				// Add core Kubernetes types first
				if err := corev1.AddToScheme(scheme); err != nil {
					logger.Warn("Failed to add core v1 types to scheme", "error", err)
					vdcManager = nil
				} else if err := rbacv1.AddToScheme(scheme); err != nil {
					logger.Warn("Failed to add rbac v1 types to scheme", "error", err)
					vdcManager = nil
				} else if err := networkingv1.AddToScheme(scheme); err != nil {
					logger.Warn("Failed to add networking v1 types to scheme", "error", err)
					vdcManager = nil
				} else if err := ovimv1.AddToScheme(scheme); err != nil {
					logger.Warn("Failed to add OVIM types to scheme", "error", err)
					vdcManager = nil
				} else {
					// Create controller-runtime manager
					ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
					mgr, err = ctrl.NewManager(restConfig, ctrl.Options{
						Scheme: scheme,
					})
				}
				if err != nil {
					logger.Warn("Failed to create controller manager", "error", err)
					vdcManager = nil
				} else {
					// Create VDC manager
					vdcManager = vdc.NewManager(mgr.GetClient(), k8sClientset, logger, cfg)
					spokeAgent.SetVDCManager(vdcManager)

					// Store the k8s client for later use with the processor
					k8sClient = mgr.GetClient()

					// Setup VDC controller
					if err = (&controller.SpokeVDCReconciler{
						Client:     mgr.GetClient(),
						Scheme:     mgr.GetScheme(),
						K8sClient:  k8sClientset,
						HubClient:  hubClient,
						VDCManager: vdcManager,
						ClusterID:  cfg.ClusterID,
					}).SetupWithManager(mgr); err != nil {
						logger.Error("Failed to setup VDC controller", "error", err)
					} else {
						logger.Info("VDC controller setup successfully")
					}

					logger.Info("VDC manager initialized successfully")
				}
			}
		}
	} else {
		logger.Info("VDC manager disabled - no Kubernetes configuration")
		vdcManager = nil
	}

	// Initialize operation processor
	opProcessor := processor.NewProcessor(cfg, logger)
	opProcessor.SetVMManager(vmManager)
	opProcessor.SetVDCManager(vdcManager)
	if k8sClient != nil {
		opProcessor.SetK8sClient(k8sClient)
		logger.Info("Kubernetes client configured for operation processor")
	}
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

	// Start the agent and controller manager in goroutines
	errChan := make(chan error, 1)

	// Start the controller manager if available
	if mgr != nil {
		go func() {
			logger.Info("Starting VDC controller manager")
			if err := mgr.Start(ctx); err != nil {
				logger.Error("Controller manager failed", "error", err)
				errChan <- err
			}
		}()
	}

	// Start the agent in a goroutine
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
