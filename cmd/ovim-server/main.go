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

	"k8s.io/klog/v2"

	"github.com/eliorerz/ovim-updated/pkg/api"
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

	// Initialize KubeVirt provisioner
	var provisioner kubevirt.VMProvisioner
	if cfg.Kubernetes.KubeVirt.Enabled {
		if cfg.Kubernetes.KubeVirt.UseMock {
			klog.Info("Initializing mock KubeVirt provisioner")
			provisioner = kubevirt.NewMockClient()
		} else {
			klog.Info("Initializing KubeVirt provisioner")
			// TODO: Initialize proper Kubernetes client config and client
			// For now, use mock provisioner since we need rest.Config and client.Client
			klog.Info("KubeVirt integration available but using mock provisioner for now")
			provisioner = kubevirt.NewMockClient()
			err = nil
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

	// Shutdown main server
	if err := mainSrv.Shutdown(ctx); err != nil {
		klog.Errorf("Server forced to shutdown: %v", err)
	}

	klog.Info("Servers exited")
}
