package v1beta1

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type BindingValidator struct{}

func (b *Binding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var bindingValidator BindingValidator
	return ctrl.NewWebhookManagedBy(mgr, &Binding{}).
		WithValidator(bindingValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-binding,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=bindings,versions=v1beta1,name=vbinding.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (bv BindingValidator) ValidateCreate(_ context.Context, bi *Binding) (warnings admission.Warnings, err error) {
	return nil, bi.Spec.RabbitmqClusterReference.validate(bi.RabbitReference())
}

func (bv BindingValidator) ValidateUpdate(_ context.Context, oldBinding, newBinding *Binding) (warnings admission.Warnings, err error) {
	var allErrs field.ErrorList
	const detailMsg = "updates on vhost and rabbitmqClusterReference are all forbidden"

	if newBinding.Spec.Vhost != oldBinding.Spec.Vhost {
		return nil, apierrors.NewForbidden(newBinding.GroupResource(), newBinding.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldBinding.Spec.RabbitmqClusterReference.Matches(&newBinding.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newBinding.GroupResource(), newBinding.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	if newBinding.Spec.Source != oldBinding.Spec.Source {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "source"),
			newBinding.Spec.Source,
			"source cannot be updated",
		))
	}

	if newBinding.Spec.Destination != oldBinding.Spec.Destination {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "destination"),
			newBinding.Spec.Destination,
			"destination cannot be updated",
		))
	}

	if newBinding.Spec.DestinationType != oldBinding.Spec.DestinationType {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "destinationType"),
			newBinding.Spec.DestinationType,
			"destinationType cannot be updated",
		))
	}

	if newBinding.Spec.RoutingKey != oldBinding.Spec.RoutingKey {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "routingKey"),
			newBinding.Spec.RoutingKey,
			"routingKey cannot be updated",
		))
	}

	if !reflect.DeepEqual(newBinding.Spec.Arguments, oldBinding.Spec.Arguments) {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "arguments"),
			newBinding.Spec.Arguments,
			"arguments cannot be updated",
		))
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, allErrs.ToAggregate()
}

func (bv BindingValidator) ValidateDelete(_ context.Context, _ *Binding) (warnings admission.Warnings, err error) {
	return nil, nil
}
