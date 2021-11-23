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
	"github.com/rabbitmq/messaging-topology-operator/internal/leaderelection"
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
	"strings"
	"time"
)

// CompositeConsumerSetReconciler reconciles a RabbitMQ Super Stream, and any resources it comprises of
type CompositeConsumerSetReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=compositeconsumersets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=compositeconsumersets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams/status,verbs=get
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;create;list;update;delete;patch;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

func (r *CompositeConsumerSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	compositeConsumerSet := &topology.CompositeConsumerSet{}
	if err := r.Get(ctx, req.NamespacedName, compositeConsumerSet); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Start reconciling")

	referencedSuperStream := &topology.SuperStream{}
	if err := r.Get(ctx, types.NamespacedName{Name: compositeConsumerSet.Spec.SuperStreamReference.Name, Namespace: compositeConsumerSet.Namespace}, referencedSuperStream); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get SuperStream from reference: %w", err)
	}

	existingPodList := &corev1.PodList{}
	err := r.Client.List(ctx, existingPodList, client.InNamespace(compositeConsumerSet.Namespace), client.MatchingLabels(map[string]string{
		leaderelection.AnnotationSuperStream: referencedSuperStream.Name,
	}))
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("Existing Pods: %#v", existingPodList.Items))

	managedResourceBuilder := managedresource.Builder{
		ObjectOwner: compositeConsumerSet,
		Scheme:      r.Scheme,
	}

	var podBeingDeleted bool
	var podIndexToBeDeleted int
	var existingPods []corev1.Pod
	for i := 0; i < len(existingPodList.Items); i++ {
		if !existingPodList.Items[i].ObjectMeta.DeletionTimestamp.IsZero() {
			logger.Info(fmt.Sprintf("Pod %s is being deleted", existingPodList.Items[i].Name))
			podBeingDeleted = true
			podIndexToBeDeleted = i
		}
		existingPods = append(existingPods, existingPodList.Items[i])
	}
	if podBeingDeleted {
		// Remove the pod to be deleted from the existing pods list
		electablePods := make([]corev1.Pod, len(existingPods))
		copy(electablePods, existingPods)
		electablePods[podIndexToBeDeleted] = electablePods[len(electablePods)-1]
		electablePods = electablePods[:len(electablePods)-1]

		// Do a leader election based on the remaining pods
		electablePods = leaderelection.Elect(electablePods)

		finalizer := fmt.Sprintf("deletion.finalizers.%s/%ss", "rabbitmq.com", strings.ToLower("CompositeConsumerPod"))
		controllerutil.RemoveFinalizer(&existingPodList.Items[podIndexToBeDeleted], finalizer)

		podsToBeUpdated := append(electablePods, existingPodList.Items[podIndexToBeDeleted])

		for _, pod := range podsToBeUpdated {
			if err := r.Client.Update(ctx, &pod); err != nil {
				return ctrl.Result{}, nil
			}
		}
		return ctrl.Result{RequeueAfter: 1*time.Second}, nil
	}

	var builders []managedresource.ResourceBuilder
	for i := 0; i < compositeConsumerSet.Spec.Replicas; i++ {
		for j := 0; j < len(referencedSuperStream.Status.Partitions); j++ {
			builders = append(
				builders,
				managedResourceBuilder.CompositeConsumerPod(
					compositeConsumerSet.Spec.ConsumerPodSpec.Default,
					referencedSuperStream.Name,
					referencedSuperStream.Status.Partitions[j],
					i,
				),
			)
		}
	}

	var podBuilders = make(map[*corev1.Pod]managedresource.ResourceBuilder, compositeConsumerSet.Spec.Replicas*len(referencedSuperStream.Status.Partitions))
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

	leaderelection.Elect(pods)

	for resource, builder := range podBuilders {
		err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			_, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, resource, func() error {
				finalizer := fmt.Sprintf("deletion.finalizers.%s/%ss", "rabbitmq.com", strings.ToLower(builder.ResourceType()))
				if resource.GetDeletionTimestamp().IsZero() && !controllerutil.ContainsFinalizer(resource, finalizer) {
					controllerutil.AddFinalizer(resource, finalizer)
				}
				return builder.Update(resource)
			})
			return apiError
		})
		if err != nil {
			msg := fmt.Sprintf("FailedReconcile%s", builder.ResourceType())
			if writerErr := r.SetReconcileSuccess(ctx, compositeConsumerSet, topology.NotReady(msg, compositeConsumerSet.Status.Conditions)); writerErr != nil {
				logger.Error(writerErr, failedStatusUpdate)
			}
			return ctrl.Result{}, err
		}
	}

	if err := r.SetReconcileSuccess(ctx, compositeConsumerSet, topology.Ready(compositeConsumerSet.Status.Conditions)); err != nil {
		logger.Error(err, failedStatusUpdate)
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *CompositeConsumerSetReconciler) SetReconcileSuccess(ctx context.Context, compositeConsumerSet *topology.CompositeConsumerSet, condition topology.Condition) error {
	compositeConsumerSet.Status.Conditions = []topology.Condition{condition}
	compositeConsumerSet.Status.ObservedGeneration = compositeConsumerSet.GetGeneration()
	return clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, compositeConsumerSet)
	})
}

func (r *CompositeConsumerSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.CompositeConsumerSet{}).
		Owns(&topology.Exchange{}).
		Owns(&topology.Binding{}).
		Owns(&topology.Queue{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
