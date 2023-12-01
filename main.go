/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"strconv"
	"strings"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/pkg/profiling"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
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
			controllers.KubernetesInternalDomainEnvVar, clusterDomain)
		return ""
	}

	if !strings.HasPrefix(clusterDomain, ".") {
		return fmt.Sprintf(".%s", clusterDomain)
	}

	return clusterDomain
}

func main() {
	var metricsAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1420#issuecomment-794525248
	klog.SetLogger(logger.WithName("messaging-topology-operator"))

	operatorNamespace := os.Getenv(controllers.OperatorNamespaceEnvVar)
	if operatorNamespace == "" {
		log.Info("unable to find operator namespace")
		os.Exit(1)
	}

	// If the environment variable is not set Getenv returns an empty string which ctrl.Options.Namespace takes to mean all namespaces should be watched
	operatorScopeNamespace := os.Getenv("OPERATOR_SCOPE_NAMESPACE")

	clusterDomain := sanitizeClusterDomainInput(os.Getenv(controllers.KubernetesInternalDomainEnvVar))

	usePlainHTTP := getBoolEnv(controllers.ConnectUsingPlainHTTPEnvVar)

	managerOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		LeaderElection:          true,
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

	if syncPeriod := os.Getenv(controllers.ControllerSyncPeriodEnvVar); syncPeriod != "" {
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

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Queue{},
		Log:                     ctrl.Log.WithName(controllers.QueueControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.QueueControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.QueueReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.QueueControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Exchange{},
		Log:                     ctrl.Log.WithName(controllers.ExchangeControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.ExchangeControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.ExchangeReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.ExchangeControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Binding{},
		Log:                     ctrl.Log.WithName(controllers.BindingControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.BindingControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.BindingReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.BindingControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.User{},
		Log:                     ctrl.Log.WithName(controllers.UserControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.UserControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		WatchTypes:              []client.Object{&corev1.Secret{}},
		ReconcileFunc:           &controllers.UserReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.UserControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Vhost{},
		Log:                     ctrl.Log.WithName(controllers.VhostControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.VhostControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.VhostReconciler{Client: mgr.GetClient()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.VhostControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Policy{},
		Log:                     ctrl.Log.WithName(controllers.PolicyControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.PolicyControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.PolicyReconciler{},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.PolicyControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Permission{},
		Log:                     ctrl.Log.WithName(controllers.PermissionControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.PermissionControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.PermissionReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.PermissionControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.SchemaReplication{},
		Log:                     ctrl.Log.WithName(controllers.SchemaReplicationControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.SchemaReplicationControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.SchemaReplicationReconciler{Client: mgr.GetClient()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.SchemaReplicationControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Federation{},
		Log:                     ctrl.Log.WithName(controllers.FederationControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.FederationControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.FederationReconciler{Client: mgr.GetClient()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.FederationControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.Shovel{},
		Log:                     ctrl.Log.WithName(controllers.ShovelControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.ShovelControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.ShovelReconciler{Client: mgr.GetClient()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.ShovelControllerName)
		os.Exit(1)
	}

	if err = (&controllers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &topology.TopicPermission{},
		Log:                     ctrl.Log.WithName(controllers.TopicPermissionControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllers.TopicPermissionControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &controllers.TopicPermissionReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		ConnectUsingPlainHTTP:   usePlainHTTP,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.TopicPermissionControllerName)
		os.Exit(1)
	}

	if err = (&controllers.SuperStreamReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.SuperStreamControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.SuperStreamControllerName),
		RabbitmqClientFactory: rabbitmqclient.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.SuperStreamControllerName)
		os.Exit(1)
	}

	if os.Getenv(controllers.EnableWebhooksEnvVar) != "false" {
		if err = (&topology.Binding{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Binding")
			os.Exit(1)
		}
		if err = (&topology.Queue{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Queue")
			os.Exit(1)
		}
		if err = (&topology.Exchange{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Exchange")
			os.Exit(1)
		}
		if err = (&topology.Vhost{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Vhost")
			os.Exit(1)
		}
		if err = (&topology.Policy{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Policy")
			os.Exit(1)
		}
		if err = (&topology.User{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "User")
			os.Exit(1)
		}
		if err = (&topology.Permission{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Permission")
			os.Exit(1)
		}
		if err = (&topology.SchemaReplication{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "SchemaReplication")
			os.Exit(1)
		}
		if err = (&topology.Federation{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Federation")
			os.Exit(1)
		}
		if err = (&topology.Shovel{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Shovel")
			os.Exit(1)
		}
		if err = (&topologyv1alpha1.SuperStream{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "SuperStream")
			os.Exit(1)
		}
		if err = (&topology.TopicPermission{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "TopicPermission")
			os.Exit(1)
		}
	}

	// +kubebuilder:scaffold:builder

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
