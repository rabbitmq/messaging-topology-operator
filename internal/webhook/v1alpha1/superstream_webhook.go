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

package v1alpha1

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rabbitmqcomv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

// SetupSuperStreamWebhookWithManager registers the webhook for SuperStream in the manager.
func SetupSuperStreamWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1alpha1.SuperStream{}).
		WithValidator(&SuperStreamCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rabbitmq-com-v1alpha1-superstream,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=superstreams,verbs=create;update,versions=v1alpha1,name=vsuperstream-v1alpha1.kb.io,admissionReviewVersions=v1

type SuperStreamCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type SuperStream.
func (v *SuperStreamCustomValidator) ValidateCreate(_ context.Context, obj *rabbitmqcomv1alpha1.SuperStream) (admission.Warnings, error) {
	return obj.Spec.RabbitmqClusterReference.ValidateOnCreate(obj.GroupResource(), obj.Name)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type SuperStream.
func (v *SuperStreamCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *rabbitmqcomv1alpha1.SuperStream) (admission.Warnings, error) {
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

	if !routingKeyUpdatePermitted(oldObj.Spec.RoutingKeys, newObj.Spec.RoutingKeys) {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "routingKeys"), "updates may only add to the existing list of routing keys"))
	}

	if newObj.Spec.Partitions < oldObj.Spec.Partitions {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "partitions"), "updates may only increase the partition count, and may not decrease it"))
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type SuperStream.
func (v *SuperStreamCustomValidator) ValidateDelete(_ context.Context, obj *rabbitmqcomv1alpha1.SuperStream) (admission.Warnings, error) {
	return nil, nil
}

// routingKeyUpdatePermitted allows updates only if adding additional keys at the end of the list of keys
func routingKeyUpdatePermitted(old, new []string) bool {
	if len(old) == 0 && len(new) != 0 {
		return false
	}
	for i := 0; i < len(old); i++ {
		if old[i] != new[i] {
			return false
		}
	}
	return true
}
