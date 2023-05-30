/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1alpha1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (s *SuperStream) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1alpha1-superstream,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=superstreams,versions=v1alpha1,name=vsuperstream.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &SuperStream{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (s *SuperStream) ValidateCreate() (admission.Warnings, error) {
	return s.Spec.RabbitmqClusterReference.ValidateOnCreate(s.GroupResource(), s.Name)
}

// ValidateUpdate returns error type 'forbidden' for updates on superstream name, vhost and rabbitmqClusterReference
func (s *SuperStream) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldSuperStream, ok := old.(*SuperStream)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a superstream but got a %T", old))
	}

	detailMsg := "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if s.Spec.Name != oldSuperStream.Spec.Name {
		return nil, apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}
	if s.Spec.Vhost != oldSuperStream.Spec.Vhost {
		return nil, apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldSuperStream.Spec.RabbitmqClusterReference.Matches(&s.Spec.RabbitmqClusterReference) {
		return nil, apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	if !routingKeyUpdatePermitted(oldSuperStream.Spec.RoutingKeys, s.Spec.RoutingKeys) {
		return nil, apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "routingKeys"), "updates may only add to the existing list of routing keys"))
	}

	if s.Spec.Partitions < oldSuperStream.Spec.Partitions {
		return nil, apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "partitions"), "updates may only increase the partition count, and may not decrease it"))
	}

	return nil, nil
}

// ValidateDelete no validation on delete
func (s *SuperStream) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

// routingKeyUpdatePermitted allows updates only if adding additional keys at the end of the list of keys
func routingKeyUpdatePermitted(old, new []string) bool {
	if len(old) == 0 && len(new) != 0 {
		return false
	}
	for i := 0; i < len(old); i++ {
		if old[i] != new[i] {
			return false
		}
	}
	return true
}
