package v1beta1

import (
	"context"
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

var _ webhook.CustomValidator = &Shovel{}

// ValidateCreate - either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided, but not both
func (s *Shovel) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	shovel, ok := obj.(*Shovel)
	if !ok {
		return nil, fmt.Errorf("expected a RabbitMQ shovel but got a %T", obj)
	}
	if err := shovel.amqp10Validate(); err != nil {
		return nil, err
	}

	return nil, s.Spec.RabbitmqClusterReference.validate(shovel.RabbitReference())
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (s *Shovel) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	//TODO implement me
	oldShovel, ok := oldObj.(*Shovel)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a shovel but got a %T", oldShovel))
	}

	newShovel, ok := newObj.(*Shovel)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a shovel but got a %T", newObj))
	}

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

func (s *Shovel) ValidateDelete(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
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
