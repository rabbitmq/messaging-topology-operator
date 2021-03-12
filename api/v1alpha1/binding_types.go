/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// BindingSpec defines the desired state of Binding
type BindingSpec struct {
	// Default to vhost '/'
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// +kubebuilder:validation:Optional
	Source string `json:"source,omitempty"`
	// +kubebuilder:validation:Optional
	Destination string `json:"destination,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=exchange;queue
	DestinationType string `json:"destinationType,omitempty"`
	// +kubebuilder:validation:Optional
	RoutingKey string `json:"routingKey,omitempty"`
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`
	// Reference to the RabbitmqCluster that the binding will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// BindingStatus defines the observed state of Binding
type BindingStatus struct {
}

// +kubebuilder:object:root=true

// Binding is the Schema for the bindings API
type Binding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BindingSpec   `json:"spec,omitempty"`
	Status BindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BindingList contains a list of Binding
type BindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Binding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Binding{}, &BindingList{})
}
