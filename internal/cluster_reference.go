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

var (
	NoSuchRabbitmqClusterError = errors.New("RabbitmqCluster object does not exist")
	ResourceNotAllowedError    = errors.New("Resource is not allowed to reference defined cluster reference. Check the namespace of the resource is allowed as part of the cluster's `rabbitmq.com/topology-allowed-namespaces` annotation")
)

func ParseRabbitmqClusterReference(ctx context.Context, c client.Client, rmq topology.RabbitmqClusterReference, requestNamespace string, credentialsLocator CredentialsLocator) (*rabbitmqv1beta1.RabbitmqCluster, *corev1.Service, CredentialsProvider, error) {
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

	if cluster.Status.Binding == nil {
		return nil, nil, nil, errors.New("no status.binding set")
	}

	if cluster.Spec.SecretBackend.Vault.DefaultUserPath == "" {
		return nil, nil, nil, errors.New("no spec.secretBackend.vault.defaultUserPath")
	}
	vaultPath := cluster.Spec.SecretBackend.Vault.DefaultUserPath

	// ask the configured Vault server for the credentials stored at
	// the default user path
	credentialsProvider, err := credentialsLocator.ReadCredentials(vaultPath)
	if err != nil {
		return nil, nil, nil, err
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, nil, nil, err
	}
	return cluster, svc, credentialsProvider, nil
}
