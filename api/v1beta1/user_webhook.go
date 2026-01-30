package v1beta1

import (
	"context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type UserValidator struct{}

func (u *User) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var userValidator UserValidator
	return ctrl.NewWebhookManagedBy(mgr, &User{}).
		WithValidator(userValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-user,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=users,versions=v1beta1,name=vuser.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate - either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided but not both
func (uv UserValidator) ValidateCreate(_ context.Context, user *User) (warnings admission.Warnings, err error) {
	return nil, user.Spec.RabbitmqClusterReference.validate(user.RabbitReference())
}

// ValidateUpdate returns error type 'forbidden' for updates on rabbitmqClusterReference
func (uv UserValidator) ValidateUpdate(_ context.Context, oldUser, newUser *User) (warnings admission.Warnings, err error) {
	if !oldUser.Spec.RabbitmqClusterReference.Matches(&newUser.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newUser.GroupResource(), newUser.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), "update on rabbitmqClusterReference is forbidden"))
	}
	return nil, nil
}

func (uv UserValidator) ValidateDelete(_ context.Context, obj *User) (warnings admission.Warnings, err error) {
	return nil, nil
}
