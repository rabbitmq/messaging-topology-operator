package v1alpha2

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (p *Permission) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(p).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1alpha2-permission,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=permissions,versions=v1alpha2,name=vpermission.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Permission{}

// no validation on create
func (p *Permission) ValidateCreate() error {
	return nil
}

// do not allow updates on spec.vhost, spec.user, and spec.rabbitmqClusterReference
// updates on spec.permissions are allowed
func (p *Permission) ValidateUpdate(old runtime.Object) error {
	oldPermission, ok := old.(*Permission)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a permission but got a %T", old))
	}

	detailMsg := "updates on user, vhost and rabbitmqClusterReference are all forbidden"
	if p.Spec.User != oldPermission.Spec.User {
		return apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "user"), detailMsg))
	}

	if p.Spec.Vhost != oldPermission.Spec.Vhost {
		return apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if p.Spec.RabbitmqClusterReference != oldPermission.Spec.RabbitmqClusterReference {
		return apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil
}

// no validation on delete
func (p *Permission) ValidateDelete() error {
	return nil
}
