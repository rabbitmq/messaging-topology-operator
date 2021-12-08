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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

// SuperStreamConsumerReconciler reconciles a RabbitMQ Super Stream, and any resources it comprises of
type SuperStreamConsumerReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreamconsumers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreamconsumers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams/status,verbs=get
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;create;list;update;delete;patch;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

func (r *SuperStreamConsumerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	superStreamConsumer := &topology.SuperStreamConsumer{}
	if err := r.Get(ctx, req.NamespacedName, superStreamConsumer); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Start reconciling")

	referencedSuperStream := &topology.SuperStream{}
	if err := r.Get(ctx, types.NamespacedName{Name: superStreamConsumer.Spec.SuperStreamReference.Name, Namespace: superStreamConsumer.Namespace}, referencedSuperStream); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get SuperStream from reference: %w", err)
	}
	if len(referencedSuperStream.Status.Partitions) != referencedSuperStream.Spec.Partitions {
		// The object is likely being reconciled, wait until all the partitions are created
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}

	managedResourceBuilder := managedresource.Builder{
		ObjectOwner: superStreamConsumer,
		Scheme:      r.Scheme,
	}

	existingPods, err := r.existingMatchingPods(ctx, superStreamConsumer)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(existingPods) > 0 {
		r.logExistingMatchingPods(ctx, existingPods)
	}

	var podBuilders = make(map[*corev1.Pod]managedresource.ResourceBuilder, len(referencedSuperStream.Status.Partitions))
	for _, partition := range referencedSuperStream.Status.Partitions {
		podSpec := r.getPartitionPodSpec(referencedSuperStream, partition, superStreamConsumer)
		if podSpec == nil {
			// There may be an existing Pod for this partition. We must delete the original Pod
			// before creating the new Pod with the new PodSpec.
			existingPods, err := r.getMatchingPods(ctx, referencedSuperStream.Namespace, map[string]string{
				managedresource.AnnotationSuperStream:          referencedSuperStream.Name,
				managedresource.AnnotationSuperStreamPartition: partition,
			})
			if err != nil {
				return ctrl.Result{}, err
			}
			for _, existingPod := range existingPods {
				err := r.deleteExistingPod(ctx, &existingPod, superStreamConsumer)
				if err != nil {
					return ctrl.Result{}, err
				}
				r.Recorder.Event(&existingPod, corev1.EventTypeNormal, "SuccessfulDelete", "Successfully deleted pod due to updated podSpec")
			}
			continue
		}

		builder := managedResourceBuilder.SuperStreamConsumerPod(
			*podSpec,
			referencedSuperStream.Name,
			partition,
		)

		resource, err := builder.Build()
		if err != nil {
			return ctrl.Result{}, err
		}
		pod := resource.(*corev1.Pod)
		podBuilders[pod] = builder
	}

	for pod, builder := range podBuilders {
		existingPod, err := r.existingActiveConsumerPod(ctx, referencedSuperStream.Namespace, map[string]string{
			managedresource.AnnotationSuperStream:          pod.Labels[managedresource.AnnotationSuperStream],
			managedresource.AnnotationSuperStreamPartition: pod.Labels[managedresource.AnnotationSuperStreamPartition],
		})
		if err != nil {
			msg := fmt.Sprintf("FailedReconcile%s", builder.ResourceType())
			if writerErr := r.SetReconcileSuccess(ctx, superStreamConsumer, topology.NotReady(msg, superStreamConsumer.Status.Conditions)); writerErr != nil {
				logger.Error(writerErr, failedStatusUpdate, "status", superStreamConsumer.Status)
			}
			return ctrl.Result{}, err
		}
		if podSpecHasNotChanged(existingPod, pod) {
			continue
		}
		if existingPod != nil {
			// There is an existing Pod for this partition, with a different PodSpec. We must delete the original Pod
			// before creating the new Pod with the new PodSpec.
			err := r.deleteExistingPod(ctx, existingPod, superStreamConsumer)
			if err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Event(existingPod, corev1.EventTypeNormal, "SuccessfulDelete", "Successfully deleted pod due to updated podSpec")
		}

		// Create the new Pod for this partition
		err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			_, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, pod, func() error {
				return builder.Update(pod)
			})
			return apiError
		})
		if err != nil {
			msg := fmt.Sprintf("FailedReconcile%s", builder.ResourceType())
			if writerErr := r.SetReconcileSuccess(ctx, superStreamConsumer, topology.NotReady(msg, superStreamConsumer.Status.Conditions)); writerErr != nil {
				logger.Error(writerErr, failedStatusUpdate, "status", superStreamConsumer.Status)
			}
			return ctrl.Result{}, err
		}
	}

	if err := r.SetReconcileSuccess(ctx, superStreamConsumer, topology.Ready(superStreamConsumer.Status.Conditions)); err != nil {
		logger.Error(err, failedStatusUpdate)
	}

	logger.Info("Finished reconciling")
	return ctrl.Result{}, nil
}

func (r *SuperStreamConsumerReconciler) deleteExistingPod(ctx context.Context, existingPod *corev1.Pod, superStreamConsumer *topology.SuperStreamConsumer) error {
	logger := ctrl.LoggerFrom(ctx)
	if err := r.Delete(ctx, existingPod); err != nil {
		if writerErr := r.SetReconcileSuccess(ctx, superStreamConsumer, topology.NotReady("FailedDeleteSuperStreamConsumerPod", superStreamConsumer.Status.Conditions)); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", superStreamConsumer.Status)
		}
		return err
	}
	return nil
}

func podSpecHasNotChanged(existingPod *corev1.Pod, pod *corev1.Pod) bool {
	return existingPod != nil && existingPod.Labels[managedresource.AnnotationConsumerPodSpecHash] == pod.Labels[managedresource.AnnotationConsumerPodSpecHash]
}

func (r *SuperStreamConsumerReconciler) getPartitionPodSpec(referencedSuperStream *topology.SuperStream, partition string, superStreamConsumer *topology.SuperStreamConsumer) *corev1.PodSpec {
	var podSpec *corev1.PodSpec = nil
	routingKey := managedresource.PartitionNameToRoutingKey(referencedSuperStream.Name, partition)
	if foundPodSpec, ok := superStreamConsumer.Spec.ConsumerPodSpec.PerRoutingKey[routingKey]; ok {
		podSpec = foundPodSpec
	} else {
		podSpec = superStreamConsumer.Spec.ConsumerPodSpec.Default
	}

	return podSpec
}

func (r *SuperStreamConsumerReconciler) SetReconcileSuccess(ctx context.Context, superStreamConsumer *topology.SuperStreamConsumer, condition topology.Condition) error {
	superStreamConsumer.Status.Conditions = []topology.Condition{condition}
	superStreamConsumer.Status.ObservedGeneration = superStreamConsumer.GetGeneration()
	return clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, superStreamConsumer)
	})
}

func (r *SuperStreamConsumerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &topology.SuperStreamConsumer{}, ".spec.superStreamReference.name", func(rawObj client.Object) []string {
		// Extract the SuperStream name from the SuperStreamConsumer Spec, if one is provided
		superStreamConsumer := rawObj.(*topology.SuperStreamConsumer)
		if superStreamConsumer.Spec.SuperStreamReference.Name == "" {
			return nil
		}
		return []string{superStreamConsumer.Spec.SuperStreamReference.Name}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.SuperStreamConsumer{}).
		Owns(&corev1.Pod{}).
		Watches(
			&source.Kind{Type: &topology.SuperStream{}},
			handler.EnqueueRequestsFromMapFunc(r.findConsumersForSuperStream),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

func (r *SuperStreamConsumerReconciler) findConsumersForSuperStream(superStream client.Object) []reconcile.Request {
	consumerList := &topology.SuperStreamConsumerList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(".spec.superStreamReference.name", superStream.GetName()),
		Namespace:     superStream.GetNamespace(),
	}
	err := r.List(context.Background(), consumerList, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(consumerList.Items))
	for i, item := range consumerList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

func (r *SuperStreamConsumerReconciler) getMatchingPods(ctx context.Context, namespace string, podLabels map[string]string) ([]corev1.Pod, error) {
	existingPodList := &corev1.PodList{}
	if err := r.Client.List(ctx, existingPodList, client.InNamespace(namespace), client.MatchingLabels(podLabels)); err != nil {
		return nil, err
	}

	return existingPodList.Items, nil
}

func (r *SuperStreamConsumerReconciler) existingActiveConsumerPod(ctx context.Context, namespace string, podLabels map[string]string) (*corev1.Pod, error) {
	existingPods, err := r.getMatchingPods(ctx, namespace, podLabels)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup matching existing pods: %w", err)
	}

	if len(existingPods) == 0 {
		return nil, nil
	}

	if len(existingPods) > 1 {
		var podNames []string
		for _, pod := range existingPods {
			podNames = append(podNames, pod.Name)
		}
		return nil, fmt.Errorf("expected to find 1 matching consumer pod, but found %d: %s", len(existingPods), podNames)
	}
	return &existingPods[0], nil
}
func (r *SuperStreamConsumerReconciler) existingMatchingPods(ctx context.Context, superStreamConsumer *topology.SuperStreamConsumer) ([]corev1.Pod, error) {
	existingPodList := &corev1.PodList{}
	err := r.Client.List(ctx, existingPodList, client.InNamespace(superStreamConsumer.Namespace), client.MatchingLabels(map[string]string{
		managedresource.AnnotationSuperStream: superStreamConsumer.Spec.SuperStreamReference.Name,
	}))
	return existingPodList.Items, err
}
func (r *SuperStreamConsumerReconciler) logExistingMatchingPods(ctx context.Context, pods []corev1.Pod) {
	logger := ctrl.LoggerFrom(ctx)
	logString := "Existing pods: "
	for _, pod := range pods {
		logString += fmt.Sprintf("%s, ", pod.Name)
	}
	logger.Info(logString)
}
