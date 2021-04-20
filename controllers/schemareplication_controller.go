package controllers

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

const replicationFinalizer = "deletion.finalizers.schemareplications.rabbitmq.com"
const schemaReplicationParameterName = "schema_definition_sync_upstream"

// SchemaReplicationReconciler reconciles a SchemaReplication object
type SchemaReplicationReconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	Recorder              record.EventRecorder
	RabbitmqClientFactory internal.RabbitMQClientFactory
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=schemareplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=schemareplications/status,verbs=get;update;patch

func (r *SchemaReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	replication := &topology.SchemaReplication{}
	if err := r.Get(ctx, req.NamespacedName, replication); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		msg := "failed to retrieve system trusted certs"
		r.Recorder.Event(replication, corev1.EventTypeWarning, "FailedUpdate", msg)
		logger.Error(err, msg)
		return ctrl.Result{}, err
	}

	rmq, svc, secret, err := internal.ParseRabbitmqClusterReference(ctx, r.Client, replication.Spec.RabbitmqClusterReference, replication.Namespace)
	if errors.Is(err, internal.NoSuchRabbitmqClusterError) && !replication.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info(noSuchRabbitDeletion, "replication", replication.Name)
		r.Recorder.Event(replication, corev1.EventTypeNormal, "SuccessfulDelete", "successfully deleted replication")
		return reconcile.Result{}, r.removeFinalizer(ctx, replication)
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

	if !replication.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteSchemaReplicationParameters(ctx, rabbitClient, replication)
	}

	if err := r.addFinalizerIfNeeded(ctx, replication); err != nil {
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
		replication.Status.Conditions = []topology.Condition{topology.NotReady(err.Error())}
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, replication)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate)
		}
		return ctrl.Result{}, err
	}

	replication.Status.Conditions = []topology.Condition{topology.Ready()}
	replication.Status.ObservedGeneration = replication.GetGeneration()
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, replication)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate)
	}
	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *SchemaReplicationReconciler) setSchemaReplicationUpstream(ctx context.Context, client internal.RabbitMQClient, replication *topology.SchemaReplication) error {
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

func (r *SchemaReplicationReconciler) addFinalizerIfNeeded(ctx context.Context, replication *topology.SchemaReplication) error {
	if replication.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(replication, replicationFinalizer) {
		controllerutil.AddFinalizer(replication, replicationFinalizer)
		if err := r.Client.Update(ctx, replication); err != nil {
			return err
		}
	}
	return nil
}

func (r *SchemaReplicationReconciler) deleteSchemaReplicationParameters(ctx context.Context, client internal.RabbitMQClient, replication *topology.SchemaReplication) error {
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
	return r.removeFinalizer(ctx, replication)
}

func (r *SchemaReplicationReconciler) getUpstreamEndpoints(ctx context.Context, replication *topology.SchemaReplication) (internal.UpstreamEndpoints, error) {
	if replication.Spec.UpstreamSecret == nil {
		return internal.UpstreamEndpoints{}, fmt.Errorf("no upstream secret provided")
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: replication.Spec.UpstreamSecret.Name, Namespace: replication.Namespace}, secret); err != nil {
		return internal.UpstreamEndpoints{}, err
	}

	endpoints, err := internal.GenerateSchemaReplicationParameters(secret)
	if err != nil {
		return internal.UpstreamEndpoints{}, err
	}

	return endpoints, nil
}

func (r *SchemaReplicationReconciler) removeFinalizer(ctx context.Context, replication *topology.SchemaReplication) error {
	controllerutil.RemoveFinalizer(replication, replicationFinalizer)
	if err := r.Client.Update(ctx, replication); err != nil {
		return err
	}
	return nil
}

func (r *SchemaReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topology.SchemaReplication{}).
		Complete(r)
}
