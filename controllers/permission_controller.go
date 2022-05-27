package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

// PermissionReconciler reconciles a Permission object
type PermissionReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	RabbitmqClientFactory   rabbitmqclient.Factory
	KubernetesClusterDomain string
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=permissions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=permissions/status,verbs=get;update;patch

func (r *PermissionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	permission := &topology.Permission{}
	if err := r.Get(ctx, req.NamespacedName, permission); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, permission)
	if err != nil {
		return ctrl.Result{}, err
	}

	credsProvider, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, r.Client, permission.Spec.RabbitmqClusterReference, permission.Namespace, r.KubernetesClusterDomain)
	if err != nil {
		return handleRMQReferenceParseError(ctx, r.Client, r.Recorder, permission, &permission.Status.Conditions, err)
	}

	rabbitClient, err := r.RabbitmqClientFactory(credsProvider, tlsEnabled, systemCertPool)
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	user := &topology.User{}
	username := permission.Spec.User
	if permission.Spec.UserReference != nil {

		if user, err = r.getUserFromReference(ctx, permission); err != nil {
			return reconcile.Result{}, err
		} else if user != nil {
			// User exist
			username = user.Status.Username
		}
	}

	if !permission.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")

		if username == "" {
			msg := "user already removed; no need to delete permission"
			logger.Info(msg)
			r.Recorder.Event(permission, corev1.EventTypeWarning, "UserNotExist", msg)
		} else if err := r.revokePermissions(ctx, rabbitClient, permission, username); err != nil {
			return ctrl.Result{}, err
		}

		r.Recorder.Event(permission, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted permission")
		return ctrl.Result{}, removeFinalizer(ctx, r.Client, permission)
	}

	// User not exist stop create or update operations
	if username == "" {
		msg := "failed create Permission, missing User"

		permission.Status.Conditions = []topology.Condition{
			topology.NotReady(msg, permission.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, permission)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", permission.Status)
		}

		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedCreateOrUpdate", msg)
		logger.Error(err, msg)
		return reconcile.Result{}, fmt.Errorf(msg)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, permission); err != nil {
		return ctrl.Result{}, err
	}

	// user != nil, not working because user has always a name set
	if user.Name != "" {
		if err := controllerutil.SetControllerReference(user, permission, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed set controller reference: %v", err)
		}
		if err := r.Client.Update(ctx, permission); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to Update object with controller reference: %w", err)
		}
	}

	spec, err := json.Marshal(permission.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.updatePermissions(ctx, rabbitClient, permission, username); err != nil {
		// Set Condition 'Ready' to false with message
		permission.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), permission.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, permission)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", permission.Status)
		}
		return ctrl.Result{}, err
	}

	permission.Status.Conditions = []topology.Condition{topology.Ready(permission.Status.Conditions)}
	permission.Status.ObservedGeneration = permission.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, permission)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate, "status", permission.Status)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *PermissionReconciler) getUserFromReference(ctx context.Context, permission *topology.Permission) (*topology.User, error) {
	logger := ctrl.LoggerFrom(ctx)

	// get User from provided user reference
	failureMsg := "failed to get User"
	user := &topology.User{}
	err := r.Get(ctx, types.NamespacedName{Name: permission.Spec.UserReference.Name, Namespace: permission.Namespace}, user)

	if err != nil && k8sApiErrors.IsNotFound(err) {
		r.Recorder.Event(permission, corev1.EventTypeWarning, "NotExist", failureMsg)
		return nil, nil
	} else if err != nil {
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedUpdate", failureMsg)
		logger.Error(err, failureMsg, "userReference", permission.Spec.UserReference.Name)
		return nil, err
	}

	// get username from User status
	if user.Status.Username == "" {
		err := fmt.Errorf("this User does not have an username set in its status")
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedUpdate", failureMsg)
		logger.Error(err, failureMsg, "userReference", permission.Spec.UserReference.Name)
		return nil, err
	}

	return user, nil
}

func (r *PermissionReconciler) updatePermissions(ctx context.Context, client rabbitmqclient.RabbitMQClient, permission *topology.Permission, user string) error {
	logger := ctrl.LoggerFrom(ctx)

	if err := validateResponse(client.UpdatePermissionsIn(permission.Spec.Vhost, user, internal.GeneratePermissions(permission))); err != nil {
		msg := "failed to set permission"
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg, "user", user, "vhost", permission.Spec.Vhost)
		return err
	}

	logger.Info("Successfully set permission", "user", user, "vhost", permission.Spec.Vhost)
	r.Recorder.Event(permission, corev1.EventTypeNormal, "SuccessfulUpdate", "successfully set permission")
	return nil
}

func (r *PermissionReconciler) revokePermissions(ctx context.Context, client rabbitmqclient.RabbitMQClient, permission *topology.Permission, user string) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.ClearPermissionsIn(permission.Spec.Vhost, user))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find user or vhost in rabbitmq server; no need to delete permission", "user", user, "vhost", permission.Spec.Vhost)
	} else if err != nil {
		msg := "failed to delete permission"
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "user", user, "vhost", permission.Spec.Vhost)
		return err
	}
	return nil
}

func (r *PermissionReconciler) SetInternalDomainName(domainName string) {
	r.KubernetesClusterDomain = domainName
}

func (r *PermissionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Permission{}).
		Complete(r)
}
