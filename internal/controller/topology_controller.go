package controller

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TopologyReconciler reconciles any topology rabbitmq objects
type TopologyReconciler struct {
	client.Client
	ReconcileFunc
	Type                    client.Object
	WatchTypes              []client.Object
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                events.EventRecorder
	RabbitmqClientFactory   rabbitmqclient.Factory
	KubernetesClusterDomain string
	ConnectUsingPlainHTTP   bool
	MaxConcurrentReconciles int
}

// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is the main entry point for the TopologyReconciler
func (r *TopologyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	obj := r.Type.DeepCopyObject().(topology.TopologyResource)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// generate RabbitMQ client
	systemCertPool, err := extractSystemCertPool(ctx, r.Recorder, obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	credsProvider, tlsEnabled, err := rabbitmqclient.ParseReference(ctx, r.Client, obj.RabbitReference(), obj.GetNamespace(), r.KubernetesClusterDomain, r.ConnectUsingPlainHTTP)
	if err != nil {
		return r.handleRMQReferenceParseError(ctx, obj, err)
	}

	rabbitClient, err := r.RabbitmqClientFactory(credsProvider, tlsEnabled, systemCertPool)
	if err != nil {
		logger.Error(err, failedGenerateRabbitClient)
		return ctrl.Result{}, err
	}

	// call DeleteFunc if obj has been marked for deletion
	objKind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	if obj.GetDeletionTimestamp() != nil {
		logger.Info("Deleting")
		if err := r.DeleteFunc(ctx, rabbitClient, obj); err != nil {
			// log and publish failed event when DeleteFunc errored
			failureMsg := fmt.Sprintf("failed to delete %s", objKind)
			r.Recorder.Eventf(obj, nil, corev1.EventTypeWarning, "FailedDelete", deleteEventAction, failureMsg)
			logger.Error(err, failureMsg)
			return ctrl.Result{}, err
		}
		successMsg := fmt.Sprintf("successfully deleted %s", objKind)
		logger.Info(successMsg)
		r.Recorder.Eventf(obj, nil, corev1.EventTypeNormal, "SuccessfulDelete", deleteEventAction, successMsg)
		return ctrl.Result{}, removeFinalizer(ctx, r.Client, obj)
	}

	if err := addFinalizerIfNeeded(ctx, r.Client, obj); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(r.getTopLevelField(obj, "Spec"))
	if err != nil {
		logger.Error(err, failedMarshalSpec)
	}

	logger.Info("Start reconciling", "spec", string(spec))

	if err := r.DeclareFunc(ctx, rabbitClient, obj); err != nil {
		// log, publish failed event, and set obj status when DeclareFunc errored
		failureMsg := fmt.Sprintf("failed to declare %s", objKind)
		r.Recorder.Eventf(obj, nil, corev1.EventTypeWarning, "FailedDeclare", createEventAction, failureMsg)
		logger.Error(err, failureMsg)
		obj.SetStatusConditions([]topology.Condition{topology.NotReady(err.Error(), r.getStatusConditions(obj))})
		r.statusUpdate(ctx, obj, logger)
		return ctrl.Result{}, err
	}

	// log, publish successful event, and set obj status
	successMsg := fmt.Sprintf("Successfully declare %s", objKind)
	logger.Info(successMsg)
	r.Recorder.Eventf(obj, nil, corev1.EventTypeNormal, "SuccessfulDeclare", createEventAction, successMsg)
	obj.SetStatusConditions([]topology.Condition{topology.Ready(r.getStatusConditions(obj))})
	r.setObservedGeneration(obj)
	r.statusUpdate(ctx, obj, logger)

	logger.Info("Finished reconciling")
	return ctrl.Result{}, nil
}

func extractSystemCertPool(ctx context.Context, recorder events.EventRecorder, object runtime.Object) (*x509.CertPool, error) {
	logger := ctrl.LoggerFrom(ctx)

	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		recorder.Eventf(object, nil, corev1.EventTypeWarning, "FailedUpdate", setupEventAction, failedRetrieveSysCertPool)
		logger.Error(err, failedRetrieveSysCertPool)
	}
	return systemCertPool, err
}

func (r *TopologyReconciler) statusUpdate(ctx context.Context, obj topology.TopologyResource, logger logr.Logger) {
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, obj)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate, "status", r.getTopLevelField(obj, "Status"))
	}
}

func (r *TopologyReconciler) handleRMQReferenceParseError(ctx context.Context, object topology.TopologyResource, err error) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	if err == nil {
		logger.Error(errors.New("expected error to parse, but it was nil"), "Failed to parse error from RabbitmqClusterReference parsing")
		return ctrl.Result{}, err
	}
	if errors.Is(err, rabbitmqclient.ErrNoSuchRabbitmqCluster) && !object.GetDeletionTimestamp().IsZero() {
		logger.Info(noSuchRabbitDeletion, "object", object.GetName())
		r.Recorder.Eventf(object, nil, corev1.EventTypeNormal, "SuccessfulDelete", deleteEventAction, "successfully deleted "+object.GetName())
		return ctrl.Result{}, removeFinalizer(ctx, r.Client, object)
	}
	if errors.Is(err, rabbitmqclient.ErrNoSuchRabbitmqCluster) {
		// If the object is not being deleted, but the RabbitmqCluster no longer exists, it could be that
		// the Cluster is temporarily down. Requeue until it comes back up.
		logger.Info("Could not generate rabbitClient for non existent cluster: " + err.Error())
		return reconcile.Result{RequeueAfter: 10 * time.Second}, err
	}
	if errors.Is(err, rabbitmqclient.ErrResourceNotAllowed) {
		logger.Info("Could not create resource: " + err.Error())
		object.SetStatusConditions([]topology.Condition{topology.NotReady(rabbitmqclient.ErrResourceNotAllowed.Error(), r.getStatusConditions(object))})
		if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.Status().Update(ctx, object)
		}); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "object", object.GetName())
		}
		return ctrl.Result{}, nil
	}

	// set status condition and publish event for any other error
	logger.Error(err, failedParseClusterRef)
	msg := fmt.Sprintf("%s: %s", failedParseClusterRef, err.Error())
	r.Recorder.Eventf(object, nil, corev1.EventTypeWarning, "FailedCreateOrUpdate", updateEventAction, msg)
	object.SetStatusConditions([]topology.Condition{topology.NotReady(msg, r.getStatusConditions(object))})
	if writerErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, object)
	}); writerErr != nil {
		logger.Error(writerErr, failedStatusUpdate, "object", object.GetName())
	}
	return ctrl.Result{}, err
}

func (r *TopologyReconciler) setObservedGeneration(obj topology.TopologyResource) {
	status := r.getTopLevelField(obj, "Status")
	if status == nil {
		return
	}
	statusValue := reflect.ValueOf(status).Elem()
	if !statusValue.IsValid() {
		return
	}
	observedGenerationValue := statusValue.FieldByName("ObservedGeneration")
	if observedGenerationValue.Kind() != reflect.Int64 || !observedGenerationValue.CanSet() {
		return
	}
	generation := obj.GetGeneration()
	observedGenerationValue.SetInt(generation)
}

func (r *TopologyReconciler) getStatusConditions(obj topology.TopologyResource) []topology.Condition {
	status := r.getTopLevelField(obj, "Status")
	if status == nil {
		return nil
	}
	statusValue := reflect.ValueOf(status).Elem()
	conditionsValue := statusValue.FieldByName("Conditions")
	if !conditionsValue.IsValid() || conditionsValue.IsZero() {
		return nil
	}
	conditions, ok := conditionsValue.Interface().([]topology.Condition)
	if !ok {
		return nil
	}
	return conditions
}

func (r *TopologyReconciler) getTopLevelField(obj topology.TopologyResource, path string) any {
	if obj == nil {
		return nil
	}
	value := reflect.ValueOf(obj).Elem().FieldByName(path)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if !value.IsValid() || !value.CanAddr() {
		return nil
	}
	return value.Addr().Interface()
}

func (r *TopologyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if len(r.WatchTypes) == 0 {
		return ctrl.NewControllerManagedBy(mgr).
			For(r.Type).
			WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
			Complete(r)
	}
	builder := ctrl.NewControllerManagedBy(mgr).
		For(r.Type).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles})
	for _, t := range r.WatchTypes {
		if err := mgr.GetFieldIndexer().IndexField(context.Background(), t, ownerKey, addResourceToIndex); err != nil {
			return err
		}
		builder = builder.Owns(t)
	}
	return builder.Complete(r)
}

func addResourceToIndex(rawObj client.Object) []string {
	switch resourceObject := rawObj.(type) {
	case *corev1.Secret:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	default:
		return nil
	}
}

func validateAndGetOwner(owner *metav1.OwnerReference) []string {
	if owner == nil {
		return nil
	}
	if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
		return nil
	}
	return []string{owner.Name}
}
