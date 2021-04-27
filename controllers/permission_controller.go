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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

const permissionFinalizer = "deletion.finalizers.permissions.rabbitmq.com"

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
		return reconcile.Result{}, r.removeFinalizer(ctx, permission)
	}
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) {
		// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
		// the Cluster is temporarily down. Requeue until it comes back up.
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
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

	if !permission.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.revokePermissions(ctx, rabbitClient, permission)
	}

	if err := r.addFinalizerIfNeeded(ctx, permission); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(permission.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.updatePermissions(ctx, rabbitClient, permission); err != nil {
		// Set Condition 'Ready' to false with message
		permission.Status.Conditions = []topology.Condition{topology.NotReady(err.Error())}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, permission)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	permission.Status.Conditions = []topology.Condition{topology.Ready()}
	permission.Status.ObservedGeneration = permission.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, permission)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *PermissionReconciler) updatePermissions(ctx context.Context, client internal.RabbitMQClient, permission *topology.Permission) error {
	logger := ctrl.LoggerFrom(ctx)

	if err := validateResponse(client.UpdatePermissionsIn(permission.Spec.Vhost, permission.Spec.User, internal.GeneratePermissions(permission))); err != nil {
		msg := "failed to set permission"
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg, "user", permission.Spec.User, "vhost", permission.Spec.Vhost)
		return err
	}

	logger.Info("Successfully set permission", "user", permission.Spec.User, "vhost", permission.Spec.Vhost)
	r.Recorder.Event(permission, corev1.EventTypeNormal, "SuccessfulUpdate", "successfully set permission")
	return nil
}

func (r *PermissionReconciler) addFinalizerIfNeeded(ctx context.Context, permission *topology.Permission) error {
	if permission.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(permission, permissionFinalizer) {
		controllerutil.AddFinalizer(permission, permissionFinalizer)
		if err := r.Client.Update(ctx, permission); err != nil {
			return err
		}
	}
	return nil
}

func (r *PermissionReconciler) revokePermissions(ctx context.Context, client internal.RabbitMQClient, permission *topology.Permission) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.ClearPermissionsIn(permission.Spec.Vhost, permission.Spec.User))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find user or vhost in rabbitmq server; no need to delete permission", "user", permission.Spec.User, "vhost", permission.Spec.Vhost)
	} else if err != nil {
		msg := "failed to delete permission"
		r.Recorder.Event(permission, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "user", permission.Spec.User, "vhost", permission.Spec.Vhost)
		return err
	}
	r.Recorder.Event(permission, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted permission")
	return r.removeFinalizer(ctx, permission)
}

func (r *PermissionReconciler) removeFinalizer(ctx context.Context, permission *topology.Permission) error {
	controllerutil.RemoveFinalizer(permission, permissionFinalizer)
	if err := r.Client.Update(ctx, permission); err != nil {
		return err
	}
	return nil
}

func (r *PermissionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Permission{}).
		Complete(r)
}
