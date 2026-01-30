/*
Copyright 2026.

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

package v1beta1

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// SetupBindingWebhookWithManager registers the webhook for Binding in the manager.
func SetupBindingWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.Binding{}).
		WithValidator(&BindingCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-binding,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=bindings,verbs=create;update,versions=v1beta1,name=vbinding-v1beta1.kb.io,admissionReviewVersions=v1

// BindingCustomValidator struct is responsible for validating the Binding resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type BindingCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Binding.
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (v *BindingCustomValidator) ValidateCreate(_ context.Context, obj *rabbitmqcomv1beta1.Binding) (admission.Warnings, error) {
	return obj.Spec.RabbitmqClusterReference.ValidateOnCreate(obj.GroupResource(), obj.Name)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Binding.
func (v *BindingCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *rabbitmqcomv1beta1.Binding) (admission.Warnings, error) {
	var allErrs field.ErrorList
	const detailMsg = "updates on vhost and rabbitmqClusterReference are all forbidden"

	if newObj.Spec.Vhost != oldObj.Spec.Vhost {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldObj.Spec.RabbitmqClusterReference.Matches(&newObj.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	if newObj.Spec.Source != oldObj.Spec.Source {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "source"),
			newObj.Spec.Source,
			"source cannot be updated",
		))
	}

	if newObj.Spec.Destination != oldObj.Spec.Destination {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "destination"),
			newObj.Spec.Destination,
			"destination cannot be updated",
		))
	}

	if newObj.Spec.DestinationType != oldObj.Spec.DestinationType {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "destinationType"),
			newObj.Spec.DestinationType,
			"destinationType cannot be updated",
		))
	}

	if newObj.Spec.RoutingKey != oldObj.Spec.RoutingKey {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "routingKey"),
			newObj.Spec.RoutingKey,
			"routingKey cannot be updated",
		))
	}

	if !reflect.DeepEqual(newObj.Spec.Arguments, oldObj.Spec.Arguments) {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "arguments"),
			newObj.Spec.Arguments,
			"arguments cannot be updated",
		))
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, allErrs.ToAggregate()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Binding.
func (v *BindingCustomValidator) ValidateDelete(_ context.Context, obj *rabbitmqcomv1beta1.Binding) (admission.Warnings, error) {
	return nil, nil
}
