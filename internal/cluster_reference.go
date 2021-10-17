package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . CredentialsProvider
type CredentialsProvider interface {
	GetUser() string
	GetPassword() string
}

type ClusterCredentials struct {
	username string
	password string
}

func (c ClusterCredentials) GetUser() string {
	return c.username
}

func (c ClusterCredentials) GetPassword() string {
	return c.password
}

var SecretStoreClientInitializer = InitializeSecretStoreClient

var (
	NoSuchRabbitmqClusterError = errors.New("RabbitmqCluster object does not exist")
	ResourceNotAllowedError    = errors.New("Resource is not allowed to reference defined cluster reference. Check the namespace of the resource is allowed as part of the cluster's `rabbitmq.com/topology-allowed-namespaces` annotation")
)

func ParseRabbitmqClusterReference(ctx context.Context, c client.Client, rmq topology.RabbitmqClusterReference, requestNamespace string) (*rabbitmqv1beta1.RabbitmqCluster, *corev1.Service, CredentialsProvider, error) {
	var namespace string
	if rmq.Namespace == "" {
		namespace = requestNamespace
	} else {
		namespace = rmq.Namespace
	}
	cluster := &rabbitmqv1beta1.RabbitmqCluster{}
	if err := c.Get(ctx, types.NamespacedName{Name: rmq.Name, Namespace: namespace}, cluster); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get cluster from reference: %s Error: %w", err, NoSuchRabbitmqClusterError)
	}

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
			return nil, nil, nil, ResourceNotAllowedError
		}
	}

	var credentialsProvider CredentialsProvider
	if cluster.Spec.SecretBackend.Vault != nil && cluster.Spec.SecretBackend.Vault.DefaultUserPath != "" {
		// ask the configured secure store for the credentials available at the path retrived from the cluster resource
		secretStoreClient, err := SecretStoreClientInitializer(cluster.Spec.SecretBackend.Vault)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("unable to create a client connection to secret store: %w", err)
		}

		credsProv, err := secretStoreClient.ReadCredentials(cluster.Spec.SecretBackend.Vault.DefaultUserPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("unable to retrieve credentials from secret store: %w", err)
		}
		credentialsProvider = credsProv
	} else {
		// use credentials in namespace Kubernetes Secret
		if cluster.Status.Binding == nil {
			return nil, nil, nil, errors.New("no status.binding set")
		}

		if cluster.Status.DefaultUser == nil {
			return nil, nil, nil, errors.New("no status.defaultUser set")
		}

		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.Binding.Name}, secret); err != nil {
			return nil, nil, nil, err
		}
		credsProv, err := readCredentialsFromKubernetesSecret(secret)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("unable to retrieve credentials from Kubernetes secret %s: %w", secret.Name, err)
		}
		credentialsProvider = credsProv
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, nil, nil, err
	}
	return cluster, svc, credentialsProvider, nil
}

func readCredentialsFromKubernetesSecret(secret *corev1.Secret) (CredentialsProvider, error) {
	if secret == nil {
		return nil, errors.New("unable to extract data from nil secret")
	}

	if secret.Data["username"] == nil {
		return nil, errors.New("secret data contains no username value")
	}

	if secret.Data["password"] == nil {
		return nil, errors.New("secret data contains no password value")
	}

	return ClusterCredentials{username: string(secret.Data["username"]), password: string(secret.Data["password"])}, nil
}
