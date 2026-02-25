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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// SetupShovelWebhookWithManager registers the webhook for Shovel in the manager.
func SetupShovelWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.Shovel{}).
		WithValidator(&ShovelCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-shovel,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=shovels,verbs=create;update,versions=v1beta1,name=vshovel-v1beta1.kb.io,admissionReviewVersions=v1

// ShovelCustomValidator struct is responsible for validating the Shovel resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ShovelCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Shovel.
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided, but not both
func (v *ShovelCustomValidator) ValidateCreate(_ context.Context, obj *rabbitmqcomv1beta1.Shovel) (admission.Warnings, error) {
	if err := amqp10Validate(obj); err != nil {
		return nil, err
	}

	return obj.Spec.RabbitmqClusterReference.ValidateOnCreate(obj.GroupResource(), obj.Name)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Shovel.
func (v *ShovelCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *rabbitmqcomv1beta1.Shovel) (admission.Warnings, error) {
	if err := amqp10Validate(newObj); err != nil {
		return nil, err
	}

	const detailMsg = "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if newObj.Spec.Name != oldObj.Spec.Name {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if newObj.Spec.Vhost != oldObj.Spec.Vhost {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldObj.Spec.RabbitmqClusterReference.Matches(&newObj.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Shovel.
func (v *ShovelCustomValidator) ValidateDelete(_ context.Context, obj *rabbitmqcomv1beta1.Shovel) (admission.Warnings, error) {
	return nil, nil
}

func amqp10Validate(s *rabbitmqcomv1beta1.Shovel) error {
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
