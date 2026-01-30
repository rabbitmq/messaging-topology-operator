package v1beta1

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implememnts admission.Validator
type QueueValidator struct{}

func (q *Queue) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var queueValidator QueueValidator
	return ctrl.NewWebhookManagedBy(mgr, &Queue{}).
		WithValidator(queueValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-queue,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=queues,versions=v1beta1,name=vqueue.kb.io,sideEffects=none,admissionReviewVersions=v1sideEffects=none,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
// Either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (qv QueueValidator) ValidateCreate(_ context.Context, inQueue *Queue) (warnings admission.Warnings, err error) {
	if inQueue.Spec.Type == "quorum" && !inQueue.Spec.Durable {
		return nil, apierrors.NewForbidden(inQueue.GroupResource(), inQueue.Name,
			field.Forbidden(field.NewPath("spec", "durable"),
				"Quorum queues must have durable set to true"))
	}
	return nil, inQueue.Spec.RabbitmqClusterReference.validate(inQueue.RabbitReference())
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
//
// Returns error type 'forbidden' for updates that the controller chooses to disallow: queue name/vhost/rabbitmqClusterReference
//
// Returns error type 'invalid' for updates that will be rejected by rabbitmq server: queue types/autoDelete/durable
func (qv QueueValidator) ValidateUpdate(_ context.Context, oldQueue, newQueue *Queue) (warnings admission.Warnings, err error) {
	var allErrs field.ErrorList
	const detailMsg = "updates on name, vhost, and rabbitmqClusterReference are all forbidden"
	if newQueue.Spec.Name != oldQueue.Spec.Name {
		return nil, apierrors.NewForbidden(newQueue.GroupResource(), newQueue.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if newQueue.Spec.Vhost != oldQueue.Spec.Vhost {
		return nil, apierrors.NewForbidden(newQueue.GroupResource(), newQueue.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldQueue.Spec.RabbitmqClusterReference.Matches(&newQueue.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newQueue.GroupResource(), newQueue.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	if newQueue.Spec.Type != oldQueue.Spec.Type {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "type"),
			newQueue.Spec.Type,
			"queue type cannot be updated",
		))
	}

	if newQueue.Spec.AutoDelete != oldQueue.Spec.AutoDelete {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "autoDelete"),
			newQueue.Spec.AutoDelete,
			"autoDelete cannot be updated",
		))
	}

	if newQueue.Spec.Durable != oldQueue.Spec.Durable {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "durable"),
			newQueue.Spec.Durable,
			"durable cannot be updated",
		))
	}

	if oldQueue.Spec.Arguments != nil && newQueue.Spec.Arguments == nil {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "arguments"),
			newQueue.Spec.Arguments,
			"queue arguments cannot be updated",
		))
	}

	if oldQueue.Spec.Arguments == nil && newQueue.Spec.Arguments != nil {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "arguments"),
			newQueue.Spec.Arguments,
			"queue arguments cannot be updated",
		))
	}

	if oldQueue.Spec.Arguments != nil && newQueue.Spec.Arguments != nil {
		previousArgs := make(map[string]any)
		err := json.Unmarshal(oldQueue.Spec.Arguments.Raw, &previousArgs)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error unmarshalling previous Queue arguments: %w", err))
		}

		updatedArgs := make(map[string]any)
		err = json.Unmarshal(newQueue.Spec.Arguments.Raw, &updatedArgs)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("error unmarshalling current Queue arguments: %w", err))
		}

		if !maps.Equal(previousArgs, updatedArgs) {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "arguments"),
				newQueue.Spec.Arguments,
				"queue arguments cannot be updated",
			))
		}
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	//goland:noinspection GoDfaNilDereference
	return nil, allErrs.ToAggregate()
}

func (qv QueueValidator) ValidateDelete(_ context.Context, _ *Queue) (warnings admission.Warnings, err error) {
	return nil, nil
}
