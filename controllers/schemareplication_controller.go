package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
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

const schemaReplicationParameterName = "schema_definition_sync_upstream"

// SchemaReplicationReconciler reconciles a SchemaReplication object
type SchemaReplicationReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	RabbitmqClientFactory   rabbitmqclient.Factory
	KubernetesClusterDomain string
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=schemareplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=schemareplications/status,verbs=get;update;patch

func (r *SchemaReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	replication := &topology.SchemaReplication{}
	if err := r.Get(ctx, req.NamespacedName, replication); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, replication)
	if err != nil {
		return ctrl.Result{}, err
	}

	credsProvider, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, r.Client, replication.Spec.RabbitmqClusterReference, replication.Namespace, r.KubernetesClusterDomain)
	if err != nil {
		return handleRMQReferenceParseError(ctx, r.Client, r.Recorder, replication, &replication.Status.Conditions, err)
	}

	rabbitClient, err := r.RabbitmqClientFactory(credsProvider, tlsEnabled, systemCertPool)
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return reconcile.Result{}, err
	}

	if !replication.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteSchemaReplicationParameters(ctx, rabbitClient, replication)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, replication); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(replication.Spec)
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.setSchemaReplicationUpstream(ctx, rabbitClient, replication); err != nil {
		// Set Condition 'Ready' to false with message
		replication.Status.Conditions = []topology.Condition{
			topology.NotReady(err.Error(), replication.Status.Conditions),
		}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, replication)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", replication.Status)
		}
		return ctrl.Result{}, err
	}

	replication.Status.Conditions = []topology.Condition{topology.Ready(replication.Status.Conditions)}
	replication.Status.ObservedGeneration = replication.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, replication)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate, "status", replication.Status)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *SchemaReplicationReconciler) setSchemaReplicationUpstream(ctx context.Context, client rabbitmqclient.RabbitMQClient, replication *topology.SchemaReplication) error {
	logger := ctrl.LoggerFrom(ctx)

	endpoints, err := r.getUpstreamEndpoints(ctx, replication)
	if err != nil {
		msg := "failed to generate upstream endpoints"
		r.Recorder.Event(replication, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg, "upstream secret", replication.Spec.UpstreamSecret)
		return err
	}

	if err := validateResponse(client.PutGlobalParameter(schemaReplicationParameterName, endpoints)); err != nil {
		msg := fmt.Sprintf("failed to set '%s' global parameter", schemaReplicationParameterName)
		r.Recorder.Event(replication, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg, "upstream secret", replication.Spec.UpstreamSecret)
		return err
	}

	msg := fmt.Sprintf("successfully set '%s' global parameter", schemaReplicationParameterName)
	logger.Info(msg)
	r.Recorder.Event(replication, corev1.EventTypeNormal, "SuccessfulUpdate", msg)
	return nil
}

func (r *SchemaReplicationReconciler) deleteSchemaReplicationParameters(ctx context.Context, client rabbitmqclient.RabbitMQClient, replication *topology.SchemaReplication) error {
	logger := ctrl.LoggerFrom(ctx)

	err := validateResponseForDeletion(client.DeleteGlobalParameter(schemaReplicationParameterName))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find global parameter; no need to delete it", "parameter", schemaReplicationParameterName)
	} else if err != nil {
		msg := fmt.Sprintf("failed to delete global parameter '%s'", schemaReplicationParameterName)
		r.Recorder.Event(replication, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg)
		return err
	}

	msg := fmt.Sprintf("successfully delete '%s' global parameter", schemaReplicationParameterName)
	logger.Info(msg)
	r.Recorder.Event(replication, corev1.EventTypeNormal, "SuccessfulDelete", msg)
	return removeFinalizer(ctx, r.Client, replication)
}

func (r *SchemaReplicationReconciler) getUpstreamEndpoints(ctx context.Context, replication *topology.SchemaReplication) (internal.UpstreamEndpoints, error) {
	if replication.Spec.UpstreamSecret == nil {
		return internal.UpstreamEndpoints{}, fmt.Errorf("no upstream secret provided")
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: replication.Spec.UpstreamSecret.Name, Namespace: replication.Namespace}, secret); err != nil {
		return internal.UpstreamEndpoints{}, err
	}

	endpoints, err := internal.GenerateSchemaReplicationParameters(secret, replication.Spec.Endpoints)
	if err != nil {
		return internal.UpstreamEndpoints{}, err
	}

	return endpoints, nil
}

func (r *SchemaReplicationReconciler) SetInternalDomainName(domainName string) {
	r.KubernetesClusterDomain = domainName
}

func (r *SchemaReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.SchemaReplication{}).
		Complete(r)
}
