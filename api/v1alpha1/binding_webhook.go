package v1alpha1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (b *Binding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(b).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1alpha1-binding,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=bindings,versions=v1alpha1,name=vbinding.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Binding{}

// no validation logic on create
func (b *Binding) ValidateCreate() error {
	return nil
}

// updates on bindings.rabbitmq.com is forbidden
func (b *Binding) ValidateUpdate(old runtime.Object) error {
	oldBinding, ok := old.(*Binding)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a binding but got a %T", old))
	}

	if oldBinding.Spec != b.Spec {
		return apierrors.NewForbidden(
			b.GroupResource(),
			b.Name,
			field.Forbidden(field.NewPath("spec"), "binding.spec is immutable"))
	}
	return nil
}

// no validation logic on delete
func (b *Binding) ValidateDelete() error {
	return nil
}
