package internal

import (
	"context"
	"errors"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . CredentialsProvider
type CredentialsProvider interface {
	Data(key string) ([]byte, bool)
}

type ClusterCredentials struct {
	data map[string][]byte
}

func (c ClusterCredentials) Data(key string) ([]byte, bool) {
	result, ok := c.data[key]
	return result, ok
}

type ClusterCredentialsProvider interface {
	GetCredentialsProvider(ctx context.Context, requestNamespace string, cluster *rabbitmqv1beta1.RabbitmqCluster) (CredentialsProvider, *corev1.Service, error)
	Accept(cluster *rabbitmqv1beta1.RabbitmqCluster) bool
}
type RabbitMQClusterCredentialsProvider struct {
	providers []ClusterCredentialsProvider
}

func NewRabbitMQClusterCredentialsProvider(client client.Client) (*RabbitMQClusterCredentialsProvider, error) {
	ccp, err := newRabbitMQClusterCredentialsProvider(client)
	if err != nil {
		return nil, err
	}
	return ccp, nil
}

func newRabbitMQClusterCredentialsProvider(c client.Client) (*RabbitMQClusterCredentialsProvider, error) {
	vault, err := newVaultClusterCredentialsProvider(c)
	if err != nil {
		return nil, err
	}
	providers := append([]ClusterCredentialsProvider{}, &vault)

	k8s, err := newK8sSecretClusterCredentialsProvider(c)
	if err != nil {
		return nil, err
	}
	providers = append(providers, &k8s)

	return &RabbitMQClusterCredentialsProvider{providers: providers}, nil
}
func namespace(requestNamespace string, cluster *rabbitmqv1beta1.RabbitmqCluster) string {
	var namespace string
	if cluster.Namespace == "" {
		namespace = requestNamespace
	} else {
		namespace = cluster.Namespace
	}
	return namespace
}
func (ccp *RabbitMQClusterCredentialsProvider) GetCredentialsProvider(ctx context.Context, requestNamespace string,
	cluster *rabbitmqv1beta1.RabbitmqCluster) (CredentialsProvider, *corev1.Service, error) {
	for _, p := range ccp.providers {
		if p.Accept(cluster) {
			return p.GetCredentialsProvider(ctx, requestNamespace, cluster)
		}
	}
	return nil, nil, fmt.Errorf("unable to find suitable credentials provider for cluster %s", cluster.Name)
}

type K8sSecretClusterCredentialsProvider struct {
	client client.Client
}

func newK8sSecretClusterCredentialsProvider(c client.Client) (K8sSecretClusterCredentialsProvider, error) {
	return K8sSecretClusterCredentialsProvider{client: c}, nil
}

type VaultClusterCredentialsProvider struct {
	vaultReader VaultReader
	client      client.Client
}

func newVaultClusterCredentialsProvider(c client.Client) (VaultClusterCredentialsProvider, error) {
	return VaultClusterCredentialsProvider{
		client:      c,
		vaultReader: nil,
	}, nil
}

func (ccp *K8sSecretClusterCredentialsProvider) GetCredentialsProvider(ctx context.Context, requestNamespace string,
	cluster *rabbitmqv1beta1.RabbitmqCluster) (CredentialsProvider, *corev1.Service, error) {
	// use credentials in namespace Kubernetes Secret
	if cluster.Status.Binding == nil {
		return nil, nil, errors.New("no status.binding set")
	}
	if cluster.Status.DefaultUser == nil {
		return nil, nil, errors.New("no status.defaultUser set")
	}
	var namespace = namespace(requestNamespace, cluster)

	secret := &corev1.Secret{}
	if err := ccp.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.Binding.Name}, secret); err != nil {
		return nil, nil, err
	}
	credentialsProvider, err := readCredentialsFromKubernetesSecret(secret)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to retrieve credentials from Kubernetes secret %s: %w", secret.Name, err)
	}
	svc := &corev1.Service{}
	if err := ccp.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, nil, err
	}
	return credentialsProvider, svc, nil
}
func (ccp *K8sSecretClusterCredentialsProvider) Accept(cluster *rabbitmqv1beta1.RabbitmqCluster) bool {
	return true
}
func (ccp *VaultClusterCredentialsProvider) GetCredentialsProvider(ctx context.Context, requestNamespace string,
	cluster *rabbitmqv1beta1.RabbitmqCluster) (CredentialsProvider, *corev1.Service, error) {
	cp, err := ccp.vaultReader.ReadCredentials(cluster.Spec.SecretBackend.Vault.DefaultUserPath)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to retrieve credentials from secret store: %w", err)
	}
	var namespace = namespace(requestNamespace, cluster)

	svc := &corev1.Service{}
	if err := ccp.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.ObjectMeta.Name}, svc); err != nil {
		return nil, nil, err
	}

	return cp, svc, nil
}
func (ccp *VaultClusterCredentialsProvider) Accept(cluster *rabbitmqv1beta1.RabbitmqCluster) bool {
	return cluster.Spec.SecretBackend.Vault != nil && cluster.Spec.SecretBackend.Vault.DefaultUserPath != ""
}
