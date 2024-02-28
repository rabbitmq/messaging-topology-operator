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

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	ctrl "sigs.k8s.io/controller-runtime"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

type QueueReconciler struct{}

func (r *QueueReconciler) DeclareFunc(_ context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	queue := obj.(*topology.Queue)
	queueSettings, err := internal.GenerateQueueSettings(queue)
	if err != nil {
		return fmt.Errorf("failed to generate queue settings: %w", err)
	}
	return validateResponse(client.DeclareQueue(queue.Spec.Vhost, queue.Spec.Name, *queueSettings))
}

// DeleteFunc deletes queue from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
// queues could be deleted manually or gone because of AutoDelete
func (r *QueueReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Deleting queues from ReconcilerFunc DeleteObj")

	queue := obj.(*topology.Queue)
	queueDeleteOptions, err := internal.GenerateQueueDeleteOptions(queue)
	if err != nil {
		return fmt.Errorf("failed to generate queue delete options: %w", err)
	}
	// Manage Quorum queue deletion if DeleteIfEmpty or DeleteIfUnused is true
	if queue.Spec.Type == "quorum" && (queue.Spec.DeleteIfEmpty || queue.Spec.DeleteIfUnused) {
		qInfo, err := client.GetQueue(queue.Spec.Vhost, queue.Spec.Name)
		if err != nil {
			return fmt.Errorf("cannot get %w queue information to verify queue is empty/unused: %s", err, queue.Spec.Name)
		}
		if qInfo.Messages > 0 && queue.Spec.DeleteIfEmpty {
			return fmt.Errorf("cannot delete queue %s because it has ready messages", queue.Spec.Name)
		}
		if qInfo.Consumers > 0 && queue.Spec.DeleteIfUnused {
			return fmt.Errorf("cannot delete queue %s because queue has consumers", queue.Spec.Name)
		}

	}

	errdel := validateResponseForDeletion(client.DeleteQueue(queue.Spec.Vhost, queue.Spec.Name, *queueDeleteOptions))
	if errors.Is(errdel, NotFound) {
		logger.Info("cannot find queue in rabbitmq server; already deleted", "queue", queue.Spec.Name)
	} else if errdel != nil {
		return errdel
	}
	return nil
}
