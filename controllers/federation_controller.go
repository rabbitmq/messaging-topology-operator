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

// FederationReconciler reconciles a Federation object
type FederationReconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	RabbitmqClientFactory internal.RabbitMQClientFactory
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=federations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=federations/status,verbs=get;update;patch

func (r *FederationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	federation := &topology.Federation{}
	if err := r.Get(ctx, req.NamespacedName, federation); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, federation)
	if err != nil {
		return ctrl.Result{}, err
	}

	rmq, svc, credsProvider, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, federation.Spec.RabbitmqClusterReference, federation.Namespace)
	if err != nil {
		return handleRMQReferenceParseError(ctx, r.Client, r.Recorder, federation, &federation.Status.Conditions, err)
	}

	var hostname string
	if rmq != nil {
		hostname = serviceDNSAddress(svc)
	}

	rabbitClient, err := r.RabbitmqClientFactory(rmq, svc, credsProvider, hostname, systemCertPool)
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	if !federation.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteFederation(ctx, rabbitClient, federation)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, federation); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(federation.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.setFederation(ctx, rabbitClient, federation); err != nil {
		// Set Condition 'Ready' to false with message
		federation.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), federation.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, federation)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", federation.Status)
		}
		return ctrl.Result{}, err
	}

	federation.Status.Conditions = []topology.Condition{topology.Ready(federation.Status.Conditions)}
	federation.Status.ObservedGeneration = federation.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, federation)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate, "status", federation.Status)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *FederationReconciler) setFederation(ctx context.Context, client internal.RabbitMQClient, federation *topology.Federation) error {
	logger := ctrl.LoggerFrom(ctx)

	uri, err := r.getUri(ctx, federation)
	if err != nil {
		msg := "failed to parse federation uri secret"
		r.Recorder.Event(federation, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg, "uri secret", federation.Spec.UriSecret.Name)
		return err
	}

	if err := validateResponse(client.PutFederationUpstream(federation.Spec.Vhost, federation.Spec.Name, internal.GenerateFederationDefinition(federation, uri))); err != nil {
		msg := "failed to set federation upstream parameter"
		r.Recorder.Event(federation, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg, "federation", federation.Spec.Name)
		return err
	}

	logger.Info("Successfully set federation Upstream parameter", "federation", federation.Spec.Name)
	r.Recorder.Event(federation, corev1.EventTypeNormal, "SuccessfulUpdate", "Successfully set federation Upstream parameter")
	return nil
}
func (r *FederationReconciler) getUri(ctx context.Context, federation *topology.Federation) (string, error) {
	if federation.Spec.UriSecret == nil {
		return "", fmt.Errorf("no uri secret provided")
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: federation.Spec.UriSecret.Name, Namespace: federation.Namespace}, secret); err != nil {
		return "", err
	}

	uri, ok := secret.Data["uri"]
	if !ok {
		return "", fmt.Errorf("could not find key 'uri' in secret %s", secret.Name)
	}

	return string(uri), nil
}

// deletes federation from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *FederationReconciler) deleteFederation(ctx context.Context, client internal.RabbitMQClient, federation *topology.Federation) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.DeleteFederationUpstream(federation.Spec.Vhost, federation.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find federation upstream parameter; no need to delete it", "federation", federation.Spec.Name)
	} else if err != nil {
		msg := "failed to delete federation upstream parameter"
		r.Recorder.Event(federation, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "federation", federation.Spec.Name)
		return err
	}
	r.Recorder.Event(federation, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted federation upstream parameter")
	return removeFinalizer(ctx, r.Client, federation)
}

func (r *FederationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.Federation{}).
		Complete(r)
}
