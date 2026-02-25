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
type UserCustomValidator struct{}

// SetupUserWebhookWithManager registers the webhook for User in the manager.
func SetupUserWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.User{}).
		WithValidator(&UserCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-user,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=users,versions=v1beta1,name=vuser.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate - either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided but not both
func (v *UserCustomValidator) ValidateCreate(_ context.Context, user *rabbitmqcomv1beta1.User) (warnings admission.Warnings, err error) {
	return user.Spec.RabbitmqClusterReference.ValidateOnCreate(user.GroupResource(), user.Name)
}

// ValidateUpdate returns error type 'forbidden' for updates on rabbitmqClusterReference
func (v *UserCustomValidator) ValidateUpdate(_ context.Context, oldUser, newUser *rabbitmqcomv1beta1.User) (warnings admission.Warnings, err error) {
	if !oldUser.Spec.RabbitmqClusterReference.Matches(&newUser.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newUser.GroupResource(), newUser.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), "update on rabbitmqClusterReference is forbidden"))
	}
	return nil, nil
}

func (v *UserCustomValidator) ValidateDelete(_ context.Context, obj *rabbitmqcomv1beta1.User) (warnings admission.Warnings, err error) {
	return nil, nil
}
