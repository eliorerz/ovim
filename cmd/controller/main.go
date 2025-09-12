package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/eliorerz/ovim-updated/controllers"
	ovimv1 "github.com/eliorerz/ovim-updated/pkg/api/v1"
	"github.com/eliorerz/ovim-updated/pkg/storage"
	ovimwebhook "github.com/eliorerz/ovim-updated/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ovimv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var webhookAddr string
	var webhookCertDir string
	var enableWebhook bool
	var dbURL string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&webhookAddr, "webhook-bind-address", ":9443", "The address the webhook endpoint binds to.")
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs", "The directory that contains the webhook server key and certificate.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableWebhook, "enable-webhook", false, "Enable admission webhook server.")
	flag.StringVar(&dbURL, "database-url", "", "Database connection URL (optional)")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	webhookServer := webhook.NewServer(webhook.Options{
		Port:    9443,
		CertDir: webhookCertDir,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ovim-controller-leader",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize database storage if URL provided
	var store storage.Storage
	if dbURL != "" {
		setupLog.Info("Initializing database connection", "url", dbURL)
		// Note: Database integration will be implemented later
		setupLog.Info("Database integration placeholder - will be implemented in Phase 3")
		store = nil
	} else {
		setupLog.Info("No database URL provided, running without database integration")
	}

	// Set up Organization Controller
	if err = (&controllers.OrganizationReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Storage: store,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Organization")
		os.Exit(1)
	}

	// Set up VirtualDataCenter Controller
	if err = (&controllers.VirtualDataCenterReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Storage: store,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VirtualDataCenter")
		os.Exit(1)
	}

	// Set up RBAC Sync Controller
	if err = (&controllers.RBACReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RBACSync")
		os.Exit(1)
	}

	// Set up Metrics Collection Controller
	if err = (&controllers.MetricsReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metrics")
		os.Exit(1)
	}

	// Set up VM Controller
	if err = (&controllers.VMReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Storage: store,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VM")
		os.Exit(1)
	}

	// Set up webhook if enabled
	if enableWebhook {
		setupLog.Info("Setting up webhook")
		if err = (&ovimwebhook.WorkloadWebhook{
			Client: mgr.GetClient(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "WorkloadWebhook")
			os.Exit(1)
		}
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
