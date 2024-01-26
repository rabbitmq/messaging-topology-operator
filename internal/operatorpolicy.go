/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"encoding/json"
	"fmt"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

func GenerateOperatorPolicy(p *topology.OperatorPolicy) (*rabbithole.OperatorPolicy, error) {
	definition := make(map[string]interface{})
	if err := json.Unmarshal(p.Spec.Definition.Raw, &definition); err != nil {
		return nil, fmt.Errorf("failed to unmarshall policy definition: %v", err)
	}

	return &rabbithole.OperatorPolicy{
		Vhost:      p.Spec.Vhost,
		Pattern:    p.Spec.Pattern,
		ApplyTo:    p.Spec.ApplyTo,
		Name:       p.Spec.Name,
		Priority:   p.Spec.Priority,
		Definition: definition,
	}, nil
}
