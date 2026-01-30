package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// OperatorPolicySpec defines the desired state of OperatorPolicy
// https://www.rabbitmq.com/parameters.html#operator-policies
type OperatorPolicySpec struct {
	// Required property; cannot be updated
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'; cannot be updated
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// Regular expression pattern used to match queues, e.g. "^my-queue$".
	// Required property.
	// +kubebuilder:validation:Required
	Pattern string `json:"pattern"`
	// What this operator policy applies to: 'queues', 'classic_queues', 'quorum_queues', 'streams'.
	// Default to 'queues'.
	// +kubebuilder:validation:Enum=queues;classic_queues;quorum_queues;streams
	// +kubebuilder:default:=queues
	ApplyTo string `json:"applyTo,omitempty"`
	// Default to '0'.
	// In the event that more than one operator policy can match a given queue, the operator policy with the greatest priority applies.
	// +kubebuilder:default:=0
	Priority int `json:"priority,omitempty"`
	// OperatorPolicy definition. Required property.
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Definition *runtime.RawExtension `json:"definition"`
	// Reference to the RabbitmqCluster that the operator policy will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// OperatorPolicyStatus defines the observed state of OperatorPolicy
type OperatorPolicyStatus struct {
	// observedGeneration is the most recent successful generation observed for this OperatorPolicy. It corresponds to the
	// OperatorPolicy's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=rabbitmq
// +kubebuilder:subresource:status

// OperatorPolicy is the Schema for the operator policies API
type OperatorPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperatorPolicySpec   `json:"spec,omitempty"`
	Status OperatorPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OperatorPolicyList contains a list of OperatorPolicy
type OperatorPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperatorPolicy `json:"items"`
}

func (p *OperatorPolicy) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    p.GroupVersionKind().Group,
		Resource: p.GroupVersionKind().Kind,
	}
}

func (p *OperatorPolicy) RabbitReference() RabbitmqClusterReference {
	return p.Spec.RabbitmqClusterReference
}

func (p *OperatorPolicy) SetStatusConditions(c []Condition) {
	p.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&OperatorPolicy{}, &OperatorPolicyList{})
}
