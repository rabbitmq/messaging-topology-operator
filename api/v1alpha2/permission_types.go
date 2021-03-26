package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PermissionSpec defines the desired state of Permission
type PermissionSpec struct {
	// Name of the user; has to an existing user.
	// Required property.
	// +kubebuilder:validation:Required
	User string `json:"user"`
	// Required property; has to an existing vhost.
	// +kubebuilder:validation:Required
	Vhost string `json:"vhost"`
	// +kubebuilder:validation:Required
	Permissions VhostPermissions `json:"permissions"`
	// Reference to the RabbitmqCluster that both the provided user and vhost are.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// Defines a RabbitMQ user permissions in the specified vhost.
// By not setting a property (configure/write/read), it result in an empty string which does not not match any permission.
// For more information, see official doc: https://www.rabbitmq.com/access-control.html#user-management
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
