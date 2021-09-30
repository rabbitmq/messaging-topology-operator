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
	"time"

	"github.com/go-logr/logr"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// QueueReconciler reconciles a RabbitMQ Queue
type QueueReconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	RabbitmqClientFactory internal.RabbitMQClientFactory
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

func (r *QueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	// fetched the queue and return if queue no longer exists
	queue := &topology.Queue{}
	if err := r.Get(ctx, req.NamespacedName, queue); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, queue)
	if err != nil {
		return ctrl.Result{}, err
	}

	rmq, svc, secret, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, queue.Spec.RabbitmqClusterReference, queue.Namespace)
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) && !queue.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info(noSuchRabbitDeletion, "queue", queue.Name)
		r.Recorder.Event(queue, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted queue")
		return reconcile.Result{}, removeFinalizer(ctx, r.Client, queue)
	}
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) {
		// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
		// the Cluster is temporarily down. Requeue until it comes back up.
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	}
	if errors.Is(err, internal.ResourceNotAllowedError) {
		logger.Info("Could not create queue resource: " + err.Error())
		queue.Status.Conditions = []topology.Condition{
			topology.NotReady(internal.ResourceNotAllowedError.Error(), queue.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, queue)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return reconcile.Result{}, nil
	}
	if err != nil {
		logger.Error(err, failedParseClusterRef)
		return reconcile.Result{}, err
	}

	rabbitClient, err := r.RabbitmqClientFactory(rmq, svc, secret, serviceDNSAddress(svc), systemCertPool)
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	// Check if the queue has been marked for deletion
	if !queue.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteQueue(ctx, rabbitClient, queue)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, queue); err != nil {
		return ctrl.Result{}, err
	}

	queueSpec, err := json.Marshal(queue.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(queueSpec))

	if err := r.declareQueue(ctx, rabbitClient, queue); err != nil {
		// Set Condition 'Ready' to false with message
		queue.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), queue.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, queue)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	queue.Status.Conditions = []topology.Condition{topology.Ready(queue.Status.Conditions)}
	queue.Status.ObservedGeneration = queue.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, queue)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *QueueReconciler) declareQueue(ctx context.Context, client internal.RabbitMQClient, queue *topology.Queue) error {
	logger := ctrl.LoggerFrom(ctx)

	queueSettings, err := internal.GenerateQueueSettings(queue)
	if err != nil {
		msg := "failed to generate queue settings"
		r.Recorder.Event(queue, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	if err := validateResponse(client.DeclareQueue(queue.Spec.Vhost, queue.Spec.Name, *queueSettings)); err != nil {
		msg := "failed to declare queue"
		r.Recorder.Event(queue, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "queue", queue.Spec.Name)
		return err
	}

	logger.Info("Successfully declared queue", "queue", queue.Spec.Name)
	r.Recorder.Event(queue, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared queue")
	return nil
}

// deletes queue from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
// queues could be deleted manually or gone because of AutoDelete
func (r *QueueReconciler) deleteQueue(ctx context.Context, client internal.RabbitMQClient, queue *topology.Queue) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.DeleteQueue(queue.Spec.Vhost, queue.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find queue in rabbitmq server; already deleted", "queue", queue.Spec.Name)
	} else if err != nil {
		msg := "failed to delete queue"
		r.Recorder.Event(queue, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "queue", queue.Spec.Name)
		return err
	}
	r.Recorder.Event(queue, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted queue")
	return removeFinalizer(ctx, r.Client, queue)
}

func (r *QueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Queue{}).
		Complete(r)
}
