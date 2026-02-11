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

// SetupSchemaReplicationWebhookWithManager registers the webhook for SchemaReplication in the manager.
func SetupSchemaReplicationWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.SchemaReplication{}).
		WithValidator(&SchemaReplicationCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-schemareplication,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=schemareplications,verbs=create;update,versions=v1beta1,name=vschemareplication-v1beta1.kb.io,admissionReviewVersions=v1

// SchemaReplicationCustomValidator struct is responsible for validating the SchemaReplication resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type SchemaReplicationCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type SchemaReplication.
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (v *SchemaReplicationCustomValidator) ValidateCreate(_ context.Context, obj *rabbitmqcomv1beta1.SchemaReplication) (admission.Warnings, error) {
	if err := validateSecret(obj); err != nil {
		return nil, err
	}

	return obj.Spec.RabbitmqClusterReference.ValidateOnCreate(obj.GroupResource(), obj.Name)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type SchemaReplication.
func (v *SchemaReplicationCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *rabbitmqcomv1beta1.SchemaReplication) (admission.Warnings, error) {
	if err := validateSecret(newObj); err != nil {
		return nil, err
	}

	const detailMsg = "update on rabbitmqClusterReference is forbidden"
	if !oldObj.Spec.RabbitmqClusterReference.Matches(&newObj.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type SchemaReplication.
func (v *SchemaReplicationCustomValidator) ValidateDelete(_ context.Context, obj *rabbitmqcomv1beta1.SchemaReplication) (admission.Warnings, error) {
	return nil, nil
}

func validateSecret(sch *rabbitmqcomv1beta1.SchemaReplication) error {
	if sch.Spec.SecretBackend.Vault != nil {
		if sch.Spec.SecretBackend.Vault.SecretPath == "" {
			return field.Required(field.NewPath("spec", "secretBackend", "vault", "secretPath"),
				"must specify secretBackend.vault.secretPath when secretBackend.vault is set")
		}
		// Endpoints is required when using Vault
		if sch.Spec.Endpoints == "" {
			return field.Required(field.NewPath("spec", "endpoints"),
				"endpoints must be set when secretBackend.vault is set")
		}
	}
	return nil
}
