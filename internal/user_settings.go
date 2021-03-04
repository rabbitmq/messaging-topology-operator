/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"strings"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

func GenerateUserSettings(userSpec topologyv1alpha1.UserSpec, password string) rabbithole.UserSettings {
	var userTagStrings []string
	for _, tag := range userSpec.Tags {
		userTagStrings = append(userTagStrings, string(tag))
	}

	return rabbithole.UserSettings{
		Name:             userSpec.Name,
		Tags:             strings.Join(userTagStrings, ","),
		PasswordHash:     rabbithole.Base64EncodedSaltedPasswordHashSHA512(password),
		HashingAlgorithm: rabbithole.HashingAlgorithmSHA512,
	}
}
