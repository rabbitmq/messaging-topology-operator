package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (u *User) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(u).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-user,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=users,versions=v1beta1,name=vuser.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &User{}

// no validation on create
func (u *User) ValidateCreate() error {
	return nil
}

// returns error type 'forbidden' for updates on rabbitmqClusterReference
// user.spec.tags can be updated
func (u *User) ValidateUpdate(old runtime.Object) error {
	oldUser, ok := old.(*User)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a user but got a %T", old))
	}

	if u.Spec.RabbitmqClusterReference != oldUser.Spec.RabbitmqClusterReference {
		return apierrors.NewForbidden(u.GroupResource(), u.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), "update on rabbitmqClusterReference is forbidden"))
	}
	return nil
}

// no validation on delete
func (u *User) ValidateDelete() error {
	return nil
}
