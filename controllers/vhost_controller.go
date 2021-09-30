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

// VhostReconciler reconciles a Vhost object
type VhostReconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	RabbitmqClientFactory internal.RabbitMQClientFactory
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts/status,verbs=get;update;patch

func (r *VhostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	vhost := &topology.Vhost{}
	if err := r.Get(ctx, req.NamespacedName, vhost); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, vhost)
	if err != nil {
		return ctrl.Result{}, err
	}

	rmq, svc, secret, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, vhost.Spec.RabbitmqClusterReference, vhost.Namespace)
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) && !vhost.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info(noSuchRabbitDeletion, "vhost", vhost.Name)
		r.Recorder.Event(vhost, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted vhost")
		return reconcile.Result{}, removeFinalizer(ctx, r.Client, vhost)
	}
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) {
		// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
		// the Cluster is temporarily down. Requeue until it comes back up.
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	}
	if errors.Is(err, internal.ResourceNotAllowedError) {
		logger.Info("Could not create vhost resource: " + err.Error())
		vhost.Status.Conditions = []topology.Condition{
			topology.NotReady(internal.ResourceNotAllowedError.Error(), vhost.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, vhost)
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

	// Check if the vhost has been marked for deletion
	if !vhost.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteVhost(ctx, rabbitClient, vhost)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, vhost); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(vhost.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.putVhost(ctx, rabbitClient, vhost); err != nil {
		// Set Condition 'Ready' to false with message
		vhost.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), vhost.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, vhost)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	vhost.Status.Conditions = []topology.Condition{topology.Ready(vhost.Status.Conditions)}
	vhost.Status.ObservedGeneration = vhost.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, vhost)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *VhostReconciler) putVhost(ctx context.Context, client internal.RabbitMQClient, vhost *topology.Vhost) error {
	logger := ctrl.LoggerFrom(ctx)

	vhostSettings := internal.GenerateVhostSettings(vhost)

	if err := validateResponse(client.PutVhost(vhost.Spec.Name, *vhostSettings)); err != nil {
		msg := "failed to create vhost"
		r.Recorder.Event(vhost, corev1.EventTypeWarning, "FailedCreate", msg)
		logger.Error(err, msg, "vhost", vhost.Spec.Name)
		return err
	}

	logger.Info("Successfully created vhost", "vhost", vhost.Spec.Name)
	r.Recorder.Event(vhost, corev1.EventTypeNormal, "SuccessfulCreate", "Successfully created vhost")
	return nil
}

// deletes vhost from server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *VhostReconciler) deleteVhost(ctx context.Context, client internal.RabbitMQClient, vhost *topology.Vhost) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.DeleteVhost(vhost.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find vhost in rabbitmq server; already deleted", "vhost", vhost.Spec.Name)
	} else if err != nil {
		msg := "failed to delete vhost"
		r.Recorder.Event(vhost, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "vhost", vhost.Spec.Name)
		return err
	}

	r.Recorder.Event(vhost, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted vhost")
	return removeFinalizer(ctx, r.Client, vhost)
}

func (r *VhostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Vhost{}).
		Complete(r)
}
