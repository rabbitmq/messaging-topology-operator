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

// ExchangeSpec defines the desired state of Exchange
type ExchangeSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// +kubebuilder:validation:Enum=direct;fanout;headers;topic
	// +kubebuilder:default:=direct
	Type       string `json:"type,omitempty"`
	Durable    bool   `json:"durable,omitempty"`
	AutoDelete bool   `json:"autoDelete,omitempty"`
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`
	// Reference to the RabbitmqCluster that the exchange will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// ExchangeStatus defines the observed state of Exchange
type ExchangeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// Exchange is the Schema for the exchanges API
type Exchange struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExchangeSpec   `json:"spec,omitempty"`
	Status ExchangeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExchangeList contains a list of Exchange
type ExchangeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Exchange `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Exchange{}, &ExchangeList{})
}
