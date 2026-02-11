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
type FederationCustomValidator struct{}

// SetupFederationWebhookWithManager registers the webhook for Federation in the manager.
func SetupFederationWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.Federation{}).
		WithValidator(&FederationCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-federation,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=federations,versions=v1beta1,name=vfederation.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (v *FederationCustomValidator) ValidateCreate(_ context.Context, fed *rabbitmqcomv1beta1.Federation) (warnings admission.Warnings, err error) {
	return fed.Spec.RabbitmqClusterReference.ValidateOnCreate(fed.GroupResource(), fed.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *FederationCustomValidator) ValidateUpdate(ctx context.Context, oldFederation, newFederation *rabbitmqcomv1beta1.Federation) (warnings admission.Warnings, err error) {
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

func (v *FederationCustomValidator) ValidateDelete(_ context.Context, _ *rabbitmqcomv1beta1.Federation) (warnings admission.Warnings, err error) {
	return nil, nil
}
