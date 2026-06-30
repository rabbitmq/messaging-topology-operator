/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1beta1

const (
	// TopologyOperatorLabel must be present on any Kubernetes Secret that the Topology Operator
	// reads through its informer cache. The manager restricts the shared Secret informer to
	// only cache Secrets carrying this label, bounding memory usage on large clusters.
	//
	// Operator-generated secrets (e.g. <user>-user-credentials) are labeled automatically.
	// User-provided secrets (importCredentialsSecret, connectionSecret, uriSecret,
	// upstreamSecret) must carry this label; admission webhooks enforce this requirement.
	TopologyOperatorLabel      = "rabbitmq.com/topology-operator"
	TopologyOperatorLabelValue = "true"
)
