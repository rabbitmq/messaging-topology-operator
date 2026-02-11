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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// SetupTopicPermissionWebhookWithManager registers the webhook for TopicPermission in the manager.
func SetupTopicPermissionWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.TopicPermission{}).
		WithValidator(&TopicPermissionCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-topicpermission,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=topicpermissions,verbs=create;update,versions=v1beta1,name=vtopicpermission-v1beta1.kb.io,admissionReviewVersions=v1

type TopicPermissionCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type TopicPermission.
func (v *TopicPermissionCustomValidator) ValidateCreate(_ context.Context, obj *rabbitmqcomv1beta1.TopicPermission) (admission.Warnings, error) {
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

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type TopicPermission.
func (v *TopicPermissionCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *rabbitmqcomv1beta1.TopicPermission) (admission.Warnings, error) {
	var errorList field.ErrorList
	if newObj.Spec.User == "" && newObj.Spec.UserReference == nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference"))
		return nil, apierrors.NewInvalid(rabbitmqcomv1beta1.GroupVersion.WithKind("TopicPermission").GroupKind(), newObj.Name, errorList)
	}

	if newObj.Spec.User != "" && newObj.Spec.UserReference != nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time"))
		return nil, apierrors.NewInvalid(rabbitmqcomv1beta1.GroupVersion.WithKind("TopicPermission").GroupKind(), newObj.Name, errorList)
	}

	const detailMsg = "updates on exchange, user, userReference, vhost and rabbitmqClusterReference are all forbidden"
	if newObj.Spec.Permissions.Exchange != oldObj.Spec.Permissions.Exchange {
		return nil, apierrors.NewForbidden(newObj.GroupResource(), newObj.Name,
			field.Forbidden(field.NewPath("spec", "permissions", "exchange"), detailMsg))
	}

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

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type TopicPermission.
func (v *TopicPermissionCustomValidator) ValidateDelete(_ context.Context, obj *rabbitmqcomv1beta1.TopicPermission) (admission.Warnings, error) {
	return nil, nil
}
