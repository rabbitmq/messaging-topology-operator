/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"github.com/rabbitmq/messaging-topology-operator/internal"
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
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		log.Info("unable to find operator namespace")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		LeaderElection:          true,
		LeaderElectionNamespace: operatorNamespace,
		LeaderElectionID:        "messaging-topology-operator-leader-election",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.QueueReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.QueueControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.QueueControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.QueueControllerName)
		os.Exit(1)
	}
	if err = (&controllers.ExchangeReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.ExchangeControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.ExchangeControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.ExchangeControllerName)
		os.Exit(1)
	}
	if err = (&controllers.BindingReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.BindingControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.BindingControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.BindingControllerName)
		os.Exit(1)
	}
	if err = (&controllers.UserReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.UserControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.UserControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.UserControllerName)
		os.Exit(1)
	}
	if err = (&controllers.VhostReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.VhostControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.VhostControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.VhostControllerName)
		os.Exit(1)
	}
	if err = (&controllers.PolicyReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.PolicyControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.PolicyControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.PolicyControllerName)
		os.Exit(1)
	}
	if err = (&controllers.PermissionReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.PermissionControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.PermissionControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.PermissionControllerName)
		os.Exit(1)
	}
	if err = (&controllers.SchemaReplicationReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.SchemaReplicationControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.SchemaReplicationControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.SchemaReplicationControllerName)
		os.Exit(1)
	}
	if err = (&controllers.FederationReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.FederationControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.FederationControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.FederationControllerName)
		os.Exit(1)
	}
	if err = (&controllers.ShovelReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(controllers.ShovelControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(controllers.ShovelControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", controllers.ShovelControllerName)
		os.Exit(1)
	}
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
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
	}
	// +kubebuilder:scaffold:builder

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
