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

// GenerateQueueDeleteOptions generates rabbithole.QueueDeleteOptions for a given Queue
// queue.Spec.Arguments (type k8s runtime.RawExtensions) is unmarshalled
func GenerateQueueDeleteOptions(q *topology.Queue) (*rabbithole.QueueDeleteOptions, error) {

	return &rabbithole.QueueDeleteOptions{
		// Set these values to false if q.Spec.Type = Quorum, not supported by the API
		IfEmpty:  q.Spec.Type != "quorum" && q.Spec.DeleteIfEmpty,
		IfUnused: q.Spec.Type != "quorum" && q.Spec.DeleteIfUnused,
	}, nil
}
