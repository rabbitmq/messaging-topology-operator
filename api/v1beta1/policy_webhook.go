package v1beta1

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type PolicyValidator struct{}

func (p *Policy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var policyValidator PolicyValidator
	return ctrl.NewWebhookManagedBy(mgr, &Policy{}).
		WithValidator(policyValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-policy,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=policies,versions=v1beta1,name=vpolicy.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (pv PolicyValidator) ValidateCreate(_ context.Context, policy *Policy) (warnings admission.Warnings, err error) {
	return nil, policy.Spec.RabbitmqClusterReference.validate(policy.RabbitReference())
}

// ValidateUpdate returns error type 'forbidden' for updates on policy name, vhost and rabbitmqClusterReference
func (pv PolicyValidator) ValidateUpdate(_ context.Context, oldPolicy, newPolicy *Policy) (warnings admission.Warnings, err error) {
	const detailMsg = "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if newPolicy.Spec.Name != oldPolicy.Spec.Name {
		return nil, apierrors.NewForbidden(newPolicy.GroupResource(), newPolicy.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if newPolicy.Spec.Vhost != oldPolicy.Spec.Vhost {
		return nil, apierrors.NewForbidden(newPolicy.GroupResource(), newPolicy.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldPolicy.Spec.RabbitmqClusterReference.Matches(&newPolicy.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newPolicy.GroupResource(), newPolicy.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

func (pv PolicyValidator) ValidateDelete(_ context.Context, _ *Policy) (warnings admission.Warnings, err error) {
	return nil, nil
}
