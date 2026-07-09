package v1beta1

import (
	"context"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Defaulter[*rabbitmqcomv1beta1.Queue]
type QueueCustomDefaulter struct{}

var _ admission.Defaulter[*rabbitmqcomv1beta1.Queue] = &QueueCustomDefaulter{}

// SetupQueueDefaulterWebhookWithManager registers the defaulting webhook for Queue in the manager.
func SetupQueueDefaulterWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.Queue{}).
		WithDefaulter(&QueueCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:verbs=create,path=/mutate-rabbitmq-com-v1beta1-queue,mutating=true,failurePolicy=fail,groups=rabbitmq.com,resources=queues,versions=v1beta1,name=mqueue.kb.io,sideEffects=none,admissionReviewVersions=v1

func (d *QueueCustomDefaulter) Default(_ context.Context, obj *rabbitmqcomv1beta1.Queue) error {
	if obj.Spec.Durable == nil {
		obj.Spec.Durable = new(true)
	}
	return nil
}
