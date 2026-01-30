/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// SetupPermissionWebhookWithManager registers the webhook for Permission in the manager.
func SetupPermissionWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.Permission{}).
		WithValidator(&PermissionCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-permission,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=permissions,verbs=create;update,versions=v1beta1,name=vpermission-v1beta1.kb.io,admissionReviewVersions=v1

// PermissionCustomValidator struct is responsible for validating the Permission resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PermissionCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Permission.
func (v *PermissionCustomValidator) ValidateCreate(_ context.Context, obj *rabbitmqcomv1beta1.Permission) (admission.Warnings, error) {
	if obj.Spec.User == "" && obj.Spec.UserReference == nil {
		return nil, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference")
	}

	if obj.Spec.User != "" && obj.Spec.UserReference != nil {
		return nil, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time")
	}

	return obj.Spec.RabbitmqClusterReference.ValidateOnCreate(obj.GroupResource(), obj.Name)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Permission.
func (v *PermissionCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *rabbitmqcomv1beta1.Permission) (admission.Warnings, error) {
	var errorList field.ErrorList
	if newObj.Spec.User == "" && newObj.Spec.UserReference == nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference"))
		return nil, apierrors.NewInvalid(rabbitmqcomv1beta1.GroupVersion.WithKind("Permission").GroupKind(), newObj.Name, errorList)
	}

	if newObj.Spec.User != "" && newObj.Spec.UserReference != nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time"))
		return nil, apierrors.NewInvalid(rabbitmqcomv1beta1.GroupVersion.WithKind("Permission").GroupKind(), newObj.Name, errorList)
	}

	const detailMsg = "updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden"
	if newObj.Spec.User != oldObj.Spec.User {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "user"), detailMsg))
	}

	if userReferenceUpdated(newObj.Spec.UserReference, oldObj.Spec.UserReference) {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "userReference"), detailMsg))
	}

	if newObj.Spec.Vhost != oldObj.Spec.Vhost {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldObj.Spec.RabbitmqClusterReference.Matches(&newObj.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Permission.
func (v *PermissionCustomValidator) ValidateDelete(_ context.Context, obj *rabbitmqcomv1beta1.Permission) (admission.Warnings, error) {
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
