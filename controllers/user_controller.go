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
	"fmt"

	"github.com/go-logr/logr"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var apiGVStr = topology.GroupVersion.String()

const (
	ownerKey  = ".metadata.controller"
	ownerKind = "User"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	RabbitmqClientFactory   rabbitmqclient.Factory
	KubernetesClusterDomain string
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=users/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	user := &topology.User{}
	if err := r.Get(ctx, req.NamespacedName, user); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, user)
	if err != nil {
		return ctrl.Result{}, err
	}

	credsProvider, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, r.Client, user.Spec.RabbitmqClusterReference, user.Namespace, r.KubernetesClusterDomain)
	if err != nil {
		return handleRMQReferenceParseError(ctx, r.Client, r.Recorder, user, &user.Status.Conditions, err)
	}

	rabbitClient, err := r.RabbitmqClientFactory(credsProvider, tlsEnabled, systemCertPool)
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	// Check if the user has been marked for deletion
	if !user.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		if user.Status.Username != "" {
			return ctrl.Result{}, r.deleteUser(ctx, rabbitClient, user)
		} else {
			// Old function, kept for compatiblity
			return ctrl.Result{}, r.deleteUserFromSecret(ctx, rabbitClient, user)
		}
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, user); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(user.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if user.Status.Credentials == nil || user.Status.Username == "" {
		username := ""
		if user.Status.Credentials != nil && user.Status.Username == "" {
			// Only run once for migration to set user.Status.Username on exsisting resources
			credentials, err := r.getUserCredentials(ctx, user)
			if err != nil {
				return ctrl.Result{}, err
			}
			username = string(credentials.Data["username"])
		} else {
			logger.Info("User does not yet have a Credentials Secret; generating", "user", user.Name)
			if username, err = r.declareCredentials(ctx, user); err != nil {
				return ctrl.Result{}, err
			}
		}

		if err := r.setUserStatus(ctx, user, username); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.declareUser(ctx, rabbitClient, user); err != nil {
		// Set Condition 'Ready' to false with message
		user.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), user.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, user)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", user.Status)
		}
		return ctrl.Result{}, err
	}

	user.Status.Conditions = []topology.Condition{topology.Ready(user.Status.Conditions)}
	user.Status.ObservedGeneration = user.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, user)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate, "status", user.Status)
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *UserReconciler) declareCredentials(ctx context.Context, user *topology.User) (string, error) {
	logger := ctrl.LoggerFrom(ctx)

	username, password, err := r.generateCredentials(ctx, user)
	if err != nil {
		msg := "failed to generate credentials"
		r.Recorder.Event(user, corev1.EventTypeWarning, "CredentialGenerateFailure", msg)
		logger.Error(err, msg)
		return "", err
	}
	logger.Info("Credentials generated for User", "user", user.Name, "generatedUsername", username)

	credentialSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.ObjectMeta.Name + "-user-credentials",
			Namespace: user.ObjectMeta.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		// The format of the generated Secret conforms to the Provisioned Service
		// type Spec. For more information, see https://k8s-service-bindings.github.io/spec/#provisioned-service.
		Data: map[string][]byte{
			"username": []byte(username),
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
			// required for OpenShift compatibility. See:
			// https://github.com/rabbitmq/cluster-operator/blob/057b61eb50102a66f504b31464e5956526cbdc90/internal/resource/statefulset.go#L220-L226
			// https://github.com/rabbitmq/messaging-topology-operator/issues/194
			for i := range credentialSecret.ObjectMeta.OwnerReferences {
				credentialSecret.ObjectMeta.OwnerReferences[i].BlockOwnerDeletion = pointer.BoolPtr(false)
			}
			return nil
		})
		return apiError
	})
	if err != nil {
		msg := "failed to create/update credentials secret"
		r.Recorder.Event(&credentialSecret, corev1.EventTypeWarning, string(operationResult), msg)
		logger.Error(err, msg)
		return "", err
	}

	logger.Info("Successfully declared credentials secret", "secret", credentialSecret.Name, "namespace", credentialSecret.Namespace)
	r.Recorder.Event(&credentialSecret, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared user")
	return username, nil
}

func (r *UserReconciler) generateCredentials(ctx context.Context, user *topology.User) (string, string, error) {
	logger := ctrl.LoggerFrom(ctx)

	var err error
	msg := fmt.Sprintf("generating/importing credentials for User %s: %#v", user.Name, user)
	logger.Info(msg)

	if user.Spec.ImportCredentialsSecret != nil {
		logger.Info("An import secret was provided in the user spec", "user", user.Name, "secretName", user.Spec.ImportCredentialsSecret.Name)
		return r.importCredentials(ctx, user.Spec.ImportCredentialsSecret.Name, user.Namespace)
	}

	username, err := internal.RandomEncodedString(24)
	if err != nil {
		msg := "failed to generate random username"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return "", "", err
	}
	password, err := internal.RandomEncodedString(24)
	if err != nil {
		msg := "failed to generate random password"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return "", "", err
	}
	return username, password, nil

}

func (r *UserReconciler) importCredentials(ctx context.Context, secretName, secretNamespace string) (string, string, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Importing user credentials from provided Secret", "secretName", secretName, "secretNamespace", secretNamespace)

	var credentialsSecret corev1.Secret
	err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, &credentialsSecret)
	if err != nil {
		return "", "", fmt.Errorf("could not find password secret %s in namespace %s; Err: %w", secretName, secretNamespace, err)
	}
	username, ok := credentialsSecret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("could not find username key in credentials secret: %s", credentialsSecret.Name)
	}
	password, ok := credentialsSecret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("could not find password key in credentials secret: %s", credentialsSecret.Name)
	}

	logger.Info("Retrieved credentials from Secret", "secretName", secretName, "retrievedUsername", string(username))
	return string(username), string(password), nil
}

func (r *UserReconciler) setUserStatus(ctx context.Context, user *topology.User, username string) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials := &corev1.LocalObjectReference{
		Name: user.Name + "-user-credentials",
	}
	user.Status.Credentials = credentials
	user.Status.Username = username
	if err := r.Status().Update(ctx, user); err != nil {
		msg := "Failed to update secret status credentials"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedStatusUpdate", msg)
		logger.Error(err, msg, "user", user.Name, "secretRef", credentials)
		return err
	}
	logger.Info("Successfully updated secret status credentials", "user", user.Name, "secretRef", credentials)
	r.Recorder.Event(user, corev1.EventTypeNormal, "SuccessfulStatusUpdate", "Successfully updated user status")
	return nil
}

func (r *UserReconciler) declareUser(ctx context.Context, client rabbitmqclient.Client, user *topology.User) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials, err := r.getUserCredentials(ctx, user)
	if err != nil {
		msg := "failed to retrieve user credentials secret from status"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "user.status", user.Status)
		return err
	}
	logger.Info("Retrieved credentials for user", "user", user.Name, "credentials", credentials.Name)

	userSettings, err := internal.GenerateUserSettings(credentials, user.Spec.Tags)
	if err != nil {
		msg := "failed to generate user settings from credential"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "user.status", user.Status)
		return err
	}
	logger.Info("Generated user settings", "user", user.Name, "settings", userSettings)

	if err := validateResponse(client.PutUser(userSettings.Name, userSettings)); err != nil {
		msg := "failed to declare user"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "user", user.Name)
		return err
	}

	logger.Info("Successfully declared user", "user", user.Name)
	r.Recorder.Event(user, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared user")
	return nil
}

func (r *UserReconciler) getUserCredentials(ctx context.Context, user *topology.User) (*corev1.Secret, error) {
	if user.Status.Credentials == nil {
		return nil, fmt.Errorf("this User does not yet have a Credentials Secret created")
	}
	credentials := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: user.Status.Credentials.Name, Namespace: user.Namespace}, credentials); err != nil {
		return nil, err
	}
	return credentials, nil
}

func (r *UserReconciler) deleteUserFromSecret(ctx context.Context, client rabbitmqclient.Client, user *topology.User) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials, err := r.getUserCredentials(ctx, user)
	if err != nil {
		msg := "failed to retrieve user credentials secret from status"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "user.status", user.Status)
		return err
	}

	err = validateResponseForDeletion(client.DeleteUser(string(credentials.Data["username"])))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find user in rabbitmq server; already deleted", "user", user.Name)
	} else if err != nil {
		msg := "failed to delete user"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "user", user.Name)
		return err
	}
	r.Recorder.Event(user, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted user")
	return removeFinalizer(ctx, r.Client, user)
}

func (r *UserReconciler) deleteUser(ctx context.Context, client rabbitmqclient.Client, user *topology.User) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.DeleteUser(user.Status.Username))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find user in rabbitmq server; already deleted", "user", user.Name)
	} else if err != nil {
		msg := "failed to delete user"
		r.Recorder.Event(user, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "user", user.Name)
		return err
	}
	r.Recorder.Event(user, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted user")
	return removeFinalizer(ctx, r.Client, user)
}

func (r *UserReconciler) SetInternalDomainName(domainName string) {
	r.KubernetesClusterDomain = domainName
}

func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Secret{}, ownerKey, addResourceToIndex); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.User{}).
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
