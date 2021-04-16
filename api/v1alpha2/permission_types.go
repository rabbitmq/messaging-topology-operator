package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PermissionSpec defines the desired state of Permission
type PermissionSpec struct {
	// Name of an existing user; required property.
	// +kubebuilder:validation:Required
	User string `json:"user"`
	// Name of an existing vhost; required property.
	// +kubebuilder:validation:Required
	Vhost string `json:"vhost"`
	// Permissions to grant to the user in the specific vhost; required property.
	// See RabbitMQ doc for more information: https://www.rabbitmq.com/access-control.html#user-management
	// +kubebuilder:validation:Required
	Permissions VhostPermissions `json:"permissions"`
	// Reference to the RabbitmqCluster that both the provided user and vhost are.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// Set of RabbitMQ permissions: configure, read and write.
// By not setting a property (configure/write/read), it result in an empty string which does not not match any permission.
type VhostPermissions struct {
	// +kubebuilder:validation:Optional
	Configure string `json:"configure,omitempty"`
	// +kubebuilder:validation:Optional
	Write string `json:"write,omitempty"`
	// +kubebuilder:validation:Optional
	Read string `json:"read,omitempty"`
}

// PermissionStatus defines the observed state of Permission
type PermissionStatus struct {
	// observedGeneration is the most recent successful generation observed for this Permission. It corresponds to the
	// Permission's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all
// +kubebuilder:subresource:status

// Permission is the Schema for the permissions API
type Permission struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PermissionSpec   `json:"spec,omitempty"`
	Status PermissionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PermissionList contains a list of Permission
type PermissionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Permission `json:"items"`
}

func (p *Permission) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    p.GroupVersionKind().Group,
		Resource: p.GroupVersionKind().Kind,
	}
}

func init() {
	SchemeBuilder.Register(&Permission{}, &PermissionList{})
}
