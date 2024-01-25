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

func (f *Federation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(f).
		Complete()
}

var _ webhook.CustomValidator = &Federation{}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-federation,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=federations,versions=v1beta1,name=vfederation.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (f *Federation) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	//TODO implement me
	fed, ok := obj.(*Federation)
	if !ok {
		return nil, fmt.Errorf("expected a RabbitMQ Federation, but got %T", obj)
	}
	return nil, f.Spec.RabbitmqClusterReference.validate(fed.RabbitReference())
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (f *Federation) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldFederation, ok := oldObj.(*Federation)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a federation but got a %T", oldObj))
	}

	newFederation, ok := newObj.(*Federation)
	if !ok {
		return nil, fmt.Errorf("expected a RabbitMQ Federation, but got %T", newObj)
	}

	const detailMsg = "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if newFederation.Spec.Name != oldFederation.Spec.Name {
		return nil, apierrors.NewForbidden(newFederation.GroupResource(), newFederation.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if newFederation.Spec.Vhost != oldFederation.Spec.Vhost {
		return nil, apierrors.NewForbidden(newFederation.GroupResource(), newFederation.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldFederation.Spec.RabbitmqClusterReference.Matches(&newFederation.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newFederation.GroupResource(), newFederation.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

func (f *Federation) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
