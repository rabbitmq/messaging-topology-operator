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
	"errors"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

const bindingFinalizer = "deletion.finalizers.bindings.rabbitmq.com"

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

	if errors.Is(err, NoSuchRabbitmqClusterError) && binding.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Could not generate rabbitClient for non existant cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	} else if err != nil && !errors.Is(err, NoSuchRabbitmqClusterError) {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	if !binding.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteBinding(ctx, rabbitClient, binding)
	}

	if err := r.addFinalizerIfNeeded(ctx, binding); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(binding.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.declareBinding(ctx, rabbitClient, binding); err != nil {
		// Set Condition 'Ready' to false with message
		binding.Status.Conditions = []topologyv1alpha1.Condition{topologyv1alpha1.NotReady(err.Error())}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, binding)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	binding.Status.Conditions = []topologyv1alpha1.Condition{topologyv1alpha1.Ready()}
	binding.Status.ObservedGeneration = binding.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, binding)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *BindingReconciler) addFinalizerIfNeeded(ctx context.Context, binding *topologyv1alpha1.Binding) error {
	if binding.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(binding, bindingFinalizer) {
		controllerutil.AddFinalizer(binding, bindingFinalizer)
		if err := r.Client.Update(ctx, binding); err != nil {
			return err
		}
	}
	return nil
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

// deletes binding from rabbitmq server
// bindings have no name; server needs BindingInfo to delete them
// if server responds with '404' Not Found, it logs and does not requeue on error
// if binding arguments are provided, deletion is not supported due to problems with
// generating properties key
// TODO: to support deletion for bindings with arguments, the controller can list bindings with a given source and destination to find its properties key instead of generating it
func (r *BindingReconciler) deleteBinding(ctx context.Context, client *rabbithole.Client, binding *topologyv1alpha1.Binding) error {
	logger := ctrl.LoggerFrom(ctx)

	if client == nil {
		logger.Info(noSuchRabbitDeletion)
		return r.removeFinalizer(ctx, binding)
	}

	info, err := internal.GenerateBindingInfo(binding)
	if err != nil {
		msg := "failed to generate binding info"
		r.Recorder.Event(binding, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg)
		return err
	}

	propertiesKey := internal.GeneratePropertiesKey(binding)
	if propertiesKey == "" {
		msg := "deletion is not supported when binding arguments are provided; delete the binding manually"
		r.Recorder.Event(binding, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg)
		return err
	}
	info.PropertiesKey = propertiesKey

	err = validateResponseForDeletion(client.DeleteBinding(binding.Spec.Vhost, *info))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find binding in rabbitmq server; already deleted")
	} else if err != nil {
		msg := "failed to delete binding"
		r.Recorder.Event(binding, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg)
		return err
	}

	return r.removeFinalizer(ctx, binding)
}

func (r *BindingReconciler) removeFinalizer(ctx context.Context, binding *topologyv1alpha1.Binding) error {
	controllerutil.RemoveFinalizer(binding, bindingFinalizer)
	if err := r.Client.Update(ctx, binding); err != nil {
		return err
	}
	return nil
}

func (r *BindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1alpha1.Binding{}).
		Complete(r)
}
