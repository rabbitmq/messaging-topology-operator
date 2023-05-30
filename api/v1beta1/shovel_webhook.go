package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (s *Shovel) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-shovel,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=shovels,versions=v1beta1,name=vshovel.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Shovel{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (s *Shovel) ValidateCreate() (admission.Warnings, error) {
	if err := s.amqp10Validate(); err != nil {
		return nil, err
	}
	return s.Spec.RabbitmqClusterReference.ValidateOnCreate(s.GroupResource(), s.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (s *Shovel) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldShovel, ok := old.(*Shovel)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a shovel but got a %T", oldShovel))
	}

	if err := s.amqp10Validate(); err != nil {
		return nil, err
	}

	detailMsg := "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if s.Spec.Name != oldShovel.Spec.Name {
		return nil, apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if s.Spec.Vhost != oldShovel.Spec.Vhost {
		return nil, apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldShovel.Spec.RabbitmqClusterReference.Matches(&s.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

func (s *Shovel) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (s *Shovel) amqp10Validate() error {
	var errorList field.ErrorList
	if s.Spec.SourceProtocol == "amqp10" && s.Spec.SourceAddress == "" {
		errorList = append(errorList, field.Required(field.NewPath("spec", "srcAddress"),
			"must specify spec.srcAddress when spec.srcProtocol is amqp10"))
		return apierrors.NewInvalid(GroupVersion.WithKind("Shovel").GroupKind(), s.Name, errorList)
	}
	if s.Spec.DestinationProtocol == "amqp10" && s.Spec.DestinationAddress == "" {
		errorList = append(errorList, field.Required(field.NewPath("spec", "destAddress"),
			"must specify spec.destAddress when spec.destProtocol is amqp10"))
		return apierrors.NewInvalid(GroupVersion.WithKind("Shovel").GroupKind(), s.Name, errorList)
	}
	return nil
}
