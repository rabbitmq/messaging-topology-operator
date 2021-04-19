/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
}

type RabbitMQClientFactory func(ctx context.Context, c client.Client, rmq topology.RabbitmqClusterReference, namespace string, certPool *x509.CertPool) (RabbitMQClient, error)

var RabbitholeClientFactory RabbitMQClientFactory = func(ctx context.Context, c client.Client, rmq topology.RabbitmqClusterReference, namespace string, certPool *x509.CertPool) (RabbitMQClient, error) {
	return generateRabbitholeClient(ctx, c, rmq, namespace, certPool)
}

var NoSuchRabbitmqClusterError = errors.New("RabbitmqCluster object does not exist")

// returns a http client for the given RabbitmqCluster
// assumes the RabbitmqCluster is reachable using its service's ClusterIP
func generateRabbitholeClient(ctx context.Context, c client.Client, rmq topology.RabbitmqClusterReference, namespace string, certPool *x509.CertPool) (rabbitmqClient RabbitMQClient, err error) {
	cluster, err := rabbitmqClusterFromReference(ctx, c, rmq, namespace)
	if err != nil {
		return nil, err
	}

	svc, secret, err := serviceSecretFromCluster(ctx, c, cluster, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get service or secret object from specified rabbitmqcluster: %w", err)
	}

	endpoint, err := managementEndpoint(cluster, svc)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoint from specified rabbitmqcluster: %w", err)
	}

	defaultUser, found := secret.Data["username"]
	if !found {
		return nil, errors.New("failed to retrieve username: key username missing from secret")
	}

	defaultUserPass, found := secret.Data["password"]
	if !found {
		return nil, errors.New("failed to retrieve username: key password missing from secret")
	}

	if cluster.TLSEnabled() {
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

func managementEndpoint(cluster *rabbitmqv1beta1.RabbitmqCluster, svc *corev1.Service) (string, error) {
	ip := net.ParseIP(svc.Spec.ClusterIP)
	if ip == nil {
		return "", fmt.Errorf("failed to get Cluster IP: invalid ClusterIP %q", svc.Spec.ClusterIP)
	}

	port, err := managementPort(svc)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s://%s:%d", managementScheme(cluster), ip.String(), port), nil
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
func managementPort(svc *corev1.Service) (int, error) {
	for _, port := range svc.Spec.Ports {
		if port.Name == "management-tls" {
			return int(port.Port), nil
		}
		if port.Name == "management" {
			return int(port.Port), nil
		}
	}
	return 0, fmt.Errorf("failed to find 'management' or 'management-tls' from service %s", svc.Name)
}

func rabbitmqClusterFromReference(ctx context.Context, c client.Client, rmq topology.RabbitmqClusterReference, namespace string) (*rabbitmqv1beta1.RabbitmqCluster, error) {
	cluster := &rabbitmqv1beta1.RabbitmqCluster{}
	if err := c.Get(ctx, types.NamespacedName{Name: rmq.Name, Namespace: namespace}, cluster); err != nil {
		return nil, fmt.Errorf("failed to get cluster from reference: %s Error: %w", err, NoSuchRabbitmqClusterError)
	}
	return cluster, nil
}

func serviceSecretFromCluster(ctx context.Context, c client.Client, cluster *rabbitmqv1beta1.RabbitmqCluster, namespace string) (*corev1.Service, *corev1.Secret, error) {
	if cluster.Status.Binding == nil {
		return nil, nil, errors.New("no status.binding set")
	}
	if cluster.Status.DefaultUser == nil {
		return nil, nil, errors.New("no status.defaultUser set")
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.Binding.Name}, secret); err != nil {
		return nil, nil, err
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, nil, err
	}
	return svc, secret, nil
}
