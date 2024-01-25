package v1beta1

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type RabbitmqClusterReference struct {
	// The name of the RabbitMQ cluster to reference.
	// Have to set either name or connectionSecret, but not both.
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// The namespace of the RabbitMQ cluster to reference.
	// Defaults to the namespace of the requested resource if omitted.
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
	// Secret contains the http management uri for the RabbitMQ cluster.
	// The Secret must contain the key `uri`, `username` and `password` or operator will error.
	// Have to set either name or connectionSecret, but not both.
	// +kubebuilder:validation:Optional
	ConnectionSecret *corev1.LocalObjectReference `json:"connectionSecret,omitempty"`
}

func (r *RabbitmqClusterReference) Matches(new *RabbitmqClusterReference) bool {
	if new.Name != r.Name || new.Namespace != r.Namespace {
		return false
	}

	// when connectionSecret has been updated
	if new.ConnectionSecret != nil && r.ConnectionSecret != nil && *new.ConnectionSecret != *r.ConnectionSecret {
		return false
	}

	// when connectionSecret is removed
	if new.ConnectionSecret == nil && r.ConnectionSecret != nil {
		return false
	}

	// when connectionSecret is added
	if new.ConnectionSecret != nil && r.ConnectionSecret == nil {
		return false
	}

	return true
}

// ValidateOnCreate validates RabbitmqClusterReference on resources create
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both; else it errors
func (r *RabbitmqClusterReference) ValidateOnCreate(_ schema.GroupResource, _ string) (admission.Warnings, error) {
	// TODO: make this function private when we deprecate or promote v1alpha1 SuperStreamController
	return nil, r.validate(*r)
}

func (r *RabbitmqClusterReference) validate(ref RabbitmqClusterReference) error {
	if ref.Name != "" && ref.ConnectionSecret != nil {
		return errors.New("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")
	}

	if ref.Name == "" && ref.ConnectionSecret == nil {
		return errors.New("invalid RabbitmqClusterReference: must provide either name or connectionSecret")
	}
	return nil
}
