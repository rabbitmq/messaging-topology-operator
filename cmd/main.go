/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package main

import (
	"crypto/fips140"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/pkg/profiling"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/controller"
	webhookv1alpha1 "github.com/rabbitmq/messaging-topology-operator/internal/webhook/v1alpha1"
	webhookv1beta1 "github.com/rabbitmq/messaging-topology-operator/internal/webhook/v1beta1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
	log    = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = rabbitmqv1beta1.AddToScheme(scheme)

	_ = topology.AddToScheme(scheme)
	_ = topologyv1alpha1.AddToScheme(scheme)
	utilruntime.Must(rabbitmqcomv1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func sanitizeClusterDomainInput(clusterDomain string) string {
	if len(clusterDomain) == 0 {
		return ""
	}

	match, _ := regexp.MatchString("^\\.?[a-z]([-a-z0-9]*[a-z0-9])?(\\.[a-z]([-a-z0-9]*[a-z0-9])?)*$", clusterDomain) // Allow-list expression
	if !match {
		log.V(1).Info("Domain name value is invalid. Only alphanumeric characters, hyphens and dots are allowed.",
			controller.KubernetesInternalDomainEnvVar, clusterDomain)
		return ""
	}

	if !strings.HasPrefix(clusterDomain, ".") {
		return fmt.Sprintf(".%s", clusterDomain)
	}

	return clusterDomain
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1420#issuecomment-794525248
	klog.SetLogger(logger.WithName("messaging-topology-operator"))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		log.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	operatorNamespace, err := getOperatorNamespace()
	if err != nil {
		log.Error(err, "unable to find operator namespace")
		os.Exit(1)
	}

	// If the environment variable is not set Getenv returns an empty string which ctrl.Options.Namespace takes to mean all namespaces should be watched
	operatorScopeNamespace := os.Getenv("OPERATOR_SCOPE_NAMESPACE")

	clusterDomain := sanitizeClusterDomainInput(os.Getenv(controller.KubernetesInternalDomainEnvVar))

	usePlainHTTP := getBoolEnv(controller.ConnectUsingPlainHTTPEnvVar)

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		log.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		log.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	managerOpts := ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsServerOptions,
		WebhookServer:           webhookServer,
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: operatorNamespace,
		LeaderElectionID:        "messaging-topology-operator-leader-election",
	}

	if operatorScopeNamespace != "" {
		// https://github.com/kubernetes-sigs/controller-runtime/blob/main/designs/cache_options.md#only-cache-namespaced-objects-in-the-foo-and-bar-namespace
		// Also see https://github.com/rabbitmq/cluster-operator/blob/e2d413c102bc73d4b5e186d1d1b1f9bf728701e1/main.go#L114-L131
		if strings.Contains(operatorScopeNamespace, ",") {
			namespaces := strings.Split(operatorScopeNamespace, ",")
			managerOpts.Cache = cache.Options{
				DefaultNamespaces: make(map[string]cache.Config),
			}
			for _, namespace := range namespaces {
				managerOpts.Cache.DefaultNamespaces[namespace] = cache.Config{}
			}
			log.Info("manager configured to watch a list of namespaces", "namespaces", namespaces)
		} else {
			managerOpts.Cache = cache.Options{
				DefaultNamespaces: map[string]cache.Config{operatorScopeNamespace: {}},
			}
			log.Info("manager configured to watch a single namespace", "namespace", operatorScopeNamespace)
		}
	}

	if syncPeriod := os.Getenv(controller.ControllerSyncPeriodEnvVar); syncPeriod != "" {
		syncPeriodDuration, err := time.ParseDuration(syncPeriod)
		if err != nil {
			log.Error(err, "unable to parse provided sync period", "sync period", syncPeriod)
			os.Exit(1)
		}
		managerOpts.Cache.SyncPeriod = &syncPeriodDuration
		log.Info(fmt.Sprintf("sync period set; all resources will be reconciled every: %s", syncPeriodDuration))
	}

	if leaseDuration := getEnvInDuration("LEASE_DURATION"); leaseDuration != 0 {
		log.Info("manager configured with lease duration", "seconds", int(leaseDuration.Seconds()))
		managerOpts.LeaseDuration = &leaseDuration
	}

	if renewDeadline := getEnvInDuration("RENEW_DEADLINE"); renewDeadline != 0 {
		log.Info("manager configured with renew deadline", "seconds", int(renewDeadline.Seconds()))
		managerOpts.RenewDeadline = &renewDeadline
	}

	if retryPeriod := getEnvInDuration("RETRY_PERIOD"); retryPeriod != 0 {
		log.Info("manager configured with retry period", "seconds", int(retryPeriod.Seconds()))
		managerOpts.RetryPeriod = &retryPeriod
	}

	var maxConcurrentReconciles int
	if maxConcurrentReconcilesEnvValue := getIntEnv(controller.MaxConcurrentReconciles); maxConcurrentReconcilesEnvValue > 0 {
		maxConcurrentReconciles = maxConcurrentReconcilesEnvValue
		log.Info(fmt.Sprintf("maxConcurrentReconciles set to %d", maxConcurrentReconciles))
	}

	if enableDebugPprof, ok := os.LookupEnv("ENABLE_DEBUG_PPROF"); ok {
		pprofEnabled, err := strconv.ParseBool(enableDebugPprof)
		if err == nil && pprofEnabled {
			_, err = profiling.AddDebugPprofEndpoints(&managerOpts)
			if err != nil {
				log.Error(err, "unable to add debug endpoints to manager")
				os.Exit(1)
			}
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOpts)
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Queue{},
		Log:                     ctrl.Log.WithName(controller.QueueControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.QueueControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.QueueReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.QueueControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Exchange{},
		Log:                     ctrl.Log.WithName(controller.ExchangeControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.ExchangeControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.ExchangeReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.ExchangeControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Binding{},
		Log:                     ctrl.Log.WithName(controller.BindingControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.BindingControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.BindingReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.BindingControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.User{},
		Log:                     ctrl.Log.WithName(controller.UserControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.UserControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		WatchTypes:              []client.Object{},
		ReconcileFunc:           &controller.UserReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.UserControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Vhost{},
		Log:                     ctrl.Log.WithName(controller.VhostControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.VhostControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.VhostReconciler{Client: mgr.GetClient()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.VhostControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Policy{},
		Log:                     ctrl.Log.WithName(controller.PolicyControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.PolicyControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.PolicyReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.PolicyControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.OperatorPolicy{},
		Log:                     ctrl.Log.WithName(controller.OperatorPolicyControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.OperatorPolicyControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.OperatorPolicyReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.OperatorPolicyControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Permission{},
		Log:                     ctrl.Log.WithName(controller.PermissionControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.PermissionControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.PermissionReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.PermissionControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.SchemaReplication{},
		Log:                     ctrl.Log.WithName(controller.SchemaReplicationControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.SchemaReplicationControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.SchemaReplicationReconciler{Client: mgr.GetClient()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.SchemaReplicationControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Federation{},
		Log:                     ctrl.Log.WithName(controller.FederationControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.FederationControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.FederationReconciler{Client: mgr.GetClient()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.FederationControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Shovel{},
		Log:                     ctrl.Log.WithName(controller.ShovelControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.ShovelControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.ShovelReconciler{Client: mgr.GetClient()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.ShovelControllerName)
		os.Exit(1)
	}

	if err = (&controller.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.TopicPermission{},
		Log:                     ctrl.Log.WithName(controller.TopicPermissionControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.TopicPermissionControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controller.TopicPermissionReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.TopicPermissionControllerName)
		os.Exit(1)
	}

	if err = (&controller.SuperStreamReconciler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log.WithName(controller.SuperStreamControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controller.SuperStreamControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controller.SuperStreamControllerName)
		os.Exit(1)
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupBindingWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Binding")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupExchangeWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Exchange")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupFederationWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Federation")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupPermissionWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Permission")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupPolicyWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Policy")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupQueueWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Queue")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupSchemaReplicationWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "SchemaReplication")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupVhostWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Vhost")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1alpha1.SetupSuperStreamWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "SuperStream")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupShovelWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Shovel")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupUserWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "User")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupTopicPermissionWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "TopicPermission")
			os.Exit(1)
		}
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupOperatorPolicyWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "OperatorPolicy")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if fips140.Enabled() {
		log.Info("FIPS 140-3 mode enabled")
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getEnvInDuration(envName string) time.Duration {
	var durationInt int64
	if durationStr, ok := os.LookupEnv(envName); ok {
		var err error
		if durationInt, err = strconv.ParseInt(durationStr, 10, 64); err != nil {
			log.Error(err, fmt.Sprintf("unable to parse provided '%s'", envName))
			os.Exit(1)
		}
	}
	return time.Duration(durationInt) * time.Second
}

func getBoolEnv(envName string) bool {
	var boolVar bool
	if boolStr, ok := os.LookupEnv(envName); ok {
		var err error
		if boolVar, err = strconv.ParseBool(boolStr); err != nil {
			log.Error(err, fmt.Sprintf("unable to parse provided '%s'", envName))
			os.Exit(1)
		}
	}
	return boolVar
}

func getIntEnv(envName string) int {
	var intVar int
	if initStr, ok := os.LookupEnv(envName); ok {
		var err error
		if intVar, err = strconv.Atoi(initStr); err != nil {
			log.Error(err, fmt.Sprintf("unable to parse provided '%s'", envName))
			os.Exit(1)
		}
	}
	return intVar
}

// getOperatorNamespace returns the namespace the operator is running in.
// It first checks the OPERATOR_NAMESPACE environment variable for backward compatibility,
// then falls back to reading from the service account namespace file.
func getOperatorNamespace() (string, error) {
	// Check environment variable first (backward compatibility)
	if ns := os.Getenv(controller.OperatorNamespaceEnvVar); ns != "" {
		return ns, nil
	}

	// Fallback to service account namespace file (when running in-cluster)
	const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	nsBytes, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace from environment variable or service account file: %w", err)
	}

	return strings.TrimSpace(string(nsBytes)), nil
}
