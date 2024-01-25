package v1beta1

import (
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (b *Binding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(b).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-binding,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=bindings,versions=v1beta1,name=vbinding.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.CustomValidator = &Binding{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (b *Binding) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	bi, ok := obj.(*Binding)
	if !ok {
		return nil, fmt.Errorf("expected a RabbitMQ Binding, but got %T", obj)
	}
	return nil, b.Spec.RabbitmqClusterReference.validate(bi.RabbitReference())
}

func (b *Binding) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldBinding, ok := oldObj.(*Binding)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a binding but got a %T", oldObj))
	}

	newBinding, ok := newObj.(*Binding)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a binding but got a %T", oldObj))
	}

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

func (b *Binding) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
