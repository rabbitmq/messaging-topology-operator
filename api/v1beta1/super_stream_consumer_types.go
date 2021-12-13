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

// SuperStreamConsumerSpec defines the desired state of SuperStreamConsumer
type SuperStreamConsumerSpec struct {
	// Reference to the SuperStream that the SuperStreamConsumer will consume from, in the same namespace.
	// Required property.
	// +kubebuilder:validation:Required
	SuperStreamReference SuperStreamReference `json:"superStreamReference"`
	// ConsumerPodSpec defines the PodSpecs to use for any consumer Pods that are created for the SuperStream.
	// +kubebuilder:validation:Required
	ConsumerPodSpec SuperStreamConsumerPodSpec `json:"consumerPodSpec"`
}

type SuperStreamConsumerPodSpec struct {
	// Default defines the PodSpec to use for all consumer Pods, if no routing key-specific PodSpec is provided.
	// +kubebuilder:validation:Optional
	Default *corev1.PodSpec `json:"default,omitempty"`
	// PerRoutingKey maps PodsSpecs to specific routing keys. If a consumer is spun up for a SuperStream partition,
	// and the routing key for that partition matches an entry in PerRoutingKey, that PodSpec will be used for the
	// consumer Pod; otherwise the default PodSpec is used.
	// +kubebuilder:validation:Optional
	PerRoutingKey map[string]*corev1.PodSpec `json:"perRoutingKey,omitempty"`
}

// SuperStreamConsumerStatus defines the observed state of SuperStreamConsumer
type SuperStreamConsumerStatus struct {
	// observedGeneration is the most recent successful generation observed for this SuperStreamConsumer. It corresponds to the
	// SuperStreamConsumer's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all
// +kubebuilder:subresource:status

// SuperStreamConsumer is the Schema for SuperStreamConsumers
type SuperStreamConsumer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SuperStreamConsumerSpec   `json:"spec,omitempty"`
	Status SuperStreamConsumerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SuperStreamConsumerList contains a list of SuperStreamConsumers
type SuperStreamConsumerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SuperStreamConsumer `json:"items"`
}

func (q *SuperStreamConsumer) GroupResource() schema.GroupResource {
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
	Namespace string `json:"namespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&SuperStreamConsumer{}, &SuperStreamConsumerList{})
}
