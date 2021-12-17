package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// ShovelReconciler reconciles a Shovel object
type ShovelReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	RabbitmqClientFactory   internal.RabbitMQClientFactory
	KubernetesClusterDomain string
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=shovels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=shovels/status,verbs=get;update;patch

func (r *ShovelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	shovel := &topology.Shovel{}
	if err := r.Get(ctx, req.NamespacedName, shovel); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, shovel)
	if err != nil {
		return ctrl.Result{}, err
	}

	credsProvider, tlsEnabled, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, shovel.Spec.RabbitmqClusterReference, shovel.Namespace)
	if err != nil {
		return handleRMQReferenceParseError(ctx, r.Client, r.Recorder, shovel, &shovel.Status.Conditions, err)
	}

	rabbitClient, err := r.RabbitmqClientFactory(credsProvider, tlsEnabled, systemCertPool)
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	if !shovel.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteShovel(ctx, rabbitClient, shovel)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, shovel); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(shovel.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling", "spec", string(spec))

	if err := r.declareShovel(ctx, rabbitClient, shovel); err != nil {
		// Set Condition 'Ready' to false with message
		shovel.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), shovel.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, shovel)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", shovel.Status)
		}
		return ctrl.Result{}, err
	}

	shovel.Status.Conditions = []topology.Condition{topology.Ready(shovel.Status.Conditions)}
	shovel.Status.ObservedGeneration = shovel.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, shovel)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate, "status", shovel.Status)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *ShovelReconciler) declareShovel(ctx context.Context, client internal.RabbitMQClient, shovel *topology.Shovel) error {
	logger := ctrl.LoggerFrom(ctx)

	srcUri, destUri, err := r.getUris(ctx, shovel)
	if err != nil {
		msg := "failed to parse shovel uri secret"
		r.Recorder.Event(shovel, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg, "uri secret", shovel.Spec.UriSecret.Name)
		return err
	}

	if err := validateResponse(client.DeclareShovel(shovel.Spec.Vhost, shovel.Spec.Name, internal.GenerateShovelDefinition(shovel, srcUri, destUri))); err != nil {
		msg := "failed to declare shovel"
		r.Recorder.Event(shovel, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg, "shovel", shovel.Spec.Name)
		return err
	}

	logger.Info("Successfully declare shovel", "shovel", shovel.Spec.Name)
	r.Recorder.Event(shovel, corev1.EventTypeNormal, "SuccessfulUpdate", "Successfully declare shovel")
	return nil
}
func (r *ShovelReconciler) getUris(ctx context.Context, shovel *topology.Shovel) (string, string, error) {
	if shovel.Spec.UriSecret == nil {
		return "", "", fmt.Errorf("no uri secret provided")
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: shovel.Spec.UriSecret.Name, Namespace: shovel.Namespace}, secret); err != nil {
		return "", "", err
	}

	srcUri, ok := secret.Data["srcUri"]
	if !ok {
		return "", "", fmt.Errorf("could not find key 'srcUri' in secret %s", secret.Name)
	}

	destUri, ok := secret.Data["destUri"]
	if !ok {
		return "", "", fmt.Errorf("could not find key 'srcUri' in secret %s", secret.Name)
	}

	return string(srcUri), string(destUri), nil
}

// deletes shovel configuration from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *ShovelReconciler) deleteShovel(ctx context.Context, client internal.RabbitMQClient, shovel *topology.Shovel) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.DeleteShovel(shovel.Spec.Vhost, shovel.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find shovel parameter; no need to delete it", "shovel", shovel.Spec.Name)
	} else if err != nil {
		msg := "failed to delete shovel parameter"
		r.Recorder.Event(shovel, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "shovel", shovel.Spec.Name)
		return err
	}
	r.Recorder.Event(shovel, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted shovel parameter")
	return removeFinalizer(ctx, r.Client, shovel)
}

func (r *ShovelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Shovel{}).
		Complete(r)
}
