package v1beta1

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Implements admission.Validator
type TopicPermissionValidator struct{}

func (t *TopicPermission) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var topicPermissionValidator TopicPermissionValidator
	return ctrl.NewWebhookManagedBy(mgr, &TopicPermission{}).
		WithValidator(topicPermissionValidator).
		Complete()
}

//+kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-topicpermission,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=topicpermissions,verbs=create;update,versions=v1beta1,name=vtopicpermission.kb.io,admissionReviewVersions={v1,v1beta1}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (tpv TopicPermissionValidator) ValidateCreate(_ context.Context, tp *TopicPermission) (warnings admission.Warnings, err error) {
	if tp.Spec.User == "" && tp.Spec.UserReference == nil {
		return nil, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference")
	}

	if tp.Spec.User != "" && tp.Spec.UserReference != nil {
		return nil, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time")
	}
	return nil, tp.Spec.RabbitmqClusterReference.validate(tp.RabbitReference())
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (tpv TopicPermissionValidator) ValidateUpdate(_ context.Context, oldPermission, newPermission *TopicPermission) (warnings admission.Warnings, err error) {
	var errorList field.ErrorList
	if newPermission.Spec.User == "" && newPermission.Spec.UserReference == nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference"))
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("TopicPermission").GroupKind(), newPermission.Name, errorList)
	}

	if newPermission.Spec.User != "" && newPermission.Spec.UserReference != nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time"))
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("TopicPermission").GroupKind(), newPermission.Name, errorList)
	}

	const detailMsg = "updates on exchange, user, userReference, vhost and rabbitmqClusterReference are all forbidden"
	if newPermission.Spec.Permissions.Exchange != oldPermission.Spec.Permissions.Exchange {
		return nil, apierrors.NewForbidden(newPermission.GroupResource(), newPermission.Name,
			field.Forbidden(field.NewPath("spec", "permissions", "exchange"), detailMsg))
	}

	if newPermission.Spec.User != oldPermission.Spec.User {
		return nil, apierrors.NewForbidden(newPermission.GroupResource(), newPermission.Name,
			field.Forbidden(field.NewPath("spec", "user"), detailMsg))
	}

	if userReferenceUpdated(newPermission.Spec.UserReference, oldPermission.Spec.UserReference) {
		return nil, apierrors.NewForbidden(newPermission.GroupResource(), newPermission.Name,
			field.Forbidden(field.NewPath("spec", "userReference"), detailMsg))
	}

	if newPermission.Spec.Vhost != oldPermission.Spec.Vhost {
		return nil, apierrors.NewForbidden(newPermission.GroupResource(), newPermission.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldPermission.Spec.RabbitmqClusterReference.Matches(&newPermission.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newPermission.GroupResource(), newPermission.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (tpv TopicPermissionValidator) ValidateDelete(_ context.Context, _ *TopicPermission) (warnings admission.Warnings, err error) {
	return nil, nil
}
