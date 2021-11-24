/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CompositeConsumerSpec defines the desired state of CompositeConsumer
type CompositeConsumerSpec struct {
	// Reference to the SuperStream that the CompositeConsumer will consume from
	// Required property.
	// +kubebuilder:validation:Required
	SuperStreamReference SuperStreamReference `json:"superStreamReference"`
	// +kubebuilder:validation:Required
	ConsumerPodSpec CompositeConsumerPodSpec `json:"consumerPodSpec"`
}

type CompositeConsumerPodSpec struct {
	// +kubebuilder:validation:Optional
	Default *corev1.PodSpec `json:"default,omitempty"`
	// +kubebuilder:validation:Optional
	PerRoutingKey map[string]*corev1.PodSpec `json:"perRoutingKey,omitempty"`
}

// CompositeConsumerStatus defines the observed state of CompositeConsumer
type CompositeConsumerStatus struct {
	// observedGeneration is the most recent successful generation observed for this CompositeConsumer. It corresponds to the
	// CompositeConsumer's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all
// +kubebuilder:subresource:status

// CompositeConsumer is the Schema for Composite Consumers
type CompositeConsumer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositeConsumerSpec   `json:"spec,omitempty"`
	Status CompositeConsumerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CompositeConsumerList contains a list of CompositeConsumers
type CompositeConsumerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CompositeConsumer `json:"items"`
}

func (q *CompositeConsumer) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    q.GroupVersionKind().Group,
		Resource: q.GroupVersionKind().Kind,
	}
}

type SuperStreamReference struct {
	// The name of the SuperStream to reference.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// The namespace of the SuperStream to reference.
	// Defaults to the namespace of the requested resource if omitted.
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace"`
}

func init() {
	SchemeBuilder.Register(&CompositeConsumer{}, &CompositeConsumerList{})
}
