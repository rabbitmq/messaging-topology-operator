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

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	// +kubebuilder:scaffold:imports
)

const (
	vhostControllerName      = "vhost-controller"
	queueControllerName      = "queue-controller"
	exchangeControllerName   = "exchange-controller"
	bindingControllerName    = "binding-controller"
	userControllerName       = "user-controller"
	policyControllerName     = "policy-controller"
	permissionControllerName = "permission-controller"
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
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     true,
		LeaderElectionID:   "messaging-topology-operator-leader-election",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.QueueReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName("controllers").WithName("Queue"),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(queueControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", queueControllerName)
		os.Exit(1)
	}
	if err = (&controllers.ExchangeReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName("controllers").WithName("Exchange"),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(exchangeControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", exchangeControllerName)
		os.Exit(1)
	}
	if err = (&controllers.BindingReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName("controllers").WithName("Binding"),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(bindingControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", bindingControllerName)
		os.Exit(1)
	}
	if err = (&controllers.UserReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName("controllers").WithName("User"),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(userControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", userControllerName)
		os.Exit(1)
	}
	if err = (&controllers.VhostReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName("controllers").WithName("Vhost"),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(vhostControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", vhostControllerName)
		os.Exit(1)
	}
	if err = (&controllers.PolicyReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName("controllers").WithName("Policy"),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(policyControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", policyControllerName)
		os.Exit(1)
	}
	if err = (&controllers.PermissionReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName("controllers").WithName("Permission"),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(permissionControllerName),
		RabbitmqClientFactory: internal.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", permissionControllerName)
		os.Exit(1)
	}

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
	// +kubebuilder:scaffold:builder

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
