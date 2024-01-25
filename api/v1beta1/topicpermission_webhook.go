package v1beta1

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (t *TopicPermission) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(t).
		Complete()
}

//+kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-topicpermission,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=topicpermissions,verbs=create;update,versions=v1beta1,name=vtopicpermission.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &TopicPermission{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (t *TopicPermission) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	tp, ok := obj.(*TopicPermission)
	if !ok {
		return nil, fmt.Errorf("expected a RabbitMQ permission but got a %T", obj)
	}

	if t.Spec.User == "" && t.Spec.UserReference == nil {
		return nil, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference")
	}

	if t.Spec.User != "" && t.Spec.UserReference != nil {
		return nil, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time")
	}
	return nil, t.Spec.RabbitmqClusterReference.validate(tp.RabbitReference())
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (t *TopicPermission) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldPermission, ok := oldObj.(*TopicPermission)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a permission but got a %T", oldObj))
	}

	newPermission, ok := newObj.(*TopicPermission)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a permission but got a %T", newObj))
	}

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
func (t *TopicPermission) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
