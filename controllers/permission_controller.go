package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rabbitmq/messaging-topology-operator/internal"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=permissions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=permissions/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=permissions/status,verbs=get;update;patch

type PermissionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *PermissionReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	permission := obj.(*topology.Permission)
	user := &topology.User{}
	username := permission.Spec.User
	if permission.Spec.UserReference != nil {
		var err error
		if user, err = r.getUserFromReference(ctx, permission); err != nil {
			return err
		} else if user != nil {
			// User exist
			username = user.Status.Username
		}
	}
	if username == "" {
		return fmt.Errorf("failed create Permission, missing User")
	}

	// user != nil, not working because user has always a name set
	if user.Name != "" {
		if err := controllerutil.SetControllerReference(user, permission, r.Scheme); err != nil {
			return fmt.Errorf("failed set controller reference: %v", err)
		}
		if err := r.Client.Update(ctx, permission); err != nil {
			return fmt.Errorf("failed to Update object with controller reference: %w", err)
		}
	}
	return validateResponse(client.UpdatePermissionsIn(permission.Spec.Vhost, username, internal.GeneratePermissions(permission)))
}

func (r *PermissionReconciler) getUserFromReference(ctx context.Context, permission *topology.Permission) (*topology.User, error) {
	logger := ctrl.LoggerFrom(ctx)

	// get User from provided user reference
	failureMsg := "failed to get User"
	user := &topology.User{}
	err := r.Get(ctx, types.NamespacedName{Name: permission.Spec.UserReference.Name, Namespace: permission.Namespace}, user)

	if err != nil && k8sApiErrors.IsNotFound(err) {
		logger.Error(fmt.Errorf("user doesn't exist"), failureMsg)
		return nil, nil
	} else if err != nil {
		logger.Error(err, failureMsg, "userReference", permission.Spec.UserReference.Name)
		return nil, err
	}

	// get username from User status
	if user.Status.Username == "" {
		err := fmt.Errorf("this User does not have an username set in its status")
		logger.Error(err, failureMsg, "userReference", permission.Spec.UserReference.Name)
		return nil, err
	}
	return user, nil
}

func (r *PermissionReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	permission := obj.(*topology.Permission)

	username := permission.Spec.User
	if permission.Spec.UserReference != nil {
		if user, err := r.getUserFromReference(ctx, permission); err != nil {
			return err
		} else if user != nil {
			// User exist
			username = user.Status.Username
		}
	}

	if username == "" {
		logger.Info("user already removed; no need to delete permission")
	} else if err := r.revokePermissions(ctx, client, permission, username); err != nil {
		return err
	}
	return removeFinalizer(ctx, r.Client, permission)
}

func (r *PermissionReconciler) revokePermissions(ctx context.Context, client rabbitmqclient.Client, permission *topology.Permission, user string) error {
	logger := ctrl.LoggerFrom(ctx)
	err := validateResponseForDeletion(client.ClearPermissionsIn(permission.Spec.Vhost, user))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find user or vhost in rabbitmq server; no need to delete permission", "user", user, "vhost", permission.Spec.Vhost)
		return nil
	}
	return err
}
