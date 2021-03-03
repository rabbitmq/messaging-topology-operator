package internal

import (
	"encoding/json"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

func GenerateExchangeSettings(e *topologyv1alpha1.Exchange) (*rabbithole.ExchangeSettings, error) {
	arguments := make(map[string]interface{})
	if e.Spec.Arguments != nil {
		if err := json.Unmarshal(e.Spec.Arguments.Raw, &arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshall exchange arguments: %v", err)
		}
	}

	return &rabbithole.ExchangeSettings{
		Durable:    e.Spec.Durable,
		AutoDelete: e.Spec.AutoDelete,
		Type:       e.Spec.Type,
		Arguments:  arguments,
	}, nil
}
