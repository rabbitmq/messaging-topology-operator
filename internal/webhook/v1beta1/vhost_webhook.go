package v1beta1

import (
	"context"
	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type VhostCustomValidator struct{}

// SetupVhostWebhookWithManager registers the webhook for Vhost in the manager.
func SetupVhostWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.Vhost{}).
		WithValidator(&VhostCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-vhost,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=vhosts,versions=v1beta1,name=vvhost.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate
//
// Either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided but not both
func (v *VhostCustomValidator) ValidateCreate(_ context.Context, vhost *rabbitmqcomv1beta1.Vhost) (warnings admission.Warnings, err error) {
	return vhost.Spec.RabbitmqClusterReference.ValidateOnCreate(vhost.GroupResource(), vhost.Name)
}

// ValidateUpdate returns error type 'forbidden' for updates on vhost name and rabbitmqClusterReference
func (v *VhostCustomValidator) ValidateUpdate(_ context.Context, oldVhost, newVhost *rabbitmqcomv1beta1.Vhost) (warnings admission.Warnings, err error) {
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

func (v *VhostCustomValidator) ValidateDelete(_ context.Context, obj *rabbitmqcomv1beta1.Vhost) (warnings admission.Warnings, err error) {
	return nil, nil
}
