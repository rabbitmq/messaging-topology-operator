/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"
	"encoding/json"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

// BindingReconciler reconciles a Binding object
type BindingReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=bindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=bindings/status,verbs=get;update;patch

func (r *BindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	binding := &topologyv1alpha1.Binding{}
	if err := r.Get(ctx, req.NamespacedName, binding); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	rabbitClient, err := rabbitholeClient(ctx, r.Client, binding.Spec.RabbitmqClusterReference)
	if err != nil {
		logger.Error(err, "Failed to generate http rabbitClient")
		return reconcile.Result{}, err
	}

	spec, err := json.Marshal(binding.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal binding spec")
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.declareBinding(ctx, rabbitClient, binding); err != nil {
		// Set Condition 'Ready' to false with message
		binding.Status.Conditions = []topologyv1alpha1.Condition{topologyv1alpha1.NotReady(err.Error())}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, binding)
		}); writerErr != nil {
			logger.Error(writerErr, failedConditionsUpdateMsg)
		}
		return ctrl.Result{}, err
	}

	binding.Status.Conditions = []topologyv1alpha1.Condition{topologyv1alpha1.Ready()}
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, binding)
	}); writerErr != nil {
		logger.Error(writerErr, failedConditionsUpdateMsg)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *BindingReconciler) declareBinding(ctx context.Context, client *rabbithole.Client, binding *topologyv1alpha1.Binding) error {
	logger := ctrl.LoggerFrom(ctx)

	info, err := internal.GenerateBindingInfo(binding)
	if err != nil {
		msg := "failed to generate binding info"
		r.Recorder.Event(binding, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	if err := validateResponse(client.DeclareBinding(binding.Spec.Vhost, *info)); err != nil {
		msg := "failed to declare binding"
		r.Recorder.Event(binding, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	logger.Info("Successfully declared binding")
	r.Recorder.Event(binding, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared binding")
	return nil
}

func (r *BindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1alpha1.Binding{}).
		Complete(r)
}
