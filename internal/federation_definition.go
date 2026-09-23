/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"strings"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

func GenerateFederationDefinition(f *topology.Federation, uri string) rabbithole.FederationDefinition {
	// rabbit-hole serialises reconnect-delay without omitempty, so an unset value would be
	// sent to RabbitMQ as an explicit 0 rather than letting the server apply its own
	// default. The CRD defaults this field, so nil only happens for objects that bypassed
	// defaulting; fall back to the RabbitMQ default of 1 rather than to 0.
	reconnectDelay := 1
	if f.Spec.ReconnectDelay != nil {
		reconnectDelay = *f.Spec.ReconnectDelay
	}

	return rabbithole.FederationDefinition{
		Uri:                 strings.Split(uri, ","),
		Expires:             f.Spec.Expires,
		MessageTTL:          int32(f.Spec.MessageTTL),
		MaxHops:             f.Spec.MaxHops,
		PrefetchCount:       f.Spec.PrefetchCount,
		ReconnectDelay:      reconnectDelay,
		AckMode:             f.Spec.AckMode,
		TrustUserId:         f.Spec.TrustUserId,
		Exchange:            f.Spec.Exchange,
		Queue:               f.Spec.Queue,
		QueueType:           f.Spec.QueueType,
		ResourceCleanupMode: f.Spec.ResourceCleanupMode,
	}
}
