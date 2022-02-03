package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *Vhost) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-vhost,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=vhosts,versions=v1beta1,name=vvhost.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Vhost{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (v *Vhost) ValidateCreate() error {
	if v.Spec.RabbitmqClusterReference.Name != "" && v.Spec.RabbitmqClusterReference.ConnectionSecret != nil {
		return apierrors.NewForbidden(v.GroupResource(), v.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"),
				"do not provide both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret"))
	}

	if v.Spec.RabbitmqClusterReference.Name == "" && v.Spec.RabbitmqClusterReference.ConnectionSecret == nil {
		return apierrors.NewForbidden(v.GroupResource(), v.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"),
				"must provide either spec.rabbitmqClusterReference.name or spec.rabbitmqClusterReference.connectionSecret"))
	}
	return nil
}

// ValidateUpdate returns error type 'forbidden' for updates on vhost name and rabbitmqClusterReference
// vhost.spec.tracing can be updated
func (v *Vhost) ValidateUpdate(old runtime.Object) error {
	oldVhost, ok := old.(*Vhost)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a vhost but got a %T", old))
	}

	detailMsg := "updates on name and rabbitmqClusterReference are all forbidden"
	if v.Spec.Name != oldVhost.Spec.Name {
		return apierrors.NewForbidden(v.GroupResource(), v.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if oldVhost.Spec.RabbitmqClusterReference.hasChange(&v.Spec.RabbitmqClusterReference) {
		return apierrors.NewForbidden(v.GroupResource(), v.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	return nil
}

// no validation on delete
func (v *Vhost) ValidateDelete() error {
	return nil
}
