/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"fmt"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// UserCredentials describes the credentials that can be provided in ImportCredentialsSecret for a User.
// If the secret is not provided, a random username and password will be generated.
type UserCredentials struct {
	// Must be present if ImportCredentialsSecret is provided.
	Username string
	// If PasswordHash is an empty string, a passwordless user is created.
	// If PasswordHash is nil, Password is used instead.
	PasswordHash *string
	// If Password is empty and PasswordHash is nil, a random password is generated.
	Password string
}

func GenerateUserSettings(credentials UserCredentials, tags []topology.UserTag) (rabbithole.UserSettings, error) {
	if credentials.Username == "" {
		return rabbithole.UserSettings{}, fmt.Errorf("username is required in credentials")
	}

	var passwordHash string
	if credentials.PasswordHash != nil {
		passwordHash = *credentials.PasswordHash
	} else {
		if credentials.Password == "" {
			return rabbithole.UserSettings{}, fmt.Errorf("neither passwordHash nor password is set in credentials")
		}
		passwordHashStr := rabbithole.Base64EncodedSaltedPasswordHashSHA512(credentials.Password)
		passwordHash = passwordHashStr
	}

	userTagStrings := make([]string, len(tags))
	for i, tag := range tags {
		userTagStrings[i] = string(tag)
	}

	return rabbithole.UserSettings{
		Name:             credentials.Username,
		Tags:             userTagStrings,
		PasswordHash:     passwordHash,
		HashingAlgorithm: rabbithole.HashingAlgorithmSHA512,
	}, nil
}

func GenerateUserLimits(userLimits *topology.UserLimits) rabbithole.UserLimitsValues {
	userLimitsValues := rabbithole.UserLimitsValues{}
	if userLimits != nil {
		if userLimits.Connections != nil {
			userLimitsValues["max-connections"] = int(*userLimits.Connections)
		}
		if userLimits.Channels != nil {
			userLimitsValues["max-channels"] = int(*userLimits.Channels)
		}
	}
	return userLimitsValues
}
