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

// CompositeConsumerSetSpec defines the desired state of CompositeConsumerSet
type CompositeConsumerSetSpec struct {
	// Reference to the SuperStream that the CompositeConsumerSet will consume from
	// Required property.
	// +kubebuilder:validation:Required
	SuperStreamReference SuperStreamReference `json:"superStreamReference"`
	// Replicas denotes the number of composite consumers to create in the set.
	// By default, this value matches the number of partitions in the SuperStream.
	// +kubebuilder:validation:Optional
	Replicas int `json:"replicas,omitempty"`
	// +kubebuilder:validation:Required
	ConsumerPodSpec CompositeConsumerPodSpec `json:"consumerPodSpec"`
}

type CompositeConsumerPodSpec struct {
	// +kubebuilder:validation:Optional
	Default corev1.PodSpec `json:"default,omitempty"`
	// +kubebuilder:validation:Optional
	PerRoutingKey map[string]corev1.PodSpec `json:"perRoutingKey,omitempty"`
}

// CompositeConsumerSetStatus defines the observed state of CompositeConsumerSet
type CompositeConsumerSetStatus struct {
	// observedGeneration is the most recent successful generation observed for this CompositeConsumerSet. It corresponds to the
	// CompositeConsumerSet's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all
// +kubebuilder:subresource:status

// CompositeConsumerSet is the Schema for Composite Consumer Sets
type CompositeConsumerSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositeConsumerSetSpec   `json:"spec,omitempty"`
	Status CompositeConsumerSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CompositeConsumerSetList contains a list of CompositeConsumerSets
type CompositeConsumerSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CompositeConsumerSet `json:"items"`
}

func (q *CompositeConsumerSet) GroupResource() schema.GroupResource {
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
	SchemeBuilder.Register(&CompositeConsumerSet{}, &CompositeConsumerSetList{})
}
