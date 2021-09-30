package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

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
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	RabbitmqClientFactory internal.RabbitMQClientFactory
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

	rmq, svc, secret, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, permission.Spec.RabbitmqClusterReference, permission.Namespace)
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) && !permission.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info(noSuchRabbitDeletion, "permission", permission.Name)
		r.Recorder.Event(permission, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted permission")
		return reconcile.Result{}, removeFinalizer(ctx, r.Client, permission)
	}
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) {
		// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
		// the Cluster is temporarily down. Requeue until it comes back up.
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	}
	if errors.Is(err, internal.ResourceNotAllowedError) {
		logger.Info("Could not create permission resource: " + err.Error())
		permission.Status.Conditions = []topology.Condition{
			topology.NotReady(internal.ResourceNotAllowedError.Error(), permission.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, permission)
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

	user := permission.Spec.User
	if permission.Spec.UserReference != nil {
		if user, err = r.getUserFromReference(ctx, permission); err != nil {
			return reconcile.Result{}, err
		}
	}

	if !permission.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.revokePermissions(ctx, rabbitClient, permission, user)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, permission); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(permission.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.updatePermissions(ctx, rabbitClient, permission, user); err != nil {
		// Set Condition 'Ready' to false with message
		permission.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), permission.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, permission)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	permission.Status.Conditions = []topology.Condition{topology.Ready(permission.Status.Conditions)}
	permission.Status.ObservedGeneration = permission.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, permission)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *PermissionReconciler) getUserFromReference(ctx context.Context, permission *topology.Permission) (string, error) {
	logger := ctrl.LoggerFrom(ctx)

	// get User from provided user reference
	failureMsg := "failed to get User"
	user := &topology.User{}
	if err := r.Get(ctx, types.NamespacedName{Name: permission.Spec.UserReference.Name, Namespace: permission.Namespace}, user); err != nil {
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedUpdate", failureMsg)
		logger.Error(err, failureMsg, "userReference", permission.Spec.UserReference.Name)
		return "", err
	}

	// get username from the credential secret from User status
	if user.Status.Credentials == nil {
		err := fmt.Errorf("this User does not have a credential secret set in its status")
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedUpdate", failureMsg)
		logger.Error(err, failureMsg, "userReference", permission.Spec.UserReference.Name)
		return "", err
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: user.Status.Credentials.Name, Namespace: user.Namespace}, secret); err != nil {
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedUpdate", failureMsg)
		logger.Error(err, failureMsg, "userReference", permission.Spec.UserReference.Name)
		return "", err
	}

	username, ok := secret.Data["username"]
	if !ok {
		err := fmt.Errorf("could not find username key")
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedUpdate", failureMsg)
		logger.Error(err, failureMsg, "userReference", permission.Spec.UserReference.Name)
		return "", nil
	}

	return string(username), nil
}

func (r *PermissionReconciler) updatePermissions(ctx context.Context, client internal.RabbitMQClient, permission *topology.Permission, user string) error {
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

func (r *PermissionReconciler) revokePermissions(ctx context.Context, client internal.RabbitMQClient, permission *topology.Permission, user string) error {
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
	r.Recorder.Event(permission, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted permission")
	return removeFinalizer(ctx, r.Client, permission)
}

func (r *PermissionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Permission{}).
		Complete(r)
}
