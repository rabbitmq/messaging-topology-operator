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

const exchangeFinalizer = "deletion.finalizers.exchanges.rabbitmq.com"

// ExchangeReconciler reconciles a Exchange object
type ExchangeReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges/status,verbs=get;update;patch

func (r *ExchangeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	exchange := &topologyv1alpha1.Exchange{}
	if err := r.Get(ctx, req.NamespacedName, exchange); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	rabbitClient, err := rabbitholeClient(ctx, r.Client, exchange.Spec.RabbitmqClusterReference)
	if err != nil {
		logger.Error(err, "Failed to generate http rabbitClient")
		return reconcile.Result{}, err
	}

	if !exchange.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.deleteExchange(ctx, rabbitClient, exchange)
	}

	if err := r.addFinalizerIfNeeded(ctx, exchange); err != nil {
		return ctrl.Result{}, err
	}

	spec, err := json.Marshal(exchange.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal exchange spec")
	}

	logger.Info("Start reconciling",
		"spec", string(spec))

	if err := r.declareExchange(ctx, rabbitClient, exchange); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *ExchangeReconciler) declareExchange(ctx context.Context, client *rabbithole.Client, exchange *topologyv1alpha1.Exchange) error {
	logger := ctrl.LoggerFrom(ctx)

	settings, err := internal.GenerateExchangeSettings(exchange)
	if err != nil {
		msg := "failed to generate exchange settings"
		r.Recorder.Event(exchange, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg)
		return err
	}

	if err := validateResponse(client.DeclareExchange(exchange.Spec.Vhost, exchange.Spec.Name, *settings)); err != nil {
		msg := "failed to declare exchange"
		r.Recorder.Event(exchange, corev1.EventTypeWarning, "FailedDeclare", msg)
		logger.Error(err, msg, "exchange", exchange.Spec.Name)
		return err
	}

	logger.Info("Successfully declared exchange", "exchange", exchange.Spec.Name)
	r.Recorder.Event(exchange, corev1.EventTypeNormal, "SuccessfulDeclare", "Successfully declared exchange")
	return nil
}

// addFinalizerIfNeeded adds a deletion finalizer if the Exchange does not have one yet and is not marked for deletion
func (r *ExchangeReconciler) addFinalizerIfNeeded(ctx context.Context, e *topologyv1alpha1.Exchange) error {
	if e.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(e, exchangeFinalizer) {
		controllerutil.AddFinalizer(e, exchangeFinalizer)
		if err := r.Client.Update(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func (r *ExchangeReconciler) deleteExchange(ctx context.Context, client *rabbithole.Client, exchange *topologyv1alpha1.Exchange) error {
	logger := ctrl.LoggerFrom(ctx)

	if err := validateResponse(client.DeleteExchange(exchange.Spec.Vhost, exchange.Spec.Name)); err != nil {
		msg := "failed to delete exchange"
		r.Recorder.Event(exchange, corev1.EventTypeWarning, "FailedDelete", msg)
		logger.Error(err, msg, "exchange", exchange.Spec.Name)
		return err
	}
	return r.removeFinalizer(ctx, exchange)
}

func (r *ExchangeReconciler) removeFinalizer(ctx context.Context, e *topologyv1alpha1.Exchange) error {
	controllerutil.RemoveFinalizer(e, exchangeFinalizer)
	if err := r.Client.Update(ctx, e); err != nil {
		return err
	}
	return nil
}

func (r *ExchangeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1alpha1.Exchange{}).
		Complete(r)
}
