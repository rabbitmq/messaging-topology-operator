package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PolicySpec defines the desired state of Policy
// https://www.rabbitmq.com/parameters.html#policies
type PolicySpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// Regular expression pattern used to match queues and exchanges, e.g. "^amq.".
	// Required property.
	// +kubebuilder:validation:Required
	Pattern string `json:"pattern"`
	// What this policy applies to: 'queues', 'exchanges', or 'all'.
	// Default to 'all'.
	// +kubebuilder:validation:Enum=queues;exchanges;all
	// +kubebuilder:default:=all
	ApplyTo string `json:"applyTo,omitempty"`
	// Default to '0'.
	// In the event that more than one policy can match a given exchange or queue, the policy with the greatest priority applies.
	// +kubebuilder:default:=0
	Priority int `json:"priority,omitempty"`
	// Policy definition. Required property.
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Definition *runtime.RawExtension `json:"definition"`
	// Reference to the RabbitmqCluster that the exchange will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// PolicyStatus defines the observed state of Policy
type PolicyStatus struct {
}

// +kubebuilder:object:root=true

// Policy is the Schema for the policies API
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec,omitempty"`
	Status PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PolicyList contains a list of Policy
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}
