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

func (p *Policy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(p).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-policy,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=policies,versions=v1beta1,name=vpolicy.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.CustomValidator = &Policy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (p *Policy) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	policy, ok := obj.(*Policy)
	if !ok {
		return nil, fmt.Errorf("expected a RabbitMQ policy but got a %T", obj)
	}
	return nil, p.Spec.RabbitmqClusterReference.validate(policy.RabbitReference())
}

// ValidateUpdate returns error type 'forbidden' for updates on policy name, vhost and rabbitmqClusterReference
func (p *Policy) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldPolicy, ok := oldObj.(*Policy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a policy but got a %T", oldObj))
	}

	newPolicy, ok := newObj.(*Policy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a policy but got a %T", newObj))
	}

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

func (p *Policy) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
