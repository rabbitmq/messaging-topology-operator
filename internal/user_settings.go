/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"encoding/base64"
	"fmt"
	"strings"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func GenerateUserSettings(credentials *corev1.Secret, tags []topologyv1alpha1.UserTag) (rabbithole.UserSettings, error) {
	username, err := decodeField(credentials, "username")
	if err != nil {
		return rabbithole.UserSettings{}, err
	}
	password, err := decodeField(credentials, "password")
	if err != nil {
		return rabbithole.UserSettings{}, err
	}

	var userTagStrings []string
	for _, tag := range tags {
		userTagStrings = append(userTagStrings, string(tag))
	}

	return rabbithole.UserSettings{
		Name:             username,
		Tags:             strings.Join(userTagStrings, ","),
		PasswordHash:     rabbithole.Base64EncodedSaltedPasswordHashSHA512(password),
		HashingAlgorithm: rabbithole.HashingAlgorithmSHA512,
	}, nil
}

func decodeField(credentials *corev1.Secret, field string) (string, error) {
	credentialField, ok := credentials.Data[field]
	if !ok {
		return "", fmt.Errorf("Could not find field %s in credentials %s", field, credentials.Name)
	}
	decodedField, err := base64.StdEncoding.DecodeString(string(credentialField))
	if err != nil {
		return "", fmt.Errorf("Failed to base64 decode string: %s", credentialField)
	}
	return string(decodedField), nil
}
