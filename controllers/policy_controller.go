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

	"github.com/rabbitmq/messaging-topology-operator/internal"
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

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	RabbitmqClientFactory internal.RabbitMQClientFactory
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=policies/status,verbs=get;update;patch

func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	policy := &topology.Policy{}

	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, policy)
	if err != nil {
		return ctrl.Result{}, err
	}

	rmq, svc, secret, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, policy.Spec.RabbitmqClusterReference, policy.Namespace)
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) && !policy.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info(noSuchRabbitDeletion, "policy", policy.Name)
		r.Recorder.Event(policy, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted policy")
		return reconcile.Result{}, removeFinalizer(ctx, r.Client, policy)
	}
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) {
		// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
		// the Cluster is temporarily down. Requeue until it comes back up.
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	}
	if errors.Is(err, internal.ResourceNotAllowedError) {
		logger.Info("Could not create policy resource: " + err.Error())
		policy.Status.Conditions = []topology.Condition{
			topology.NotReady(internal.ResourceNotAllowedError.Error(), policy.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, policy)
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

	if !policy.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deletePolicy(ctx, rabbitClient, policy)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, policy); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(policy.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.putPolicy(ctx, rabbitClient, policy); err != nil {
		// Set Condition 'Ready' to false with message
		policy.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), policy.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, policy)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	policy.Status.Conditions = []topology.Condition{topology.Ready(policy.Status.Conditions)}
	policy.Status.ObservedGeneration = policy.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, policy)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

// creates or updates a given policy using rabbithole client.PutPolicy
func (r *PolicyReconciler) putPolicy(ctx context.Context, client internal.RabbitMQClient, policy *topology.Policy) error {
	logger := ctrl.LoggerFrom(ctx)

	generatePolicy, err := internal.GeneratePolicy(policy)
	if err != nil {
		msg := "failed to generate Policy"
		r.Recorder.Event(policy, corev1.EventTypeWarning, "FailedCreateOrUpdate", msg)
		logger.Error(err, msg)
		return err
	}

	if err = validateResponse(client.PutPolicy(policy.Spec.Vhost, policy.Spec.Name, *generatePolicy)); err != nil {
		msg := "failed to create Policy"
		r.Recorder.Event(policy, corev1.EventTypeWarning, "FailedCreateOrUpdate", msg)
		logger.Error(err, msg, "policy", policy.Spec.Name)
		return err
	}
	logger.Info("Successfully created policy", "policy", policy.Spec.Name)
	r.Recorder.Event(policy, corev1.EventTypeNormal, "SuccessfulCreateOrUpdate", "Successfully created/updated policy")
	return nil
}

// deletes policy from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *PolicyReconciler) deletePolicy(ctx context.Context, client internal.RabbitMQClient, policy *topology.Policy) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.DeletePolicy(policy.Spec.Vhost, policy.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find policy in rabbitmq server; already deleted", "policy", policy.Spec.Name)
	} else if err != nil {
		msg := "failed to delete policy"
		r.Recorder.Event(policy, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "policy", policy.Spec.Name)
		return err
	}
	r.Recorder.Event(policy, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted policy")
	return removeFinalizer(ctx, r.Client, policy)
}

func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Policy{}).
		Complete(r)
}
