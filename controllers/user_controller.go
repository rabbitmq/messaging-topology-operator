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
	"fmt"

	"github.com/go-logr/logr"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var apiGVStr = topologyv1alpha1.GroupVersion.String()

const (
	userFinalizer = "deletion.finalizers.users.rabbitmq.com"
	ownerKey      = ".metadata.controller"
	ownerKind     = "User"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update

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

	if err := r.declareCredentials(ctx, user); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.setUserStatus(ctx, user); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.declareUser(ctx, rabbitClient, user); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *UserReconciler) declareCredentials(ctx context.Context, user *topologyv1alpha1.User) error {
	logger := ctrl.LoggerFrom(ctx)

	// TODO: If a user has provided a Secret containing the desired password, use it instead here.
	password, err := internal.RandomEncodedString(24)
	if err != nil {
		msg := "failed to generate random password"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	credentialSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.ObjectMeta.Name + "-user-credentials",
			Namespace: user.ObjectMeta.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte(base64.StdEncoding.EncodeToString([]byte(user.Spec.Name))),
			"password": []byte(password),
		},
	}

	var operationResult controllerutil.OperationResult
	err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		var apiError error
		operationResult, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, &credentialSecret, func() error {
			if err := controllerutil.SetControllerReference(user, &credentialSecret, r.Scheme); err != nil {
				return fmt.Errorf("failed setting controller reference: %v", err)
			}
			return nil
		})
		return apiError
	})
	if err != nil {
		msg := "failed to create/update credentials secret"
		r.Recorder.Event(&credentialSecret, corev1.EventTypeWarning, string(operationResult), msg)
		logger.Error(err, msg)
		return err
	}

	logger.Info("Successfully declared credentials secret", "user", credentialSecret.ObjectMeta.Name)
	r.Recorder.Event(&credentialSecret, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared user")
	return nil
}

func (r *UserReconciler) setUserStatus(ctx context.Context, user *topologyv1alpha1.User) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials := &corev1.LocalObjectReference{
		Name: user.Spec.Name + "-user-credentials",
	}
	user.Status.Credentials = credentials
	if err := r.Status().Update(ctx, user); err != nil {
		return err
	}
	logger.Info("Successfully updated secret status credentials", "user", user.Name, "secretRef", credentials)
	r.Recorder.Event(user, corev1.EventTypeNormal, "SuccessfulStatusUpdate", "Successfully updated user status")
	return nil
}

func (r *UserReconciler) declareUser(ctx context.Context, client *rabbithole.Client, user *topologyv1alpha1.User) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: user.Status.Credentials.Name, Namespace: user.Namespace}, credentials); err != nil {
		return err
	}

	userSettings, err := internal.GenerateUserSettings(credentials, user.Spec.Tags)
	if err != nil {
		msg := "failed to generate user settings from credential in status"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "user.status", user.Status)
		return err
	}

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
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Secret{}, ownerKey, addResourceToIndex); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1alpha1.User{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func addResourceToIndex(rawObj client.Object) []string {
	switch resourceObject := rawObj.(type) {
	case *corev1.Secret:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	default:
		return nil
	}
}

func validateAndGetOwner(owner *metav1.OwnerReference) []string {
	if owner == nil {
		return nil
	}
	if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
		return nil
	}
	return []string{owner.Name}
}
