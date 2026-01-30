package v1beta1

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type VhostValidator struct{}

func (v *Vhost) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var vhostValidator VhostValidator
	return ctrl.NewWebhookManagedBy(mgr, &Vhost{}).
		WithValidator(vhostValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-vhost,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=vhosts,versions=v1beta1,name=vvhost.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate
//
// Either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided but not both
func (v VhostValidator) ValidateCreate(_ context.Context, vhost *Vhost) (warnings admission.Warnings, err error) {
	return nil, vhost.Spec.RabbitmqClusterReference.validate(vhost.RabbitReference())
}

// ValidateUpdate returns error type 'forbidden' for updates on vhost name and rabbitmqClusterReference
func (v VhostValidator) ValidateUpdate(_ context.Context, oldVhost, newVhost *Vhost) (warnings admission.Warnings, err error) {
	const detailMsg = "updates on name and rabbitmqClusterReference are all forbidden"
	if newVhost.Spec.Name != oldVhost.Spec.Name {
		return nil, apierrors.NewForbidden(newVhost.GroupResource(), newVhost.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if !oldVhost.Spec.RabbitmqClusterReference.Matches(&newVhost.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newVhost.GroupResource(), newVhost.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	return nil, nil
}

func (v VhostValidator) ValidateDelete(_ context.Context, obj *Vhost) (warnings admission.Warnings, err error) {
	return nil, nil
}
