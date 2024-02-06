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

func (p *OperatorPolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(p).
		For(p).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-operatorpolicy,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=operatorpolicies,versions=v1beta1,name=voperatorpolicy.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.CustomValidator = &OperatorPolicy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (p *OperatorPolicy) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	op, ok := obj.(*OperatorPolicy)
	if !ok {
		return nil, fmt.Errorf("expected an operator policy but got %T", obj)
	}
	return nil, p.Spec.RabbitmqClusterReference.validate(op.RabbitReference())
}

// ValidateUpdate returns error type 'forbidden' for updates on operator policy name, vhost and rabbitmqClusterReference
func (p *OperatorPolicy) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldOperatorPolicy, ok := oldObj.(*OperatorPolicy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an operator policy but got a %T", oldObj))
	}

	newOperatorPolicy, ok := newObj.(*OperatorPolicy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an operator policy but got a %T", newObj))
	}

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

func (p *OperatorPolicy) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
