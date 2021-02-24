package controllers

import (
	"context"
	"errors"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	//"k8s.io/apimachinery/pkg/runtime"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net"
)

// returns a http client for the given RabbitmqClusterReference
// assumes that the RabbitmqCluster is reachable using its ClusterIP
// assumes
func (r *QueueReconciler) rabbitClient(ctx context.Context, rmq v1beta1.RabbitmqClusterReference) (*rabbithole.Client, error) {
	svc, secret, err := r.serviceSecretFromReference(ctx, rmq)
	if err != nil {
		return nil, fmt.Errorf("failed to get service or secret object from specified rabbitmqcluster: %v", err)
	}

	ip := net.ParseIP(svc.Spec.ClusterIP)
	if ip == nil {
		return nil, fmt.Errorf("failed to get Cluster IP: invalid ClusterIP %q", svc.Spec.ClusterIP)
	}

	port, err := managementPort(svc)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("http://%s:%d", ip.String(), port)

	defaultUser, found := secret.Data["username"]
	if !found {
		return nil, errors.New("failed to retrieve username: key username missing from secret")
	}

	defaultUserPass, found := secret.Data["password"]
	if !found {
		return nil, errors.New("failed to retrieve username: key password missing from secret")
	}

	client, err := rabbithole.NewClient(endpoint, string(defaultUser), string(defaultUserPass))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate rabbit client: %v", err)
	}

	return client, nil
}

// returns rmq management port from given service
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

// returns the service and the secret object from given RabbitmqClusterReference
func (r *QueueReconciler) serviceSecretFromReference(ctx context.Context, rmq v1beta1.RabbitmqClusterReference) (*corev1.Service, *corev1.Secret, error) {
	cluster := &rabbitmqv1beta1.RabbitmqCluster{}
	if err := r.Get(ctx, types.NamespacedName{Name: rmq.Name, Namespace: rmq.Namespace}, cluster); err != nil {
		return nil, nil, err
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: rmq.Namespace, Name: cluster.Status.DefaultUser.SecretReference.Name}, secret); err != nil {
		return nil, nil, err
	}

	svc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: rmq.Namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, nil, err
	}
	return svc, secret, nil
}
