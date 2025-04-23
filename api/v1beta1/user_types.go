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

// UserSpec defines the desired state of User.
type UserSpec struct {
	// List of permissions tags to associate with the user. This determines the level of
	// access to the RabbitMQ management UI granted to the user. Omitting this field will
	// lead to a user than can still connect to the cluster through messaging protocols,
	// but cannot perform any management actions.
	// For more information, see https://www.rabbitmq.com/management.html#permissions.
	Tags []UserTag `json:"tags,omitempty"`
	// Reference to the RabbitmqCluster that the user will be created for. This cluster must
	// exist for the User object to be created.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
	// Defines a Secret containing the credentials for the User. If this field is omitted, random a username and
	// password will be generated. The Secret must have the following keys in its Data field:
	//
	//  * `username` – Must be present or the import will fail.
	//  * `passwordHash` – The SHA-512 hash of the password. If the hash is an empty string, a passwordless user
	//    will be created. For more information, see https://www.rabbitmq.com/docs/passwords.
	//  * `password` – Plain-text password. Will be used only if the `passwordHash` key is missing.
	//
	// Note that this import only occurs at creation time, and is ignored once a password has been set on a User.
	ImportCredentialsSecret *corev1.LocalObjectReference `json:"importCredentialsSecret,omitempty"`
	// Limits to apply to a user to restrict the number of connections and channels
	// the user can create. These limits can be used as guard rails in environments
	// where applications cannot be trusted and monitored in detail, for example,
	// when RabbitMQ clusters are offered as a service. See https://www.rabbitmq.com/docs/user-limits.
	UserLimits *UserLimits `json:"limits,omitempty"`
}

// UserStatus defines the observed state of User.
type UserStatus struct {
	// observedGeneration is the most recent successful generation observed for this User. It corresponds to the
	// User's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
	// Provides a reference to a Secret object containing the user credentials.
	Credentials *corev1.LocalObjectReference `json:"credentials,omitempty"`
	// Provide rabbitmq Username
	Username string `json:"username"`
}

// UserTag defines the level of access to the management UI allocated to the user.
// For more information, see https://www.rabbitmq.com/management.html#permissions.
// +kubebuilder:validation:Enum=management;policymaker;monitoring;administrator
type UserTag string

// Limits to apply to a user to restrict the number of connections and channels
// the user can create. These limits can be used as guard rails in environments
// where applications cannot be trusted and monitored in detail, for example,
// when RabbitMQ clusters are offered as a service. See https://www.rabbitmq.com/docs/user-limits.
type UserLimits struct {
	// Limits how many connections the user can open.
	Connections *int32 `json:"connections,omitempty"`
	// Limits how many AMQP 0.9.1 channels the user can open.
	Channels *int32 `json:"channels,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=rabbitmq
// +kubebuilder:subresource:status

// User is the Schema for the users API.
// +kubebuilder:subresource:status
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec configures the desired state of the User object.
	Spec UserSpec `json:"spec,omitempty"`
	// Status exposes the observed state of the User object.
	Status UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of Users.
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func (u *User) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    u.GroupVersionKind().Group,
		Resource: u.GroupVersionKind().Kind,
	}
}

func (u *User) RabbitReference() RabbitmqClusterReference {
	return u.Spec.RabbitmqClusterReference
}

func (u *User) SetStatusConditions(c []Condition) {
	u.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
