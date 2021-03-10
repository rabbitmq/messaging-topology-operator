/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VhostSpec defines the desired state of Vhost
type VhostSpec struct {
	// Name of the vhost; see https://www.rabbitmq.com/vhosts.html.
	// +kubebuilder:validation:Required
	Name    string `json:"name"`
	Tracing bool   `json:"tracing,omitempty"`
	// Reference to the RabbitmqCluster that the vhost will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// VhostStatus defines the observed state of Vhost
type VhostStatus struct {
}

// +kubebuilder:object:root=true

// Vhost is the Schema for the vhosts API
type Vhost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VhostSpec   `json:"spec,omitempty"`
	Status VhostStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VhostList contains a list of Vhost
type VhostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vhost `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Vhost{}, &VhostList{})
}
