package v1beta1

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type FederationValidator struct{}

func (f *Federation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var federationValidator FederationValidator
	return ctrl.NewWebhookManagedBy(mgr, &Federation{}).
		WithValidator(federationValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-federation,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=federations,versions=v1beta1,name=vfederation.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (fv FederationValidator) ValidateCreate(_ context.Context, fed *Federation) (warnings admission.Warnings, err error) {
	return nil, fed.Spec.RabbitmqClusterReference.validate(fed.RabbitReference())
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (fv FederationValidator) ValidateUpdate(ctx context.Context, oldFederation, newFederation *Federation) (warnings admission.Warnings, err error) {
	const detailMsg = "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if newFederation.Spec.Name != oldFederation.Spec.Name {
		return nil, apierrors.NewForbidden(newFederation.GroupResource(), newFederation.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if newFederation.Spec.Vhost != oldFederation.Spec.Vhost {
		return nil, apierrors.NewForbidden(newFederation.GroupResource(), newFederation.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldFederation.Spec.RabbitmqClusterReference.Matches(&newFederation.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newFederation.GroupResource(), newFederation.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

func (fv FederationValidator) ValidateDelete(_ context.Context, _ *Federation) (warnings admission.Warnings, err error) {
	return nil, nil
}
