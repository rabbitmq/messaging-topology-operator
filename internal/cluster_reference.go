package internal

import (
	"context"
	"errors"
	"fmt"

	rabbitmqv1beta2 "github.com/rabbitmq/cluster-operator/api/v1beta2"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	NoSuchRabbitmqClusterError = errors.New("RabbitmqCluster object does not exist")
	ResourceNotAllowedError    = errors.New("Resource is not allowed to reference defined cluster reference. Check the namespace of the resource is allowed as part of the cluster's .spec.messagingTopologyNamespaces field")
)

func ParseRabbitmqClusterReference(ctx context.Context, c client.Client, rmq topology.RabbitmqClusterReference, requestNamespace string) (*rabbitmqv1beta2.RabbitmqCluster, *corev1.Service, *corev1.Secret, error) {
	var namespace string
	if rmq.Namespace == "" {
		namespace = requestNamespace
	} else {
		namespace = rmq.Namespace
	}
	cluster := &rabbitmqv1beta2.RabbitmqCluster{}
	if err := c.Get(ctx, types.NamespacedName{Name: rmq.Name, Namespace: namespace}, cluster); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get cluster from reference: %s Error: %w", err, NoSuchRabbitmqClusterError)
	}

	if rmq.Namespace != "" && rmq.Namespace != requestNamespace {
		var isAllowed bool
		for _, allowedNamespace := range cluster.Spec.MessagingTopologyNamespaces {
			if requestNamespace == allowedNamespace {
				isAllowed = true
				break
			}
		}
		if isAllowed == false {
			return nil, nil, nil, ResourceNotAllowedError
		}
	}

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

	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, nil, nil, err
	}
	return cluster, svc, secret, nil
}
