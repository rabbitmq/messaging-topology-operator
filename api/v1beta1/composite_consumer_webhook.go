package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (s *CompositeConsumer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-compositeconsumer,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=compositeconsumer,versions=v1beta1,name=vcompositeconsumer.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &CompositeConsumer{}

// no validation on create
func (s *CompositeConsumer) ValidateCreate() error {
	return nil
}

// returns error type 'forbidden' for updates on superstream name and rabbitmqClusterReference
func (s *CompositeConsumer) ValidateUpdate(old runtime.Object) error {
	oldCompositeConsumer, ok := old.(*CompositeConsumer)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a superstream but got a %T", old))
	}

	detailMsg := "updates on superStreamReference are forbidden"

	if s.Spec.SuperStreamReference != oldCompositeConsumer.Spec.SuperStreamReference {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "superStreamReference"), detailMsg))
	}
	return nil
}

// ValidateDelete no validation on delete
func (s *CompositeConsumer) ValidateDelete() error {
	return nil
}
