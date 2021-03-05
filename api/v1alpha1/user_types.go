/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserSpec defines the desired state of User
type UserSpec struct {
	// +kubebuilder:validation:Required
	Name string    `json:"name"`
	Tags []UserTag `json:"tags,omitempty"`
	// Reference to the RabbitmqCluster that the user will be created in
	// Required property
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
	// TODO: Allow the provision of the user with a pre-defined password through a Secret here
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// Provides a reference to a Secret object containing the user credentials.
	Credentials *corev1.LocalObjectReference `json:"credentials,omitempty"`
}

// UserTag defines the level of access to the management UI allocated to the user.
// +kubebuilder:validation:Enum=management;policymaker;monitoring;administrator
type UserTag string

// +kubebuilder:object:root=true

// User is the Schema for the users API
// +kubebuilder:subresource:status
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
