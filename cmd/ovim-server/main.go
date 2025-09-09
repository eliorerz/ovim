package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/api"
	"github.com/eliorerz/ovim-updated/pkg/config"
	"github.com/eliorerz/ovim-updated/pkg/kubevirt"
	"github.com/eliorerz/ovim-updated/pkg/storage"
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

	// Initialize KubeVirt provisioner
	var provisioner kubevirt.VMProvisioner
	if cfg.Kubernetes.KubeVirt.Enabled {
		if cfg.Kubernetes.KubeVirt.UseMock {
			klog.Info("Initializing mock KubeVirt provisioner")
			provisioner = kubevirt.NewMockClient()
		} else {
			klog.Info("Initializing KubeVirt provisioner")
			kubeconfig := cfg.Kubernetes.ConfigPath
			if cfg.Kubernetes.InCluster {
				kubeconfig = "" // Use in-cluster config
			}
			provisioner, err = kubevirt.NewClient(kubeconfig, cfg.Kubernetes.KubeVirt.Namespace)
			if err != nil {
				klog.Warningf("Failed to initialize KubeVirt client: %v", err)
				klog.Info("Falling back to mock KubeVirt provisioner")
				provisioner = kubevirt.NewMockClient()
			} else {
				// Test connection
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := provisioner.CheckConnection(ctx); err != nil {
					klog.Warningf("KubeVirt connection test failed: %v", err)
					klog.Info("Falling back to mock KubeVirt provisioner")
					provisioner = kubevirt.NewMockClient()
				} else {
					klog.Info("KubeVirt connection successful")
				}
			}
		}
	} else {
		klog.Info("KubeVirt provisioning disabled, using mock provisioner")
		provisioner = kubevirt.NewMockClient()
	}

	server := api.NewServer(cfg, storageImpl, provisioner)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: server.Handler(),
	}

	go func() {
		klog.Infof("OVIM Backend Server listening on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	klog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		klog.Fatalf("Server forced to shutdown: %v", err)
	}

	klog.Info("Server exited")
}
