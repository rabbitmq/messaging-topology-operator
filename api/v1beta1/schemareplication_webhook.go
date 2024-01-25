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

func (s *SchemaReplication) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-schemareplication,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=schemareplications,versions=v1beta1,name=vschemareplication.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.CustomValidator = &SchemaReplication{}

// ValidateCreate - either secretBackend.vault.secretPath or upstreamSecret must
// be provided but not both. Either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided but not both.
func (s *SchemaReplication) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	sch, ok := obj.(*SchemaReplication)
	if !ok {
		return nil, fmt.Errorf("expected a schema replication type but got a %T", obj)
	}
	err = sch.validateSecret()
	if err != nil {
		return nil, err
	}

	return nil, s.Spec.RabbitmqClusterReference.validate(sch.RabbitReference())
}

// ValidateUpdate - either secretBackend.vault.secretPath or upstreamSecret must
// be provided but not both.
func (s *SchemaReplication) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldReplication, ok := oldObj.(*SchemaReplication)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a schema replication type but got a %T", oldObj))
	}

	newReplication, ok := newObj.(*SchemaReplication)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a schema replication type but got a %T", newObj))
	}

	if !oldReplication.Spec.RabbitmqClusterReference.Matches(&newReplication.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newReplication.GroupResource(), newReplication.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), "update on rabbitmqClusterReference is forbidden"))
	}
	return nil, newReplication.validateSecret()
}

// ValidateDelete no validation on delete
func (s *SchemaReplication) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
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
