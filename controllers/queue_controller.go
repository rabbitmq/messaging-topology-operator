/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const deletionFinalizer = "deletion.finalizers.queues.rabbitmq.com"

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
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get;update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

func (r *QueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	// fetched the q and return if q no longer exists
	q := &topology.Queue{}
	if err := r.Get(ctx, req.NamespacedName, q); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		msg := "failed to retrieve system trusted certs"
		r.Recorder.Event(q, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg)
		return ctrl.Result{}, err
	}

	// create rabbitmq http rabbitClient
	rabbitClient, err := r.RabbitmqClientFactory(ctx, r.Client, q.Spec.RabbitmqClusterReference, q.Namespace, systemCertPool)
	// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
	// the Cluster is temporarily down. Requeue until it comes back up.
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) && q.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	} else if err != nil && !errors.Is(err, internal.NoSuchRabbitmqClusterError) {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	// Check if the q has been marked for deletion
	if !q.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteQueue(ctx, rabbitClient, q)
	}

	if err := r.addFinalizerIfNeeded(ctx, q); err != nil {
		return ctrl.Result{}, err
	}

	queueSpec, err := json.Marshal(q.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(queueSpec))

	if err := r.declareQueue(ctx, rabbitClient, q); err != nil {
		// Set Condition 'Ready' to false with message
		q.Status.Conditions = []topology.Condition{topology.NotReady(err.Error())}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, q)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	q.Status.Conditions = []topology.Condition{topology.Ready()}
	q.Status.ObservedGeneration = q.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, q)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *QueueReconciler) declareQueue(ctx context.Context, client internal.RabbitMQClient, q *topology.Queue) error {
	logger := ctrl.LoggerFrom(ctx)

	queueSettings, err := internal.GenerateQueueSettings(q)
	if err != nil {
		msg := "failed to generate queue settings"
		r.Recorder.Event(q, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	if err := validateResponse(client.DeclareQueue(q.Spec.Vhost, q.Spec.Name, *queueSettings)); err != nil {
		msg := "failed to declare queue"
		r.Recorder.Event(q, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "queue", q.Spec.Name)
		return err
	}

	logger.Info("Successfully declared queue", "queue", q.Spec.Name)
	r.Recorder.Event(q, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared queue")
	return nil
}

// addFinalizerIfNeeded adds a deletion finalizer if the Queue does not have one yet and is not marked for deletion
func (r *QueueReconciler) addFinalizerIfNeeded(ctx context.Context, q *topology.Queue) error {
	if q.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(q, deletionFinalizer) {
		controllerutil.AddFinalizer(q, deletionFinalizer)
		if err := r.Client.Update(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// deletes queue from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
// queues could be deleted manually or gone because of AutoDelete
func (r *QueueReconciler) deleteQueue(ctx context.Context, client internal.RabbitMQClient, q *topology.Queue) error {
	logger := ctrl.LoggerFrom(ctx)

	if client == nil || reflect.ValueOf(client).IsNil() {
		logger.Info(noSuchRabbitDeletion, "queue", q.Name)
		r.Recorder.Event(q, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted queue")
		return r.removeFinalizer(ctx, q)
	}

	err := validateResponseForDeletion(client.DeleteQueue(q.Spec.Vhost, q.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find queue in rabbitmq server; already deleted", "queue", q.Spec.Name)
	} else if err != nil {
		msg := "failed to delete queue"
		r.Recorder.Event(q, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "queue", q.Spec.Name)
		return err
	}
	r.Recorder.Event(q, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted queue")
	return r.removeFinalizer(ctx, q)
}

func (r *QueueReconciler) removeFinalizer(ctx context.Context, q *topology.Queue) error {
	controllerutil.RemoveFinalizer(q, deletionFinalizer)
	if err := r.Client.Update(ctx, q); err != nil {
		return err
	}
	return nil
}

func (r *QueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Queue{}).
		Complete(r)
}
