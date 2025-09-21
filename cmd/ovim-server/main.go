package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eliorerz/ovim-updated/pkg/acm"
	"github.com/eliorerz/ovim-updated/pkg/api"
	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	tlsutils "github.com/eliorerz/ovim-updated/pkg/tls"
	"github.com/eliorerz/ovim-updated/pkg/version"
)

const (
	defaultPort             = "8080"
	gracefulShutdownTimeout = 30 * time.Second
)

func main() {
	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("OVIM Server %s\n", version.Get().String())
		os.Exit(0)
	}

	klog.Info("Starting OVIM Backend Server")

	cfg, err := config.Load(*configPath)
	if err != nil {
		klog.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize storage based on configuration
	var storageImpl storage.Storage
	if cfg.Database.URL != "" && cfg.Database.URL != config.DefaultDatabaseURL {
		klog.Info("Initializing PostgreSQL storage")
		storageImpl, err = storage.NewPostgresStorage(cfg.Database.URL)
		if err != nil {
			klog.Warningf("Failed to initialize PostgreSQL storage: %v", err)
			klog.Info("Falling back to in-memory storage")
			storageImpl, err = storage.NewMemoryStorage()
			if err != nil {
				klog.Fatalf("Failed to initialize fallback storage: %v", err)
			}
		}
	} else {
		klog.Info("Initializing in-memory storage")
		storageImpl, err = storage.NewMemoryStorage()
		if err != nil {
			klog.Fatalf("Failed to initialize storage: %v", err)
		}
	}

	defer func() {
		if err := storageImpl.Close(); err != nil {
			klog.Errorf("Failed to close storage: %v", err)
		}
	}()

	// Initialize KubeVirt provisioner placeholder - will be initialized after Kubernetes client
	var provisioner kubevirt.VMProvisioner

	// Initialize Kubernetes client if enabled
	var k8sClient client.Client
	var kubernetesClient kubernetes.Interface
	var eventRecorder record.EventRecorder
	if cfg.Kubernetes.InCluster || cfg.Kubernetes.ConfigPath != "" {
		var restConfig *rest.Config
		var err error

		if cfg.Kubernetes.InCluster {
			klog.Info("Initializing in-cluster Kubernetes client")
			restConfig, err = rest.InClusterConfig()
		} else {
			klog.Infof("Initializing Kubernetes client with kubeconfig: %s", cfg.Kubernetes.ConfigPath)
			// TODO: Implement kubeconfig loading if needed
			// For now, try in-cluster as fallback
			restConfig, err = rest.InClusterConfig()
		}

		if err != nil {
			klog.Warningf("Failed to initialize Kubernetes config: %v", err)
			klog.Info("OVIM will run without Kubernetes integration (CRDs disabled)")
		} else {
			// Create a new scheme with OVIM CRDs registered
			clientScheme := runtime.NewScheme()

			// Add the default Kubernetes scheme
			if err := scheme.AddToScheme(clientScheme); err != nil {
				klog.Warningf("Failed to add Kubernetes scheme: %v", err)
				klog.Info("OVIM will run without Kubernetes integration (CRDs disabled)")
				k8sClient = nil
			} else {
				// Add OVIM CRDs to the scheme
				if err := ovimv1.AddToScheme(clientScheme); err != nil {
					klog.Warningf("Failed to add OVIM scheme: %v", err)
					klog.Info("OVIM will run without Kubernetes integration (CRDs disabled)")
					k8sClient = nil
				} else {
					// Create the controller-runtime client with our custom scheme
					k8sClient, err = client.New(restConfig, client.Options{
						Scheme: clientScheme,
					})
					if err != nil {
						klog.Warningf("Failed to create Kubernetes client: %v", err)
						klog.Info("OVIM will run without Kubernetes integration (CRDs disabled)")
						k8sClient = nil
					} else {
						klog.Info("Kubernetes client initialized successfully with OVIM CRDs")

						// Create event recorder if k8s client is available
						kubernetesClient, err = kubernetes.NewForConfig(restConfig)
						if err != nil {
							klog.Warningf("Failed to create Kubernetes clientset for event recorder: %v", err)
						} else {
							eventBroadcaster := record.NewBroadcaster()
							eventBroadcaster.StartLogging(klog.Infof)
							eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
								Interface: kubernetesClient.CoreV1().Events(""),
							})
							eventRecorder = eventBroadcaster.NewRecorder(clientScheme, corev1.EventSource{Component: "ovim-server"})
							klog.Info("Event recorder initialized successfully")
						}
					}
				}
			}
		}
	} else {
		klog.Info("Kubernetes integration disabled in configuration")
	}

	// Initialize KubeVirt provisioner now that we have Kubernetes client
	if cfg.Kubernetes.KubeVirt.Enabled {
		if cfg.Kubernetes.KubeVirt.UseMock {
			klog.Info("Initializing mock KubeVirt provisioner")
			provisioner = kubevirt.NewMockClient()
		} else if k8sClient != nil {
			klog.Info("Initializing KubeVirt provisioner with Kubernetes client")
			// Get the REST config from the Kubernetes client setup
			var restConfig *rest.Config
			var err error
			if cfg.Kubernetes.InCluster {
				restConfig, err = rest.InClusterConfig()
			} else {
				// TODO: Implement kubeconfig loading if needed
				restConfig, err = rest.InClusterConfig()
			}

			if err != nil {
				klog.Warningf("Failed to get Kubernetes config for KubeVirt: %v", err)
				klog.Info("Falling back to mock KubeVirt provisioner")
				provisioner = kubevirt.NewMockClient()
			} else {
				kubevirtClient, err := kubevirt.NewClient(restConfig, k8sClient)
				if err != nil {
					klog.Warningf("Failed to initialize KubeVirt client: %v", err)
					klog.Info("Falling back to mock KubeVirt provisioner")
					provisioner = kubevirt.NewMockClient()
				} else {
					// Test connection
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := kubevirtClient.CheckConnection(ctx); err != nil {
						klog.Warningf("KubeVirt connection test failed: %v", err)
						klog.Info("Falling back to mock KubeVirt provisioner")
						provisioner = kubevirt.NewMockClient()
					} else {
						klog.Info("KubeVirt connection successful")
						provisioner = kubevirtClient
					}
				}
			}
		} else {
			klog.Info("KubeVirt enabled but no Kubernetes client available, using mock provisioner")
			provisioner = kubevirt.NewMockClient()
		}
	} else {
		klog.Info("KubeVirt provisioning disabled, using mock provisioner")
		provisioner = kubevirt.NewMockClient()
	}

	// Initialize ACM zone sync if Kubernetes client is available
	var acmService *acm.Service
	if k8sClient != nil {
		// Create ACM sync configuration
		syncConfig := acm.SyncConfig{
			Enabled:                true,
			Interval:               5 * time.Minute,
			Namespace:              "open-cluster-management",
			AutoCreateZones:        true,
			ZonePrefix:             "",
			DefaultQuotaPercentage: 80,
			ExcludedClusters:       []string{},
			RequiredLabels:         map[string]string{},
		}

		// Create ACM client options (using in-cluster config)
		clientOpts := acm.ClientOptions{
			Namespace: "open-cluster-management",
			Timeout:   30 * time.Second,
		}

		// Initialize ACM service with proper options structure
		serviceOpts := acm.ServiceOptions{
			Storage:       storageImpl,
			Config:        syncConfig,
			ClientOptions: clientOpts,
		}

		acmService, err := acm.NewService(serviceOpts)
		if err != nil {
			klog.Errorf("Failed to initialize ACM service: %v", err)
			klog.Info("Continuing without ACM zone sync - zones will need to be managed manually")
		} else {
			// Start ACM zone sync in background
			ctx := context.Background()
			if err := acmService.Start(ctx); err != nil {
				klog.Errorf("Failed to start ACM zone sync: %v", err)
				klog.Info("ACM zone sync failed - this is expected if ACM is not installed or RBAC permissions are insufficient")
				klog.Info("Zones will need to be managed manually through the OVIM API")
				// Set acmService to nil so we don't try to stop it later
				acmService = nil
			} else {
				klog.Info("ACM zone sync started successfully - zones will be automatically discovered from managed clusters")
			}
		}
	} else {
		klog.Info("Kubernetes client not available, skipping ACM zone sync")
	}

	server := api.NewServer(cfg, storageImpl, provisioner, k8sClient, kubernetesClient, eventRecorder)
	handler := server.Handler()

	// Channel to collect server errors
	serverErrors := make(chan error, 1)

	var mainSrv *http.Server

	// Only run HTTPS server when TLS is enabled
	if cfg.Server.TLS.Enabled {
		// Set default certificate paths if not provided
		certFile := cfg.Server.TLS.CertFile
		keyFile := cfg.Server.TLS.KeyFile
		if certFile == "" {
			certFile = filepath.Join(".", "certs", "server.crt")
		}
		if keyFile == "" {
			keyFile = filepath.Join(".", "certs", "server.key")
		}

		// Ensure certificates exist
		if err := tlsutils.EnsureCertificates(certFile, keyFile, cfg.Server.TLS.AutoGenerateCert); err != nil {
			klog.Fatalf("Failed to ensure TLS certificates: %v", err)
		}

		// Load TLS configuration
		tlsConfig, err := tlsutils.LoadTLSConfig(certFile, keyFile)
		if err != nil {
			klog.Fatalf("Failed to load TLS configuration: %v", err)
		}

		mainSrv = &http.Server{
			Addr:         ":" + cfg.Server.TLS.Port,
			Handler:      handler,
			TLSConfig:    tlsConfig,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		}

		go func() {
			klog.Infof("OVIM Backend HTTPS Server listening on port %s", cfg.Server.TLS.Port)
			if err := mainSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				serverErrors <- fmt.Errorf("HTTPS server error: %w", err)
			}
		}()
	} else {
		// Fallback to HTTP only if TLS is disabled (for development/testing)
		klog.Warning("TLS is disabled - running HTTP server only (not recommended for production)")
		mainSrv = &http.Server{
			Addr:         ":" + cfg.Server.Port,
			Handler:      handler,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		}

		go func() {
			klog.Infof("OVIM Backend HTTP Server listening on port %s", cfg.Server.Port)
			if err := mainSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverErrors <- fmt.Errorf("HTTP server error: %w", err)
			}
		}()
	}

	// Wait for shutdown signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		klog.Fatalf("Server error: %v", err)
	case <-quit:
		klog.Info("Shutting down servers...")
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	// Shutdown ACM service
	if acmService != nil {
		klog.Info("Stopping ACM zone sync...")
		acmService.Stop()
	}

	// Shutdown main server
	if err := mainSrv.Shutdown(ctx); err != nil {
		klog.Errorf("Server forced to shutdown: %v", err)
	}

	klog.Info("Servers exited")
}
