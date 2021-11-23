/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/managedresource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CompositeConsumerReconciler reconciles a RabbitMQ Super Stream, and any resources it comprises of
type CompositeConsumerReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=compositeconsumers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=compositeconsumers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams/status,verbs=get
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;create;list;update;delete;patch;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

func (r *CompositeConsumerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	compositeConsumer := &topology.CompositeConsumer{}
	if err := r.Get(ctx, req.NamespacedName, compositeConsumer); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Start reconciling")

	referencedSuperStream := &topology.SuperStream{}
	if err := r.Get(ctx, types.NamespacedName{Name: compositeConsumer.Spec.SuperStreamReference.Name, Namespace: compositeConsumer.Namespace}, referencedSuperStream); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get SuperStream from reference: %w", err)
	}

	managedResourceBuilder := managedresource.Builder{
		ObjectOwner: compositeConsumer,
		Scheme:      r.Scheme,
	}

	existingPods, err := r.existingMatchingPods(ctx, compositeConsumer)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(existingPods) > 0 {
		r.logExistingMatchingPods(ctx, existingPods)
	}

	var builders []managedresource.ResourceBuilder
	for j := 0; j < len(referencedSuperStream.Status.Partitions); j++ {
		builders = append(
			builders,
			managedResourceBuilder.CompositeConsumerPod(
				compositeConsumer.Spec.ConsumerPodSpec.Default,
				referencedSuperStream.Name,
				referencedSuperStream.Status.Partitions[j],
			),
		)
	}

	var podBuilders = make(map[*corev1.Pod]managedresource.ResourceBuilder, len(referencedSuperStream.Status.Partitions))
	var pods []corev1.Pod
	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			return ctrl.Result{}, err
		}
		pod := resource.(*corev1.Pod)
		podBuilders[pod] = builder
		pods = append(pods, *pod)
	}

	for resource, builder := range podBuilders {
		err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			_, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, resource, func() error {
				return builder.Update(resource)
			})
			return apiError
		})
		if err != nil {
			msg := fmt.Sprintf("FailedReconcile%s", builder.ResourceType())
			if writerErr := r.SetReconcileSuccess(ctx, compositeConsumer, topology.NotReady(msg, compositeConsumer.Status.Conditions)); writerErr != nil {
				logger.Error(writerErr, failedStatusUpdate)
			}
			return ctrl.Result{}, err
		}
	}

	if err := r.SetReconcileSuccess(ctx, compositeConsumer, topology.Ready(compositeConsumer.Status.Conditions)); err != nil {
		logger.Error(err, failedStatusUpdate)
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *CompositeConsumerReconciler) SetReconcileSuccess(ctx context.Context, compositeConsumer *topology.CompositeConsumer, condition topology.Condition) error {
	compositeConsumer.Status.Conditions = []topology.Condition{condition}
	compositeConsumer.Status.ObservedGeneration = compositeConsumer.GetGeneration()
	return clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, compositeConsumer)
	})
}

func (r *CompositeConsumerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.CompositeConsumer{}).
		Owns(&topology.Exchange{}).
		Owns(&topology.Binding{}).
		Owns(&topology.Queue{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func (r *CompositeConsumerReconciler) existingMatchingPods(ctx context.Context, compositeConsumer *topology.CompositeConsumer) ([]corev1.Pod, error) {
	existingPodList := &corev1.PodList{}
	err := r.Client.List(ctx, existingPodList, client.InNamespace(compositeConsumer.Namespace), client.MatchingLabels(map[string]string{
		managedresource.AnnotationSuperStream: compositeConsumer.Spec.SuperStreamReference.Name,
	}))
	return existingPodList.Items, err
}
func (r *CompositeConsumerReconciler) logExistingMatchingPods(ctx context.Context, pods []corev1.Pod) {
	logger := ctrl.LoggerFrom(ctx)
	logString := "Existing pods: "
	for _, pod := range pods {
		logString += fmt.Sprintf("%s, ", pod.Name)
	}
	logger.Info(logString)
}
