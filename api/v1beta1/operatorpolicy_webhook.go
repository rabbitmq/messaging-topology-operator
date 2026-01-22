package v1beta1

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type OperatorPolicyValidator struct{}

func (p *OperatorPolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var operatorPolicyValidator OperatorPolicyValidator
	return ctrl.NewWebhookManagedBy(mgr, &OperatorPolicy{}).
		WithValidator(operatorPolicyValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-operatorpolicy,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=operatorpolicies,versions=v1beta1,name=voperatorpolicy.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (opv OperatorPolicyValidator) ValidateCreate(_ context.Context, op *OperatorPolicy) (warnings admission.Warnings, err error) {
	return nil, op.Spec.RabbitmqClusterReference.validate(op.RabbitReference())
}

// ValidateUpdate returns error type 'forbidden' for updates on operator policy name, vhost and rabbitmqClusterReference
func (opv OperatorPolicyValidator) ValidateUpdate(ctx context.Context, oldOperatorPolicy, newOperatorPolicy *OperatorPolicy) (warnings admission.Warnings, err error) {
	const detailMsg = "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if newOperatorPolicy.Spec.Name != oldOperatorPolicy.Spec.Name {
		return nil, apierrors.NewForbidden(newOperatorPolicy.GroupResource(), newOperatorPolicy.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if newOperatorPolicy.Spec.Vhost != oldOperatorPolicy.Spec.Vhost {
		return nil, apierrors.NewForbidden(newOperatorPolicy.GroupResource(), newOperatorPolicy.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldOperatorPolicy.Spec.RabbitmqClusterReference.Matches(&newOperatorPolicy.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newOperatorPolicy.GroupResource(), newOperatorPolicy.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

func (opv OperatorPolicyValidator) ValidateDelete(_ context.Context, _ *OperatorPolicy) (warnings admission.Warnings, err error) {
	return nil, nil
}
