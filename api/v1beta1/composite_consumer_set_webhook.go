package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (s *CompositeConsumerSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-superstream,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=superstreams,versions=v1beta1,name=vsuperstream.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &CompositeConsumerSet{}

// no validation on create
func (s *CompositeConsumerSet) ValidateCreate() error {
	return nil
}

// returns error type 'forbidden' for updates on superstream name and rabbitmqClusterReference
func (s *CompositeConsumerSet) ValidateUpdate(old runtime.Object) error {
	oldCompositeConsumerSet, ok := old.(*CompositeConsumerSet)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a superstream but got a %T", old))
	}

	detailMsg := "updates on superStreamReference are forbidden"

	if s.Spec.SuperStreamReference != oldCompositeConsumerSet.Spec.SuperStreamReference {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "superStreamReference"), detailMsg))
	}
	return nil
}

// ValidateDelete no validation on delete
func (s *CompositeConsumerSet) ValidateDelete() error {
	return nil
}
