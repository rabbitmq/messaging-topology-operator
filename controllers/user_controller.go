/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/go-logr/logr"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
)

const userFinalizer = "deletion.finalizers.users.rabbitmq.com"

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=users/status,verbs=get;update;patch

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	user := &topologyv1alpha1.User{}
	if err := r.Get(ctx, req.NamespacedName, user); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	rabbitClient, err := rabbitholeClient(ctx, r.Client, user.Spec.RabbitmqClusterReference)
	if err != nil {
		logger.Error(err, "Failed to generate http rabbitClient")
		return reconcile.Result{}, err
	}

	// Check if the user has been marked for deletion
	if !user.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteUser(ctx, rabbitClient, user)
	}

	if err := r.addFinalizerIfNeeded(ctx, user); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(user.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal binding spec")
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	// TODO: Create a Secret object containing the credentials for the user
	if err := r.declareUser(ctx, rabbitClient, user); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *UserReconciler) declareUser(ctx context.Context, client *rabbithole.Client, user *topologyv1alpha1.User) error {
	logger := ctrl.LoggerFrom(ctx)

	// TODO: If a user has provided a Secret containing the desired password, use it instead here.
	password, err := internal.RandomEncodedString(24)
	if err != nil {
		msg := "failed to generate random password"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	decodedPassword, err := base64.StdEncoding.DecodeString(password)
	if err != nil {
		msg := "failed to decode random password"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	userSettings := internal.GenerateUserSettings(user.Spec, string(decodedPassword))

	if err := validateResponse(client.PutUser(user.Spec.Name, userSettings)); err != nil {
		msg := "failed to declare user"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "user", user.Spec.Name)
		return err
	}

	logger.Info("Successfully declared user", "user", user.Spec.Name)
	r.Recorder.Event(user, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared user")
	return nil
}

// addFinalizerIfNeeded adds a deletion finalizer if the User does not have one yet and is not marked for deletion
func (r *UserReconciler) addFinalizerIfNeeded(ctx context.Context, user *topologyv1alpha1.User) error {
	if user.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(user, userFinalizer) {
		controllerutil.AddFinalizer(user, userFinalizer)
		if err := r.Client.Update(ctx, user); err != nil {
			return err
		}
	}
	return nil
}

func (r *UserReconciler) deleteUser(ctx context.Context, client *rabbithole.Client, user *topologyv1alpha1.User) error {
	logger := ctrl.LoggerFrom(ctx)

	if err := validateResponse(client.DeleteUser(user.Spec.Name)); err != nil {
		msg := "failed to delete user"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "user", user.Spec.Name)
		return err
	}
	return r.removeFinalizer(ctx, user)
}

func (r *UserReconciler) removeFinalizer(ctx context.Context, user *topologyv1alpha1.User) error {
	controllerutil.RemoveFinalizer(user, userFinalizer)
	if err := r.Client.Update(ctx, user); err != nil {
		return err
	}
	return nil
}

func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1alpha1.User{}).
		Complete(r)
}
