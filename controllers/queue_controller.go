/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const deletionFinalizer = "deletion.finalizers.queues.rabbitmq.com"

// QueueReconciler reconciles a Queue object
type QueueReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get;update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

func (r *QueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	// fetched the q and return if q no longer exists
	q := &topologyv1beta1.Queue{}
	if err := r.Get(ctx, req.NamespacedName, q); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// create rabbitmq http client
	client, err := r.rabbitClient(ctx, q.Spec.RabbitmqClusterReference)
	if err != nil {
		logger.Error(err, "Failed to generate http client")
		return reconcile.Result{}, err
	}

	// Check if the q has been marked for deletion
	if !q.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteQueue(ctx, client, q)
	}

	if err := r.addFinalizerIfNeeded(ctx, q); err != nil {
		return ctrl.Result{}, err
	}

	queueSpec, err := json.Marshal(q.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal q spec")
	}

	logger.Info("Start reconciling",
		"spec", string(queueSpec))

	if err := r.declareQueue(ctx, client, q); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *QueueReconciler) declareQueue(ctx context.Context, client *rabbithole.Client, q *topologyv1beta1.Queue) error {
	logger := ctrl.LoggerFrom(ctx)

	queueSettings, err := internal.GenerateQueueSettings(q)
	if err != nil {
		msg := "failed to declare queue"
		r.Recorder.Event(q, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	//queueSettings := make(map[string]interface{})
	//if q.Spec.Arguments != nil {
	//	if err := json.Unmarshal(q.Spec.Arguments.Raw, &queueSettings); err != nil {
	//		msg := "failed to unmarshall queue arguments"
	//		r.Recorder.Event(q, corev1.EventTypeWarning, "FailedDeclare", msg)
	//		logger.Error(err, msg)
	//		return err
	//	}
	//}
	//
	//// bug in rabbithole; setting queue type in QueueSettings.Type does not have impact
	//if q.Spec.Type != "" {
	//	queueSettings["x-queue-type"] = q.Spec.Type
	//}

	res, err := client.DeclareQueue(q.Spec.Vhost, q.Name, *queueSettings)
	if err != nil {
		msg := "failed to declare queue"
		r.Recorder.Event(q, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	if res.StatusCode >= http.StatusBadRequest {
		msg := "failed to declare queue"
		r.Recorder.Event(q, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(errors.New("FailedDeclare"), "server response", res.Body)
		return fmt.Errorf("response from server: %s", res.Body)
	}

	logger.Info("server response", "response.body", res.Body)
	return nil
}

// addFinalizerIfNeeded adds a deletion finalizer if the Queue does not have one yet and is not marked for deletion
func (r *QueueReconciler) addFinalizerIfNeeded(ctx context.Context, q *topologyv1beta1.Queue) error {
	if q.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(q, deletionFinalizer) {
		controllerutil.AddFinalizer(q, deletionFinalizer)
		if err := r.Client.Update(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func (r *QueueReconciler) deleteQueue(ctx context.Context, client *rabbithole.Client, q *topologyv1beta1.Queue) error {
	client.DeleteQueue(q.Spec.Vhost, q.Name)
	return r.removeFinalizer(ctx, q)
}

func (r *QueueReconciler) removeFinalizer(ctx context.Context, q *topologyv1beta1.Queue) error {
	controllerutil.RemoveFinalizer(q, deletionFinalizer)
	if err := r.Client.Update(ctx, q); err != nil {
		return err
	}
	return nil
}

func (r *QueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1beta1.Queue{}).
		Complete(r)
}
