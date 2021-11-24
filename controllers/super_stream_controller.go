/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
	"time"
)

// SuperStreamReconciler reconciles a RabbitMQ Super Stream, and any resources it comprises of
type SuperStreamReconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	RabbitmqClientFactory internal.RabbitMQClientFactory
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=bindings,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

func (r *SuperStreamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	superStream := &topology.SuperStream{}
	if err := r.Get(ctx, req.NamespacedName, superStream); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	rmq, _, _, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, superStream.Spec.RabbitmqClusterReference, superStream.Namespace)
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) && !superStream.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info(noSuchRabbitDeletion, "superStream", superStream.Name)
		r.Recorder.Event(superStream, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted superStream")
		return reconcile.Result{}, removeFinalizer(ctx, r.Client, superStream)
	}
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) {
		// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
		// the Cluster is temporarily down. Requeue until it comes back up.
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	}
	if errors.Is(err, internal.ResourceNotAllowedError) {
		logger.Info("Could not create superStream resource: " + err.Error())
		superStream.Status.Conditions = []topology.Condition{
			topology.NotReady(internal.ResourceNotAllowedError.Error(), superStream.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, superStream)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return reconcile.Result{}, nil
	}
	if err != nil {
		logger.Error(err, failedParseClusterRef)
		return reconcile.Result{}, err
	}

	logger.Info("Start reconciling")

	if len(superStream.Spec.RoutingKeys) == 0 {
		if err := r.generateRoutingKeys(ctx, superStream); err != nil {
			return reconcile.Result{}, err
		}
	} else if len(superStream.Spec.RoutingKeys) != superStream.Spec.Partitions {
		err := fmt.Errorf(
			"expected number of routing keys (%d) to match number of partitions (%d)",
			len(superStream.Spec.RoutingKeys),
			superStream.Spec.Partitions,
		)
		msg := fmt.Sprintf("SuperStream %s failed to reconcile", superStream.Name)
		logger.Error(err, msg)
		if writerErr := r.SetReconcileSuccess(ctx, superStream, topology.NotReady(msg, superStream.Status.Conditions)); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return reconcile.Result{}, err
	}

	// Each SuperStream generates, for n partitions, 1 exchange, n streams and n bindings
	managedResourceBuilder := managedresource.Builder{
		ObjectOwner: superStream,
		Scheme:      r.Scheme,
	}

	rmqClusterRef := &topology.RabbitmqClusterReference{
		Name:      rmq.Name,
		Namespace: rmq.Namespace,
	}
	builders := []managedresource.ResourceBuilder{managedResourceBuilder.SuperStreamExchange(rmqClusterRef)}
	for index, routingKey := range superStream.Spec.RoutingKeys {
		builders = append(
			builders,
			managedResourceBuilder.SuperStreamPartition(routingKey, rmqClusterRef),
			managedResourceBuilder.SuperStreamBinding(index, routingKey, rmqClusterRef),
		)
	}

	var partitionQueueNames []string
	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			return ctrl.Result{}, err
		}

		err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			_, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, resource, func() error {
				return builder.Update(resource)
			})
			return apiError
		})
		if err != nil {
			msg := fmt.Sprintf("FailedReconcile%s", builder.ResourceType())
			if writerErr := r.SetReconcileSuccess(ctx, superStream, topology.NotReady(msg, superStream.Status.Conditions)); writerErr != nil {
				logger.Error(writerErr, failedStatusUpdate)
			}
			return ctrl.Result{}, err
		}

		if builder.ResourceType() == "Partition" {
			partition := resource.(*topology.Queue)
			partitionQueueNames = append(partitionQueueNames, partition.Spec.Name)
		}
	}

	superStream.Status.Partitions = partitionQueueNames
	if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, superStream)
	}); err != nil {
		logger.Error(err, failedStatusUpdate)
	}

	if err := r.SetReconcileSuccess(ctx, superStream, topology.Ready(superStream.Status.Conditions)); err != nil {
		logger.Error(err, failedStatusUpdate)
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *SuperStreamReconciler) generateRoutingKeys(ctx context.Context, superStream *topology.SuperStream) error {
	for i := 0; i < superStream.Spec.Partitions; i++ {
		superStream.Spec.RoutingKeys = append(superStream.Spec.RoutingKeys, strconv.Itoa(i))
	}
	if err := r.Update(ctx, superStream); err != nil {
		return err
	}
	return nil
}

func (r *SuperStreamReconciler) SetReconcileSuccess(ctx context.Context, superStream *topology.SuperStream, condition topology.Condition) error {
	superStream.Status.Conditions = []topology.Condition{condition}
	superStream.Status.ObservedGeneration = superStream.GetGeneration()
	return clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, superStream)
	})
}

func (r *SuperStreamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.SuperStream{}).
		Complete(r)
}
