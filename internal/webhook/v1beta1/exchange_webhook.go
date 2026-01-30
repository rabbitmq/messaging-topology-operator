package v1beta1

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// Implements admission.Validator
type ExchangeCustomValidator struct{}

// SetupExchangeWebhookWithManager registers the webhook for Exchange in the manager.
func SetupExchangeWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.Exchange{}).
		WithValidator(&ExchangeCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-exchange,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=exchanges,versions=v1beta1,name=vexchange.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (v *ExchangeCustomValidator) ValidateCreate(_ context.Context, ex *rabbitmqcomv1beta1.Exchange) (warnings admission.Warnings, err error) {
	return ex.Spec.RabbitmqClusterReference.ValidateOnCreate(ex.GroupResource(), ex.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
// returns error type 'forbidden' for updates that the controller chooses to disallow: exchange name/vhost/rabbitmqClusterReference
// returns error type 'invalid' for updates that will be rejected by rabbitmq server: exchange types/autoDelete/durable
// exchange.spec.arguments can be updated
func (v *ExchangeCustomValidator) ValidateUpdate(_ context.Context, oldExchange, newExchange *rabbitmqcomv1beta1.Exchange) (warnings admission.Warnings, err error) {
	var allErrs field.ErrorList
	const detailMsg = "updates on name, vhost, and rabbitmqClusterReference are all forbidden"
	if newExchange.Spec.Name != oldExchange.Spec.Name {
		return nil, apierrors.NewForbidden(newExchange.GroupResource(), newExchange.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if newExchange.Spec.Vhost != oldExchange.Spec.Vhost {
		return nil, apierrors.NewForbidden(newExchange.GroupResource(), newExchange.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldExchange.Spec.RabbitmqClusterReference.Matches(&newExchange.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newExchange.GroupResource(), newExchange.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	if newExchange.Spec.Type != oldExchange.Spec.Type {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "type"),
			newExchange.Spec.Type,
			"exchange type cannot be updated",
		))
	}

	if newExchange.Spec.AutoDelete != oldExchange.Spec.AutoDelete {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "autoDelete"),
			newExchange.Spec.AutoDelete,
			"autoDelete cannot be updated",
		))
	}

	if newExchange.Spec.Durable != oldExchange.Spec.Durable {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "durable"),
			newExchange.Spec.Durable,
			"durable cannot be updated",
		))
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, allErrs.ToAggregate()
}

func (v *ExchangeCustomValidator) ValidateDelete(_ context.Context, _ *rabbitmqcomv1beta1.Exchange) (warnings admission.Warnings, err error) {
	return nil, nil
}
