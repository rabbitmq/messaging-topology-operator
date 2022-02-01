/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . RabbitMQClient
type RabbitMQClient interface {
	PutUser(string, rabbithole.UserSettings) (*http.Response, error)
	DeleteUser(string) (*http.Response, error)
	DeclareBinding(string, rabbithole.BindingInfo) (*http.Response, error)
	DeleteBinding(string, rabbithole.BindingInfo) (*http.Response, error)
	ListQueueBindingsBetween(string, string, string) ([]rabbithole.BindingInfo, error)
	ListExchangeBindingsBetween(string, string, string) ([]rabbithole.BindingInfo, error)
	UpdatePermissionsIn(string, string, rabbithole.Permissions) (*http.Response, error)
	ClearPermissionsIn(string, string) (*http.Response, error)
	PutPolicy(string, string, rabbithole.Policy) (*http.Response, error)
	DeletePolicy(string, string) (*http.Response, error)
	DeclareQueue(string, string, rabbithole.QueueSettings) (*http.Response, error)
	DeleteQueue(string, string, ...rabbithole.QueueDeleteOptions) (*http.Response, error)
	DeclareExchange(string, string, rabbithole.ExchangeSettings) (*http.Response, error)
	DeleteExchange(string, string) (*http.Response, error)
	PutVhost(string, rabbithole.VhostSettings) (*http.Response, error)
	DeleteVhost(string) (*http.Response, error)
	PutGlobalParameter(name string, value interface{}) (*http.Response, error)
	DeleteGlobalParameter(name string) (*http.Response, error)
	PutFederationUpstream(vhost, name string, def rabbithole.FederationDefinition) (res *http.Response, err error)
	DeleteFederationUpstream(vhost, name string) (res *http.Response, err error)
	DeclareShovel(vhost, shovel string, info rabbithole.ShovelDefinition) (res *http.Response, err error)
	DeleteShovel(vhost, shovel string) (res *http.Response, err error)
}

type RabbitMQClientFactory func(rmq *rabbitmqv1beta1.RabbitmqCluster, svc *corev1.Service, credsProvider CredentialsProvider, hostname string, certPool *x509.CertPool) (RabbitMQClient, error)

var RabbitholeClientFactory RabbitMQClientFactory = func(rmq *rabbitmqv1beta1.RabbitmqCluster, svc *corev1.Service, credsProvider CredentialsProvider, hostname string, certPool *x509.CertPool) (RabbitMQClient, error) {
	return generateRabbitholeClient(rmq, svc, credsProvider, hostname, certPool)
}

// generateRabbitholeClient returns a http client for the given RabbitmqCluster
// if provided RabbitmqCluster is nil, generateRabbitholeClient uses username, passwords, and uri
// information from credsProvider to generate a rabbit client
func generateRabbitholeClient(rmq *rabbitmqv1beta1.RabbitmqCluster, svc *corev1.Service, credsProvider CredentialsProvider, hostname string, certPool *x509.CertPool) (rabbitmqClient RabbitMQClient, err error) {
	if rmq == nil {
		return generateRabbitmqClientFromCredsProvider(credsProvider)
	}

	defaultUser, found := credsProvider.Data("username")
	if !found {
		return nil, keyMissingErr("username")
	}

	defaultUserPass, found := credsProvider.Data("password")
	if !found {
		return nil, keyMissingErr("password")
	}

	endpoint, err := managementEndpoint(rmq, svc, hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoint from specified rabbitmqcluster: %w", err)
	}

	if rmq.TLSEnabled() {
		// create TLS config for https request
		cfg := new(tls.Config)
		cfg.RootCAs = certPool

		transport := &http.Transport{TLSClientConfig: cfg}
		rabbitmqClient, err = rabbithole.NewTLSClient(endpoint, string(defaultUser), string(defaultUserPass), transport)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate rabbit rabbitmqClient: %v", err)
		}
	} else {
		rabbitmqClient, err = rabbithole.NewClient(endpoint, string(defaultUser), string(defaultUserPass))
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate rabbit rabbitmqClient: %v", err)
		}
	}
	return rabbitmqClient, nil
}

// generateRabbitmqClientFromCredsProvider generates a rabbit http client
// it expects key "username", "password", "uri" to be present, else it errors
//  TODO: it currently hardcode scheme to http, https will be added in a followup PR
func generateRabbitmqClientFromCredsProvider(credsProvider CredentialsProvider) (RabbitMQClient, error) {
	defaultUser, found := credsProvider.Data("username")
	if !found {
		return nil, keyMissingErr("username")
	}

	defaultUserPass, found := credsProvider.Data("password")
	if !found {
		return nil, keyMissingErr("password")
	}

	uri, found := credsProvider.Data("uri")
	if !found {
		return nil, keyMissingErr("uri")
	}

	rabbitmqClient, err := rabbithole.NewClient("http://"+string(uri), string(defaultUser), string(defaultUserPass))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate rabbit rabbitmqClient: %v", err)
	}

	return rabbitmqClient, nil
}

func keyMissingErr(key string) error {
	return errors.New(fmt.Sprintf("failed to retrieve %s: key %s missing from credentials", key, key))
}

func managementEndpoint(cluster *rabbitmqv1beta1.RabbitmqCluster, svc *corev1.Service, hostname string) (string, error) {
	port := managementPort(svc)
	if port == 0 {
		return "", fmt.Errorf("failed to find 'management' or 'management-tls' from service %s", svc.Name)
	}

	return fmt.Sprintf("%s://%s:%d", managementScheme(cluster), hostname, port), nil
}

// returns RabbitMQ management scheme from given cluster
func managementScheme(cluster *rabbitmqv1beta1.RabbitmqCluster) string {
	if cluster.TLSEnabled() {
		return "https"
	} else {
		return "http"
	}
}

// returns RabbitMQ management port from given service
// if both "management-tls" and "management" ports are present, returns the "management-tls" port
func managementPort(svc *corev1.Service) int {
	var httpPort int
	for _, port := range svc.Spec.Ports {
		if port.Name == "management-tls" {
			return int(port.Port)
		}
		if port.Name == "management" {
			httpPort = int(port.Port)
		}
	}
	return httpPort
}
