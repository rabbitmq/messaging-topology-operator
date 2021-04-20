package controllers

import (
	"context"
	"crypto/x509"
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

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
)

const vhostFinalizer = "deletion.finalizers.vhosts.rabbitmq.com"

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

	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		msg := "failed to retrieve system trusted certs"
		r.Recorder.Event(vhost, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg)
		return ctrl.Result{}, err
	}

	rmq, svc, secret, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, vhost.Spec.RabbitmqClusterReference, vhost.Namespace)
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) && !vhost.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info(noSuchRabbitDeletion, "vhost", vhost.Name)
		r.Recorder.Event(vhost, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted vhost")
		return reconcile.Result{}, r.removeFinalizer(ctx, vhost)
	}
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) {
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	}
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	rabbitClient, err := r.RabbitmqClientFactory(rmq, svc, secret, serviceDNSAddress(svc), systemCertPool)

	// Check if the vhost has been marked for deletion
	if !vhost.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteVhost(ctx, rabbitClient, vhost)
	}

	if err := r.addFinalizerIfNeeded(ctx, vhost); err != nil {
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
		vhost.Status.Conditions = []topology.Condition{topology.NotReady(err.Error())}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, vhost)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	vhost.Status.Conditions = []topology.Condition{topology.Ready()}
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

func (r *VhostReconciler) addFinalizerIfNeeded(ctx context.Context, vhost *topology.Vhost) error {
	if vhost.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(vhost, vhostFinalizer) {
		controllerutil.AddFinalizer(vhost, vhostFinalizer)
		if err := r.Client.Update(ctx, vhost); err != nil {
			return err
		}
	}
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
	return r.removeFinalizer(ctx, vhost)
}

func (r *VhostReconciler) removeFinalizer(ctx context.Context, vhost *topology.Vhost) error {
	controllerutil.RemoveFinalizer(vhost, vhostFinalizer)
	if err := r.Client.Update(ctx, vhost); err != nil {
		return err
	}
	return nil
}

func (r *VhostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Vhost{}).
		Complete(r)
}
