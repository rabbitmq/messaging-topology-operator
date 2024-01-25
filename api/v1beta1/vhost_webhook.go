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

func (v *Vhost) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(v).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-vhost,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=vhosts,versions=v1beta1,name=vvhost.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.CustomValidator = &Vhost{}

// ValidateCreate
//
// Either rabbitmqClusterReference.name or
// rabbitmqClusterReference.connectionSecret must be provided but not both
func (v *Vhost) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	vhost, ok := obj.(*Vhost)
	if !ok {
		return nil, fmt.Errorf("expected a RabbitMQ vhost but got a %T", obj)
	}
	return nil, v.Spec.RabbitmqClusterReference.validate(vhost.RabbitReference())
}

// ValidateUpdate returns error type 'forbidden' for updates on vhost name and rabbitmqClusterReference
func (v *Vhost) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldVhost, ok := oldObj.(*Vhost)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a vhost but got a %T", oldObj))
	}

	newVhost, ok := newObj.(*Vhost)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a vhost but got a %T", newObj))
	}

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

func (v *Vhost) ValidateDelete(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
