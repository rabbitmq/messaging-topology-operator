/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"fmt"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func GenerateUserSettings(credentials *corev1.Secret, tags []topology.UserTag) (rabbithole.UserSettings, error) {
	username, ok := credentials.Data["username"]
	if !ok {
		return rabbithole.UserSettings{}, fmt.Errorf("could not find username in credentials secret %s", credentials.Name)
	}

	passwordHash, ok := credentials.Data["passwordHash"]
	if !ok {
		// Use password as a fallback
		password, ok := credentials.Data["password"]
		if !ok {
			return rabbithole.UserSettings{}, fmt.Errorf("could not find passwordHash or password in credentials secret %s", credentials.Name)
		}
		// To avoid sending raw passwords over the wire, compute a password hash using a random salt
		// and use this in the UserSettings instead.
		// For more information on this hashing algorithm, see
		// https://www.rabbitmq.com/passwords.html#computing-password-hash.
		passwordHashStr := rabbithole.Base64EncodedSaltedPasswordHashSHA512(string(password))
		passwordHash = []byte(passwordHashStr)
	}

	var userTagStrings []string
	for _, tag := range tags {
		userTagStrings = append(userTagStrings, string(tag))
	}

	return rabbithole.UserSettings{
		Name:             string(username),
		Tags:             userTagStrings,
		PasswordHash:     string(passwordHash),
		HashingAlgorithm: rabbithole.HashingAlgorithmSHA512,
	}, nil
}
