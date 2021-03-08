package controllers

import (
	"context"
	"encoding/json"
	"errors"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

const vhostFinalizer = "deletion.finalizers.vhosts.rabbitmq.com"

// VhostReconciler reconciles a Vhost object
type VhostReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts/status,verbs=get;update;patch

func (r *VhostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	vhost := &topologyv1alpha1.Vhost{}
	if err := r.Get(ctx, req.NamespacedName, vhost); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	rabbitClient, err := rabbitholeClient(ctx, r.Client, vhost.Spec.RabbitmqClusterReference)
	if err != nil {
		logger.Error(err, "Failed to generate http rabbitClient")
		return reconcile.Result{}, err
	}

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
		logger.Error(err, "Failed to marshal vhost spec")
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.putVhost(ctx, rabbitClient, vhost); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *VhostReconciler) putVhost(ctx context.Context, client *rabbithole.Client, vhost *topologyv1alpha1.Vhost) error {
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

func (r *VhostReconciler) addFinalizerIfNeeded(ctx context.Context, vhost *topologyv1alpha1.Vhost) error {
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
func (r *VhostReconciler) deleteVhost(ctx context.Context, client *rabbithole.Client, vhost *topologyv1alpha1.Vhost) error {
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

	return r.removeFinalizer(ctx, vhost)
}

func (r *VhostReconciler) removeFinalizer(ctx context.Context, vhost *topologyv1alpha1.Vhost) error {
	controllerutil.RemoveFinalizer(vhost, vhostFinalizer)
	if err := r.Client.Update(ctx, vhost); err != nil {
		return err
	}
	return nil
}

func (r *VhostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1alpha1.Vhost{}).
		Complete(r)
}
