package internal

import (
	"encoding/json"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

func GenerateBindingInfo(binding *topologyv1alpha1.Binding) (*rabbithole.BindingInfo, error) {
	arguments := make(map[string]interface{})
	if binding.Spec.Arguments != nil {
		if err := json.Unmarshal(binding.Spec.Arguments.Raw, &arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshall binding arguments: %v", err)
		}
	}

	return &rabbithole.BindingInfo{
		Vhost:           binding.Spec.Vhost,
		Source:          binding.Spec.Source,
		Destination:     binding.Spec.Destination,
		DestinationType: binding.Spec.DestinationType,
		RoutingKey:      binding.Spec.RoutingKey,
		Arguments:       arguments,
	}, nil
}
