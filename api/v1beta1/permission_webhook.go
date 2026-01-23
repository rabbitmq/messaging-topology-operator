package v1beta1

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Implements admission.Validator
type PermissionValidator struct{}

func (p *Permission) SetupWebhookWithManager(mgr ctrl.Manager) error {
	var permissionValidator PermissionValidator
	return ctrl.NewWebhookManagedBy(mgr, &Permission{}).
		WithValidator(permissionValidator).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-permission,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=permissions,versions=v1beta1,name=vpermission.kb.io,sideEffects=none,admissionReviewVersions=v1

// ValidateCreate checks if only one of spec.user and spec.userReference is specified
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (pv PermissionValidator) ValidateCreate(_ context.Context, pe *Permission) (warnings admission.Warnings, err error) {
	if pe.Spec.User == "" && pe.Spec.UserReference == nil {
		return nil, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference")
	}

	if pe.Spec.User != "" && pe.Spec.UserReference != nil {
		return nil, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time")
	}

	return nil, pe.Spec.RabbitmqClusterReference.validate(pe.RabbitReference())
}

// ValidateUpdate do not allow updates on spec.vhost, spec.user, spec.userReference, and spec.rabbitmqClusterReference
// updates on spec.permissions are allowed
// only one of spec.user and spec.userReference can be specified
func (pv PermissionValidator) ValidateUpdate(_ context.Context, oldPermission, newPermission *Permission) (warnings admission.Warnings, err error) {
	var errorList field.ErrorList
	if newPermission.Spec.User == "" && newPermission.Spec.UserReference == nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference"))
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("Permission").GroupKind(), newPermission.Name, errorList)
	}

	if newPermission.Spec.User != "" && newPermission.Spec.UserReference != nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time"))
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("Permission").GroupKind(), newPermission.Name, errorList)
	}

	const detailMsg = "updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden"
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

func (pv PermissionValidator) ValidateDelete(_ context.Context, _ *Permission) (warnings admission.Warnings, err error) {
	return nil, nil
}

// returns true if userReference, which is a pointer to corev1.LocalObjectReference, has changed
func userReferenceUpdated(new, old *corev1.LocalObjectReference) bool {
	if new == nil && old == nil {
		return false
	}
	if (new == nil && old != nil) ||
		(new != nil && old == nil) {
		return true
	}
	if new.Name != old.Name {
		return true
	}
	return false
}
