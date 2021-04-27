package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (s *SchemaReplication) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-schemareplication,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=schemareplications,versions=v1beta1,name=vschemareplication.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &SchemaReplication{}

// no validation on create
func (s *SchemaReplication) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (s *SchemaReplication) ValidateUpdate(old runtime.Object) error {
	oldReplication, ok := old.(*SchemaReplication)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a schema replication type but got a %T", old))
	}

	if s.Spec.RabbitmqClusterReference != oldReplication.Spec.RabbitmqClusterReference {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), "update on rabbitmqClusterReference is forbidden"))
	}
	return nil
}

// no validation on delete
func (s *SchemaReplication) ValidateDelete() error {
	return nil
}
