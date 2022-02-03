package v1beta1

import corev1 "k8s.io/api/core/v1"

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
	// The Secret must contain the key `uri` or operator will error.
	// Have to set either name or connectionSecret, but not both.
	// +kubebuilder:validation:Optional
	ConnectionSecret *corev1.LocalObjectReference `json:"connectionSecret,omitempty"`
}

func (r *RabbitmqClusterReference) hasChange(new *RabbitmqClusterReference) bool {
	if new.Name != r.Name || new.Namespace != r.Namespace {
		return true
	}

	// when connectionSecret has been updated
	if new.ConnectionSecret != nil && r.ConnectionSecret != nil && *new.ConnectionSecret != *r.ConnectionSecret {
		return true
	}

	// when connectionSecret is removed
	if new.ConnectionSecret == nil && r.ConnectionSecret != nil {
		return true
	}

	// when connectionSecret is added
	if new.ConnectionSecret != nil && r.ConnectionSecret == nil {
		return true
	}

	return false
}
