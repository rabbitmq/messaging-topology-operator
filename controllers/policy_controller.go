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

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

const policyFinalizer = "deletion.finalizers.policies.rabbitmq.com"

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=policies/status,verbs=get;update;patch

func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	policy := &topologyv1alpha1.Policy{}

	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	rabbitClient, err := rabbitholeClient(ctx, r.Client, policy.Spec.RabbitmqClusterReference)

	// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
	// the Cluster is temporarily down. Requeue until it comes back up.
	if errors.Is(err, NoSuchRabbitmqClusterError) && policy.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Could not generate rabbitClient for non existant cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	} else if err != nil && !errors.Is(err, NoSuchRabbitmqClusterError) {
		logger.Error(err, "Failed to generate http rabbitClient")
		return reconcile.Result{}, err
	}

	if !policy.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deletePolicy(ctx, rabbitClient, policy)
	}

	if err := r.addFinalizerIfNeeded(ctx, policy); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(policy.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal policy spec")
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.putPolicy(ctx, rabbitClient, policy); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

// creates or updates a given policy using rabbithole client.PutPolicy
func (r *PolicyReconciler) putPolicy(ctx context.Context, client *rabbithole.Client, policy *topologyv1alpha1.Policy) error {
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

func (r *PolicyReconciler) addFinalizerIfNeeded(ctx context.Context, policy *topologyv1alpha1.Policy) error {
	if policy.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(policy, policyFinalizer) {
		controllerutil.AddFinalizer(policy, policyFinalizer)
		if err := r.Client.Update(ctx, policy); err != nil {
			return err
		}
	}
	return nil
}

// deletes policy from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *PolicyReconciler) deletePolicy(ctx context.Context, client *rabbithole.Client, policy *topologyv1alpha1.Policy) error {
	logger := ctrl.LoggerFrom(ctx)

	if client == nil {
		logger.Info("Not deleting policy as RabbitmqCluster is already deleted or down", "policy", policy.Name)
		return r.removeFinalizer(ctx, policy)
	}

	err := validateResponseForDeletion(client.DeletePolicy(policy.Spec.Vhost, policy.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find policy in rabbitmq server; already deleted", "policy", policy.Spec.Name)
	} else if err != nil {
		msg := "failed to delete policy"
		r.Recorder.Event(policy, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "policy", policy.Spec.Name)
		return err
	}
	return r.removeFinalizer(ctx, policy)
}

func (r *PolicyReconciler) removeFinalizer(ctx context.Context, policy *topologyv1alpha1.Policy) error {
	controllerutil.RemoveFinalizer(policy, policyFinalizer)
	if err := r.Client.Update(ctx, policy); err != nil {
		return err
	}
	return nil
}

func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1alpha1.Policy{}).
		Complete(r)
}
