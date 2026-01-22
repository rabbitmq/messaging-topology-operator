package v1beta1

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type ShovelValidator struct{}

func (s *Shovel) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var shovelValidator ShovelValidator
	return ctrl.NewWebhookManagedBy(mgr, &Shovel{}).
		WithValidator(shovelValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-shovel,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=shovels,versions=v1beta1,name=vshovel.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate - either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided, but not both
func (sv ShovelValidator) ValidateCreate(_ context.Context, shovel *Shovel) (warnings admission.Warnings, err error) {
	if err := shovel.amqp10Validate(); err != nil {
		return nil, err
	}

	return nil, shovel.Spec.RabbitmqClusterReference.validate(shovel.RabbitReference())
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (sv ShovelValidator) ValidateUpdate(_ context.Context, oldShovel, newShovel *Shovel) (warnings admission.Warnings, err error) {
	if err := newShovel.amqp10Validate(); err != nil {
		return nil, err
	}

	const detailMsg = "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if newShovel.Spec.Name != oldShovel.Spec.Name {
		return nil, apierrors.NewForbidden(newShovel.GroupResource(), newShovel.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if newShovel.Spec.Vhost != oldShovel.Spec.Vhost {
		return nil, apierrors.NewForbidden(newShovel.GroupResource(), newShovel.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldShovel.Spec.RabbitmqClusterReference.Matches(&newShovel.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newShovel.GroupResource(), newShovel.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

func (sv ShovelValidator) ValidateDelete(_ context.Context, obj *Shovel) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (s *Shovel) amqp10Validate() error {
	if s.Spec.SourceProtocol == "amqp10" && s.Spec.SourceAddress == "" {
		return field.Required(field.NewPath("spec", "srcAddress"),
			"must specify spec.srcAddress when spec.srcProtocol is amqp10")
	}
	if s.Spec.DestinationProtocol == "amqp10" && s.Spec.DestinationAddress == "" {
		return field.Required(field.NewPath("spec", "destAddress"),
			"must specify spec.destAddress when spec.destProtocol is amqp10")
	}
	return nil
}
