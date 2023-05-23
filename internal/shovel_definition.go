package internal

import (
	"encoding/json"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"strings"
)

func GenerateShovelDefinition(s *topology.Shovel, srcUri, destUri string) (*rabbithole.ShovelDefinition, error) {
	srcConArgs := make(map[string]interface{})
	if s.Spec.SourceConsumerArgs != nil {
		if err := json.Unmarshal(s.Spec.SourceConsumerArgs.Raw, &srcConArgs); err != nil {
			return nil, fmt.Errorf("failed to unmarshall: %v", err)
		}
	}
	appProperties := make(map[string]interface{})
	if s.Spec.DestinationApplicationProperties != nil {
		if err := json.Unmarshal(s.Spec.DestinationApplicationProperties.Raw, &appProperties); err != nil {
			return nil, fmt.Errorf("failed to unmarshall: %v", err)
		}
	}
	destProperties := make(map[string]interface{})
	if s.Spec.DestinationProperties != nil {
		if err := json.Unmarshal(s.Spec.DestinationProperties.Raw, &destProperties); err != nil {
			return nil, fmt.Errorf("failed to unmarshall: %v", err)
		}
	}
	destPubProperties := make(map[string]interface{})
	if s.Spec.DestinationPublishProperties != nil {
		if err := json.Unmarshal(s.Spec.DestinationPublishProperties.Raw, &destPubProperties); err != nil {
			return nil, fmt.Errorf("failed to unmarshall: %v", err)
		}
	}
	destMsgAnnotations := make(map[string]interface{})
	if s.Spec.DestinationMessageAnnotations != nil {
		if err := json.Unmarshal(s.Spec.DestinationMessageAnnotations.Raw, &destMsgAnnotations); err != nil {
			return nil, fmt.Errorf("failed to unmarshall: %v", err)
		}
	}

	return &rabbithole.ShovelDefinition{
		SourceURI:                        strings.Split(srcUri, ","),
		DestinationURI:                   strings.Split(destUri, ","),
		AckMode:                          s.Spec.AckMode,
		AddForwardHeaders:                s.Spec.AddForwardHeaders,
		DeleteAfter:                      rabbithole.DeleteAfter(s.Spec.DeleteAfter),
		DestinationAddForwardHeaders:     s.Spec.DestinationAddForwardHeaders,
		DestinationAddTimestampHeader:    s.Spec.DestinationAddTimestampHeader,
		DestinationAddress:               s.Spec.DestinationAddress,
		DestinationApplicationProperties: appProperties,
		DestinationExchange:              s.Spec.DestinationExchange,
		DestinationExchangeKey:           s.Spec.DestinationExchangeKey,
		DestinationProperties:            destProperties,
		DestinationProtocol:              s.Spec.DestinationProtocol,
		DestinationPublishProperties:     destPubProperties,
		DestinationQueue:                 s.Spec.DestinationQueue,
		DestinationMessageAnnotations:    destMsgAnnotations,
		PrefetchCount:                    s.Spec.PrefetchCount,
		ReconnectDelay:                   s.Spec.ReconnectDelay,
		SourceAddress:                    s.Spec.SourceAddress,
		SourceDeleteAfter:                rabbithole.DeleteAfter(s.Spec.SourceDeleteAfter),
		SourceExchange:                   s.Spec.SourceExchange,
		SourceExchangeKey:                s.Spec.SourceExchangeKey,
		SourcePrefetchCount:              s.Spec.SourcePrefetchCount,
		SourceProtocol:                   s.Spec.SourceProtocol,
		SourceQueue:                      s.Spec.SourceQueue,
		SourceConsumerArgs:               srcConArgs,
	}, nil
}
