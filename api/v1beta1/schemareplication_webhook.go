package v1beta1

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implememts admission.Validator
type SchemaReplicationValidator struct{}

func (s *SchemaReplication) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var schemaReplicationValidator SchemaReplicationValidator
	return ctrl.NewWebhookManagedBy(mgr, &SchemaReplication{}).
		WithValidator(schemaReplicationValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-schemareplication,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=schemareplications,versions=v1beta1,name=vschemareplication.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate - either secretBackend.vault.secretPath or upstreamSecret must
// be provided but not both. Either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided but not both.
func (srv SchemaReplicationValidator) ValidateCreate(_ context.Context, sch *SchemaReplication) (warnings admission.Warnings, err error) {
	err = sch.validateSecret()
	if err != nil {
		return nil, err
	}

	return nil, sch.Spec.RabbitmqClusterReference.validate(sch.RabbitReference())
}

// ValidateUpdate - either secretBackend.vault.secretPath or upstreamSecret must
// be provided but not both.
func (srv SchemaReplicationValidator) ValidateUpdate(_ context.Context, oldReplication, newReplication *SchemaReplication) (warnings admission.Warnings, err error) {
	if !oldReplication.Spec.RabbitmqClusterReference.Matches(&newReplication.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newReplication.GroupResource(), newReplication.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), "update on rabbitmqClusterReference is forbidden"))
	}
	return nil, newReplication.validateSecret()
}

// ValidateDelete no validation on delete
func (srv SchemaReplicationValidator) ValidateDelete(_ context.Context, _ *SchemaReplication) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (s *SchemaReplication) validateSecret() error {
	if s.Spec.UpstreamSecret != nil &&
		s.Spec.UpstreamSecret.Name != "" &&
		s.Spec.SecretBackend.Vault != nil &&
		s.Spec.SecretBackend.Vault.SecretPath != "" {
		return field.Forbidden(field.NewPath("spec"), "do not provide both secretBackend.vault.secretPath and upstreamSecret")
	}

	if (s.Spec.UpstreamSecret == nil || s.Spec.UpstreamSecret.Name == "") &&
		(s.Spec.SecretBackend.Vault == nil || s.Spec.SecretBackend.Vault.SecretPath == "") {
		return field.Forbidden(field.NewPath("spec"), "must provide either secretBackend.vault.secretPath or upstreamSecret")
	}
	return nil
}
