package internal

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ConnectionCredentials
type ConnectionCredentials interface {
	Data(key string) ([]byte, bool)
}

type ClusterCredentials struct {
	data map[string][]byte
}

func (c ClusterCredentials) Data(key string) ([]byte, bool) {
	result, ok := c.data[key]
	return result, ok
}

var SecretStoreClientProvider = GetSecretStoreClient

var (
	NoSuchRabbitmqClusterError = errors.New("RabbitmqCluster object does not exist")
	ResourceNotAllowedError    = errors.New("Resource is not allowed to reference defined cluster reference. Check the namespace of the resource is allowed as part of the cluster's `rabbitmq.com/topology-allowed-namespaces` annotation")
)

func ParseRabbitmqClusterReference(ctx context.Context, c client.Client, rmq topology.RabbitmqClusterReference, requestNamespace string) (ConnectionCredentials, bool, error) {
	if rmq.ConnectionSecret != nil {
		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: requestNamespace, Name: rmq.ConnectionSecret.Name}, secret); err != nil {
			return nil, false, err
		}
		return readCredentialsFromKubernetesSecret(secret)
	}

	var namespace string
	if rmq.Namespace == "" {
		namespace = requestNamespace
	} else {
		namespace = rmq.Namespace
	}

	cluster := &rabbitmqv1beta1.RabbitmqCluster{}
	if err := c.Get(ctx, types.NamespacedName{Name: rmq.Name, Namespace: namespace}, cluster); err != nil {
		return nil, false, fmt.Errorf("failed to get cluster from reference: %s Error: %w", err, NoSuchRabbitmqClusterError)
	}

	if !AllowedNamespace(rmq, requestNamespace, cluster) {
		return nil, false, ResourceNotAllowedError
	}

	var user, pass string
	if cluster.Spec.SecretBackend.Vault != nil && cluster.Spec.SecretBackend.Vault.DefaultUserPath != "" {
		// ask the configured secure store for the credentials available at the path retrieved from the cluster resource
		secretStoreClient, err := SecretStoreClientProvider()
		if err != nil {
			return nil, false, fmt.Errorf("unable to create a client connection to secret store: %w", err)
		}

		user, pass, err = secretStoreClient.ReadCredentials(cluster.Spec.SecretBackend.Vault.DefaultUserPath)
		if err != nil {
			return nil, false, fmt.Errorf("unable to retrieve credentials from secret store: %w", err)
		}
	} else {
		// use credentials in namespace Kubernetes Secret
		if cluster.Status.Binding == nil {
			return nil, false, errors.New("no status.binding set")
		}

		if cluster.Status.DefaultUser == nil {
			return nil, false, errors.New("no status.defaultUser set")
		}

		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.Binding.Name}, secret); err != nil {
			return nil, false, err
		}
		var err error
		user, pass, err = readUsernamePassword(secret)
		if err != nil {
			return nil, false, fmt.Errorf("unable to retrieve credentials from Kubernetes secret %s: %w", secret.Name, err)
		}
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, false, err
	}

	endpoint, err := managementURI(svc, cluster.TLSEnabled())
	if err != nil {
		return nil, false, fmt.Errorf("failed to get endpoint from specified rabbitmqcluster: %w", err)
	}

	return ClusterCredentials{
		data: map[string][]byte{
			"username": []byte(user),
			"password": []byte(pass),
			"uri":      []byte(endpoint),
		},
	}, cluster.TLSEnabled(), nil
}

func AllowedNamespace(rmq topology.RabbitmqClusterReference, requestNamespace string, cluster *rabbitmqv1beta1.RabbitmqCluster) bool {
	if rmq.Namespace != "" && rmq.Namespace != requestNamespace {
		var isAllowed bool
		if allowedNamespaces, ok := cluster.Annotations["rabbitmq.com/topology-allowed-namespaces"]; ok {
			for _, allowedNamespace := range strings.Split(allowedNamespaces, ",") {
				if requestNamespace == allowedNamespace || allowedNamespace == "*" {
					isAllowed = true
					break
				}
			}
		}
		if !isAllowed {
			return false
		}
	}
	return true
}

func readCredentialsFromKubernetesSecret(secret *corev1.Secret) (ConnectionCredentials, bool, error) {
	if secret == nil {
		return nil, false, fmt.Errorf("unable to retrieve information from Kubernetes secret %s: %w", secret.Name, errors.New("nil secret"))
	}

	uBytes, found := secret.Data["uri"]
	if !found {
		return nil, false, keyMissingErr("uri")
	}

	uri := string(uBytes)
	if string(uBytes[0:4]) != "http" {
		uri = "http://" + uri // set scheme to http if not provided
	}
	var tlsEnabled bool
	if parsed, err := url.Parse(uri); err != nil {
		return nil, false, err
	} else if parsed.Scheme == "https" {
		tlsEnabled = true
	}

	return ClusterCredentials{
		data: map[string][]byte{
			"username": secret.Data["username"],
			"password": secret.Data["password"],
			"uri":      []byte(uri),
		},
	}, tlsEnabled, nil
}

func readUsernamePassword(secret *corev1.Secret) (string, string, error) {
	if secret == nil {
		return "", "", errors.New("unable to extract data from nil secret")
	}

	return string(secret.Data["username"]), string(secret.Data["password"]), nil
}

func managementURI(svc *corev1.Service, tlsEnabled bool) (string, error) {
	port := managementPort(svc)
	if port == 0 {
		return "", fmt.Errorf("failed to find 'management' or 'management-tls' from service %s", svc.Name)
	}

	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, serviceDNSAddress(svc), port), nil
}

// serviceDNSAddress returns the cluster-local DNS entry associated
// with the provided Service
func serviceDNSAddress(svc *corev1.Service) string {
	// NOTE: this does not use the `cluster.local` suffix, because that is not
	// uniform across clusters. See the `clusterDomain` KubeletConfiguration
	// value for how this can be changed for a cluster.
	return fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace)
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
