package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ShovelSpec defines the desired state of Shovel
// For how to configure Shovel, see: https://www.rabbitmq.com/shovel.html.
type ShovelSpec struct {
	// Required property; cannot be updated
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'; cannot be updated
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// Reference to the RabbitmqCluster that this Shovel will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
	// Secret contains the AMQP URI(s) to configure Shovel destination and source.
	// The Secret must contain the key `destUri` and `srcUri` or operator will error.
	// Both fields should be one or multiple uris separated by ','.
	// Required property.
	// +kubebuilder:validation:Required
	UriSecret *corev1.LocalObjectReference `json:"uriSecret"`
	// +kubebuilder:validation:Enum=on-confirm;on-publish;no-ack
	AckMode                       string `json:"ackMode,omitempty"`
	PrefetchCount                 int    `json:"prefetchCount,omitempty"`
	ReconnectDelay                int    `json:"reconnectDelay,omitempty"`
	AddForwardHeaders             bool   `json:"addForwardHeaders,omitempty"`
	DeleteAfter                   string `json:"deleteAfter,omitempty"`
	SourceDeleteAfter             string `json:"srcDeleteAfter,omitempty"`
	SourcePrefetchCount           int    `json:"srcPrefetchCount,omitempty"`
	DestinationAddForwardHeaders  bool   `json:"destAddForwardHeaders,omitempty"`
	DestinationAddTimestampHeader bool   `json:"destAddTimestampHeader,omitempty"`

	// +kubebuilder:validation:Enum=amqp091;amqp10
	DestinationProtocol string `json:"destProtocol,omitempty"`
	// amqp091 configuration
	DestinationQueue string `json:"destQueue,omitempty"`
	// amqp091 configuration
	DestinationExchange string `json:"destExchange,omitempty"`
	// amqp091 configuration
	DestinationExchangeKey string `json:"destExchangeKey,omitempty"`
	// amqp091 configuration
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	DestinationPublishProperties *runtime.RawExtension `json:"destPublishProperties,omitempty"`
	// amqp10 configuration; required if destProtocol is amqp10
	DestinationAddress string `json:"destAddress,omitempty"`
	// amqp10 configuration
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	DestinationApplicationProperties *runtime.RawExtension `json:"destApplicationProperties,omitempty"`
	// amqp10 configuration
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	DestinationProperties *runtime.RawExtension `json:"destProperties,omitempty"`
	// amqp10 configuration
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	DestinationMessageAnnotations *runtime.RawExtension `json:"destMessageAnnotations,omitempty"`

	// +kubebuilder:validation:Enum=amqp091;amqp10
	SourceProtocol string `json:"srcProtocol,omitempty"`
	// amqp091 configuration
	SourceQueue string `json:"srcQueue,omitempty"`
	// amqp091 configuration
	SourceExchange string `json:"srcExchange,omitempty"`
	// amqp091 configuration
	SourceExchangeKey string `json:"srcExchangeKey,omitempty"`
	// amqp091 configuration
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	SourceConsumerArgs *runtime.RawExtension `json:"srcConsumerArgs,omitempty"`
	// amqp10 configuration; required if srcProtocol is amqp10
	SourceAddress string `json:"srcAddress,omitempty"`
}

// ShovelStatus defines the observed state of Shovel
type ShovelStatus struct {
	// observedGeneration is the most recent successful generation observed for this Shovel. It corresponds to the
	// Shovel's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all;rabbitmq
// +kubebuilder:subresource:status

// Shovel is the Schema for the shovels API
type Shovel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShovelSpec   `json:"spec,omitempty"`
	Status ShovelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ShovelList contains a list of Shovel
type ShovelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Shovel `json:"items"`
}

func (s *Shovel) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    s.GroupVersionKind().Group,
		Resource: s.GroupVersionKind().Kind,
	}
}

func (s *Shovel) RabbitReference() RabbitmqClusterReference {
	return s.Spec.RabbitmqClusterReference
}

func (s *Shovel) SetStatusConditions(c []Condition) {
	s.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&Shovel{}, &ShovelList{})
}
