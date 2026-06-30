/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controller

import (
	"context"
	"errors"
	"fmt"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var apiGVStr = topology.GroupVersion.String()

const (
	ownerKey  = ".metadata.controller"
	ownerKind = "User"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=users/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update

type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *UserReconciler) declareCredentials(ctx context.Context, user *topology.User) (string, error) {
	logger := ctrl.LoggerFrom(ctx)

	credentials, err := r.generateCredentials(ctx, user)
	if err != nil {
		logger.Error(err, "failed to generate credentials")
		return "", err
	}
	// Neither PasswordHash nor Password wasn't in the provided input secret we need to generate a random password
	if credentials.PasswordHash == nil && credentials.Password == "" {
		credentials.Password, err = internal.RandomEncodedString(24)
		if err != nil {
			return "", fmt.Errorf("failed to generate random password: %w", err)
		}
	}

	logger.Info("Credentials generated for User", "user", user.Name, "generatedUsername", credentials.Username)

	credentialSecretData := map[string][]byte{
		"username": []byte(credentials.Username),
	}
	if credentials.PasswordHash != nil {
		// Create `passwordHash` field only if necessary, to distinguish between an unset hash and an empty one
		credentialSecretData["passwordHash"] = []byte(*credentials.PasswordHash)
	} else {
		// Store password in the credential secret only if it will be used
		credentialSecretData["password"] = []byte(credentials.Password)
	}

	credentialSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name + "-user-credentials",
			Namespace: user.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		// The format of the generated Secret conforms to the Provisioned Service
		// type Spec. For more information, see https://k8s-service-bindings.github.io/spec/#provisioned-service.
		Data: credentialSecretData,
	}

	var operationResult controllerutil.OperationResult
	err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		var apiError error
		operationResult, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, &credentialSecret, func() error {
			if credentialSecret.Labels == nil {
				credentialSecret.Labels = make(map[string]string)
			}
			credentialSecret.Labels[topology.TopologyOperatorLabel] = topology.TopologyOperatorLabelValue
			if err := controllerutil.SetControllerReference(user, &credentialSecret, r.Scheme); err != nil {
				return fmt.Errorf("failed setting controller reference: %v", err)
			}
			// required for OpenShift compatibility. See:
			// https://github.com/rabbitmq/cluster-operator/blob/057b61eb50102a66f504b31464e5956526cbdc90/internal/resource/statefulset.go#L220-L226
			// https://github.com/rabbitmq/messaging-topology-operator/issues/194
			for i := range credentialSecret.OwnerReferences {
				if credentialSecret.ObjectMeta.OwnerReferences[i].Kind == user.Kind {
					credentialSecret.ObjectMeta.OwnerReferences[i].BlockOwnerDeletion = new(false)
				}
			}
			return nil
		})
		return apiError
	})
	if err != nil {
		msg := fmt.Sprintf("failed to create/update credentials secret: %s", string(operationResult))
		logger.Error(err, msg)
		return "", err
	}

	logger.Info("Successfully declared credentials secret", "secret", credentialSecret.Name, "namespace", credentialSecret.Namespace)
	return credentials.Username, nil
}

func (r *UserReconciler) generateCredentials(ctx context.Context, user *topology.User) (internal.UserCredentials, error) {
	logger := ctrl.LoggerFrom(ctx)

	var err error
	msg := fmt.Sprintf("generating/importing credentials for User %s: %#v", user.Name, user)
	logger.Info(msg)

	credentials := internal.UserCredentials{}

	credentials.Username, err = internal.RandomEncodedString(24)
	if err != nil {
		return credentials, fmt.Errorf("failed to generate random username: %w", err)
	}
	credentials.Password, err = internal.RandomEncodedString(24)
	if err != nil {
		return credentials, fmt.Errorf("failed to generate random password: %w", err)
	}
	return credentials, nil
}

func (r *UserReconciler) importCredentials(ctx context.Context, secretName, secretNamespace string) (internal.UserCredentials, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Importing user credentials from provided Secret", "secretName", secretName, "secretNamespace", secretNamespace)

	var credentials internal.UserCredentials
	var credentialsSecret corev1.Secret

	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, &credentialsSecret)
	if err != nil {
		return credentials, fmt.Errorf("could not find password secret %s in namespace %s; Err: %w", secretName, secretNamespace, err)
	}

	username, ok := credentialsSecret.Data["username"]
	if !ok || len(username) == 0 {
		return credentials, fmt.Errorf("could not find username key in credentials secret: %s", credentialsSecret.Name)
	}
	credentials.Username = string(username)

	password := credentialsSecret.Data["password"]
	credentials.Password = string(password)

	passwordHash, ok := credentialsSecret.Data["passwordHash"]
	if ok {
		credentials.PasswordHash = new(string)
		*credentials.PasswordHash = string(passwordHash)
	}

	logger.Info("Retrieved credentials from Secret", "secretName", secretName, "retrievedUsername", string(username))
	return credentials, nil
}

func (r *UserReconciler) setUserStatus(ctx context.Context, user *topology.User, username, credentialsSecretName string) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials := &corev1.LocalObjectReference{
		Name: credentialsSecretName,
	}
	user.Status.Credentials = credentials
	user.Status.Username = username
	if err := r.Status().Update(ctx, user); err != nil {
		msg := "Failed to update secret status credentials"
		logger.Error(err, msg, "user", user.Name, "secretRef", credentials)
		return err
	}
	logger.Info("Successfully updated secret status credentials", "user", user.Name, "secretRef", credentials)
	return nil
}

func (r *UserReconciler) DeclareFunc(ctx context.Context, rmqc rabbitmqclient.Client, obj topology.TopologyResource) error {
	user := obj.(*topology.User)
	if user.Spec.ImportCredentialsSecret != nil {
		return r.declareWithImportedCredentials(ctx, rmqc, user)
	}
	return r.declareWithGeneratedCredentials(ctx, rmqc, user)
}

func (r *UserReconciler) declareWithImportedCredentials(ctx context.Context, rmqc rabbitmqclient.Client, user *topology.User) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials, err := r.importCredentials(ctx, user.Spec.ImportCredentialsSecret.Name, user.Namespace)
	if err != nil {
		return err
	}

	// Update status on first run, or correct a migration where Status.Credentials.Name
	// still points to the old generated secret rather than the import secret.
	if user.Status.Credentials == nil ||
		user.Status.Credentials.Name != user.Spec.ImportCredentialsSecret.Name ||
		user.Status.Username != credentials.Username {
		if err := r.setUserStatus(ctx, user, credentials.Username, user.Spec.ImportCredentialsSecret.Name); err != nil {
			return err
		}
	}

	userSettings, err := internal.GenerateUserSettings(credentials, user.Spec.Tags)
	if err != nil {
		return fmt.Errorf("failed to generate user settings from credentials: %w", err)
	}
	logger.Info("Generated user settings from import secret", "user", user.Name)

	if err := validateResponse(rmqc.PutUser(userSettings.Name, userSettings)); err != nil {
		return err
	}

	return r.reconcileUserLimits(ctx, rmqc, user, credentials.Username)
}

func (r *UserReconciler) declareWithGeneratedCredentials(ctx context.Context, rmqc rabbitmqclient.Client, user *topology.User) error {
	logger := ctrl.LoggerFrom(ctx)

	// Always call declareCredentials to ensure the generated secret exists and is labeled.
	// CreateOrUpdate fetches the existing secret first, so the mutate function only adds the
	// label — credentials in the Data field are never regenerated for existing secrets.
	username, err := r.declareCredentials(ctx, user)
	if err != nil {
		return err
	}

	credentials, err := r.getUserCredentials(ctx, user)
	if err != nil {
		// Cache propagation lag after the first labeling on upgrade: requeue.
		return fmt.Errorf("failed to retrieve generated credentials secret (may be requeuing after label migration): %w", err)
	}
	logger.Info("Retrieved credentials for user", "user", user.Name, "credentials", credentials.Name)

	// username from declareCredentials is only valid for newly created secrets.
	// For existing secrets, read the actual username from the secret.
	actualUsername := string(credentials.Data["username"])
	if actualUsername == "" {
		actualUsername = username
	}

	if user.Status.Credentials == nil || user.Status.Username == "" ||
		user.Status.Credentials.Name != user.Name+"-user-credentials" {
		if err := r.setUserStatus(ctx, user, actualUsername, user.Name+"-user-credentials"); err != nil {
			return err
		}
	}

	userSettings, err := internal.GenerateUserSettings(secretToCredentials(credentials), user.Spec.Tags)
	if err != nil {
		return fmt.Errorf("failed to generate user settings from credential: %w", err)
	}
	logger.Info("Generated user settings", "user", user.Name, "settings", userSettings)

	if err := validateResponse(rmqc.PutUser(userSettings.Name, userSettings)); err != nil {
		return err
	}

	return r.reconcileUserLimits(ctx, rmqc, user, actualUsername)
}

func (r *UserReconciler) reconcileUserLimits(ctx context.Context, rmqc rabbitmqclient.Client, user *topology.User, username string) error {
	logger := ctrl.LoggerFrom(ctx)

	newUserLimits := internal.GenerateUserLimits(user.Spec.UserLimits)
	logger.Info("Getting existing user limits", "user", user.Name)
	existingUserLimits, err := r.getUserLimits(rmqc, username)
	if err != nil {
		return err
	}
	limitsToDelete := r.userLimitsToDelete(existingUserLimits, newUserLimits)
	if len(limitsToDelete) > 0 {
		logger.Info("Deleting outdated user limits", "user", user.Name, "limits", limitsToDelete)
		if err := validateResponseForDeletion(rmqc.DeleteUserLimits(username, limitsToDelete)); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	if len(newUserLimits) > 0 {
		logger.Info("Creating new user limits", "user", user.Name, "limits", newUserLimits)
		return validateResponse(rmqc.PutUserLimits(username, newUserLimits))
	}
	return nil
}

func secretToCredentials(s *corev1.Secret) internal.UserCredentials {
	c := internal.UserCredentials{
		Username: string(s.Data["username"]),
		Password: string(s.Data["password"]),
	}
	if h, ok := s.Data["passwordHash"]; ok {
		hs := string(h)
		c.PasswordHash = &hs
	}
	return c
}

func (r *UserReconciler) userLimitsToDelete(existingUserLimits, newUserLimits rabbithole.UserLimitsValues) (limitsToDelete rabbithole.UserLimits) {
	userLimitKeys := []string{"max-connections", "max-channels"}
	for _, limit := range userLimitKeys {
		_, oldExists := existingUserLimits[limit]
		_, newExists := newUserLimits[limit]
		if oldExists && !newExists {
			limitsToDelete = append(limitsToDelete, limit)
		}
	}
	return limitsToDelete
}

func (r *UserReconciler) getUserLimits(rmqc rabbitmqclient.Client, username string) (rabbithole.UserLimitsValues, error) {
	userLimitsInfo, err := rmqc.GetUserLimits(username)
	if errors.Is(err, error(rabbithole404)) {
		return rabbithole.UserLimitsValues{}, nil
	} else if err != nil {
		return rabbithole.UserLimitsValues{}, err
	}
	if len(userLimitsInfo) == 0 {
		return rabbithole.UserLimitsValues{}, nil
	}
	return userLimitsInfo[0].Value, nil
}

func (r *UserReconciler) getUserCredentials(ctx context.Context, user *topology.User) (*corev1.Secret, error) {
	credentials := &corev1.Secret{}
	secretName := user.Name + "-user-credentials"
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: user.Namespace}, credentials); err != nil {
		return nil, err
	}
	return credentials, nil
}

func (r *UserReconciler) DeleteFunc(ctx context.Context, rmqc rabbitmqclient.Client, obj topology.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	user := obj.(*topology.User)

	if user.Status.Username == "" {
		logger.Info("user never successfully created in rabbitmq server; skipping deletion", "user", user.Name)
		return nil
	}

	err := validateResponseForDeletion(rmqc.DeleteUser(user.Status.Username))
	if errors.Is(err, ErrNotFound) {
		logger.Info("cannot find user in rabbitmq server; already deleted", "user", user.Name)
	} else if err != nil {
		return err
	}
	return nil
}

func (r *UserReconciler) SetupControllerBuilder(ctx context.Context, mgr ctrl.Manager, b *ctrlbuilder.Builder) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &topology.User{}, ".spec.importCredentialsSecret.name",
		func(obj client.Object) []string {
			u := obj.(*topology.User)
			if u.Spec.ImportCredentialsSecret != nil {
				return []string{u.Spec.ImportCredentialsSecret.Name}
			}
			return nil
		},
	); err != nil {
		return err
	}

	b.Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.secretToUserRequests))
	return nil
}

func (r *UserReconciler) secretToUserRequests(ctx context.Context, obj client.Object) []reconcile.Request {
	secret := obj.(*corev1.Secret)

	// Generated secret: owned by a User CR
	if owner := metav1.GetControllerOf(secret); owner != nil && owner.Kind == ownerKind {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{
			Name: owner.Name, Namespace: secret.Namespace,
		}}}
	}

	// Import secret: find Users referencing it via field index
	var userList topology.UserList
	if err := r.List(ctx, &userList,
		client.InNamespace(secret.Namespace),
		client.MatchingFields{".spec.importCredentialsSecret.name": secret.Name},
	); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(userList.Items))
	for _, u := range userList.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: u.Name, Namespace: u.Namespace},
		})
	}
	return reqs
}
