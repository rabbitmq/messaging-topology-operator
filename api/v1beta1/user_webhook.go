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

func (u *User) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(u).
		For(u).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-user,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=users,versions=v1beta1,name=vuser.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.CustomValidator = &User{}

// ValidateCreate - either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided but not both
func (u *User) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	user, ok := obj.(*User)
	if !ok {
		return nil, fmt.Errorf("expected a RabbitMQ user but got a %T", obj)
	}
	return nil, u.Spec.RabbitmqClusterReference.validate(user.RabbitReference())
}

// ValidateUpdate returns error type 'forbidden' for updates on rabbitmqClusterReference
func (u *User) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldUser, ok := oldObj.(*User)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a user but got a %T", oldObj))
	}

	newUser, ok := newObj.(*User)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a user but got a %T", newObj))
	}

	if !oldUser.Spec.RabbitmqClusterReference.Matches(&newUser.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newUser.GroupResource(), newUser.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), "update on rabbitmqClusterReference is forbidden"))
	}
	return nil, nil
}

func (u *User) ValidateDelete(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
