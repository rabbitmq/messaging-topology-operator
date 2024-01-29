package v1beta1

import (
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
		For(p).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-operatorpolicy,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=operatorpolicies,versions=v1beta1,name=voperatorpolicy.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &OperatorPolicy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (p *OperatorPolicy) ValidateCreate() (admission.Warnings, error) {
	return p.Spec.RabbitmqClusterReference.ValidateOnCreate(p.GroupResource(), p.Name)
}

// ValidateUpdate returns error type 'forbidden' for updates on operator policy name, vhost and rabbitmqClusterReference
func (p *OperatorPolicy) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldOperatorPolicy, ok := old.(*OperatorPolicy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an operator policy but got a %T", old))
	}

	detailMsg := "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if p.Spec.Name != oldOperatorPolicy.Spec.Name {
		return nil, apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if p.Spec.Vhost != oldOperatorPolicy.Spec.Vhost {
		return nil, apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldOperatorPolicy.Spec.RabbitmqClusterReference.Matches(&p.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

func (p *OperatorPolicy) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
