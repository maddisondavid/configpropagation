package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	configv1alpha1 "configpropagation/src/api/v1alpha1"
	"configpropagation/src/controllers/configpropagation"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(configv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool
	var webhookPort int
	enableWebhooks := defaultEnableWebhooks()

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the health probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "Webhook server port.")
	flag.BoolVar(&enableWebhooks, "enable-webhooks", enableWebhooks, "Enable Kubernetes admission webhooks.")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "configpropagation-operator",
		WebhookServer:          crwebhook.NewServer(crwebhook.Options{Port: webhookPort}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := configpropagation.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ConfigPropagation")
		os.Exit(1)
	}

	if enableWebhooks {
		if err := (&configv1alpha1.ConfigPropagation{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ConfigPropagation")
			os.Exit(1)
		}
	}

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

func defaultEnableWebhooks() bool {
	env := os.Getenv("ENABLE_WEBHOOKS")
	if env == "" {
		return true
	}
	parsed, err := strconv.ParseBool(env)
	if err != nil {
		setupLog.Error(fmt.Errorf("invalid ENABLE_WEBHOOKS value: %w", err), "defaulting webhooks to enabled")
		return true
	}
	return parsed
}
