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

	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// ExchangeReconciler reconciles a Exchange object
type ExchangeReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	RabbitmqClientFactory   rabbitmqclient.Factory
	KubernetesClusterDomain string
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges/status,verbs=get;update;patch

func (r *ExchangeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	exchange := &topology.Exchange{}
	if err := r.Get(ctx, req.NamespacedName, exchange); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, exchange)
	if err != nil {
		return ctrl.Result{}, err
	}

	credsProvider, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, r.Client, exchange.Spec.RabbitmqClusterReference, exchange.Namespace, r.KubernetesClusterDomain)
	if err != nil {
		return handleRMQReferenceParseError(ctx, r.Client, r.Recorder, exchange, &exchange.Status.Conditions, err)
	}

	rabbitClient, err := r.RabbitmqClientFactory(credsProvider, tlsEnabled, systemCertPool)
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	if !exchange.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteExchange(ctx, rabbitClient, exchange)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, exchange); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(exchange.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.declareExchange(ctx, rabbitClient, exchange); err != nil {
		// Set Condition 'Ready' to false with message
		exchange.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), exchange.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, exchange)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", exchange.Status)
		}
		return ctrl.Result{}, err
	}

	exchange.Status.Conditions = []topology.Condition{topology.Ready(exchange.Status.Conditions)}
	exchange.Status.ObservedGeneration = exchange.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, exchange)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate, "status", exchange.Status)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *ExchangeReconciler) declareExchange(ctx context.Context, client rabbitmqclient.Client, exchange *topology.Exchange) error {
	logger := ctrl.LoggerFrom(ctx)

	settings, err := internal.GenerateExchangeSettings(exchange)
	if err != nil {
		msg := "failed to generate exchange settings"
		r.Recorder.Event(exchange, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	if err := validateResponse(client.DeclareExchange(exchange.Spec.Vhost, exchange.Spec.Name, *settings)); err != nil {
		msg := "failed to declare exchange"
		r.Recorder.Event(exchange, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "exchange", exchange.Spec.Name)
		return err
	}

	logger.Info("Successfully declared exchange", "exchange", exchange.Spec.Name)
	r.Recorder.Event(exchange, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared exchange")
	return nil
}

// deletes exchange from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *ExchangeReconciler) deleteExchange(ctx context.Context, client rabbitmqclient.Client, exchange *topology.Exchange) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.DeleteExchange(exchange.Spec.Vhost, exchange.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find exchange in rabbitmq server; already deleted", "exchange", exchange.Spec.Name)
	} else if err != nil {
		msg := "failed to delete exchange"
		r.Recorder.Event(exchange, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "exchange", exchange.Spec.Name)
		return err
	}
	r.Recorder.Event(exchange, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted exchange")
	return removeFinalizer(ctx, r.Client, exchange)
}

func (r *ExchangeReconciler) SetInternalDomainName(domainName string) {
	r.KubernetesClusterDomain = domainName
}

func (r *ExchangeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Exchange{}).
		Complete(r)
}
