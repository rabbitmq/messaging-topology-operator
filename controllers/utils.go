package controllers

import (
	"context"
	"errors"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	"io/ioutil"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net"
)

// returns a http client for the given RabbitmqCluster
// assumes the RabbitmqCluster is reachable using its service's ClusterIP
func rabbitholeClient(ctx context.Context, c client.Client, rmq v1alpha1.RabbitmqClusterReference) (*rabbithole.Client, error) {
	svc, secret, err := serviceSecretFromReference(ctx, c, rmq)
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

func serviceSecretFromReference(ctx context.Context, c client.Client, rmq v1alpha1.RabbitmqClusterReference) (*corev1.Service, *corev1.Secret, error) {
	cluster := &rabbitmqv1beta1.RabbitmqCluster{}
	if err := c.Get(ctx, types.NamespacedName{Name: rmq.Name, Namespace: rmq.Namespace}, cluster); err != nil {
		return nil, nil, err
	}

	secret := &corev1.Secret{}
	// TODO: use cluster.Status.Binding instead of cluster.Status.DefaultUser.SecretReference.Name after the PR exposes Status.Binding is released
	if err := c.Get(ctx, types.NamespacedName{Namespace: rmq.Namespace, Name: cluster.Status.DefaultUser.SecretReference.Name}, secret); err != nil {
		return nil, nil, err
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: rmq.Namespace, Name: cluster.Status.DefaultUser.ServiceReference.Name}, svc); err != nil {
		return nil, nil, err
	}
	return svc, secret, nil
}

// TODO: check possible status code response from RabbitMQ
// validate status code above 300 might not be all failure case
func validateResponse(res *http.Response, err error) error {
	if err != nil {
		return err
	}

	if res.StatusCode >= http.StatusMultipleChoices {
		body, _ := ioutil.ReadAll(res.Body)
		res.Body.Close()
		return fmt.Errorf("request failed with status code %d and body %q", res.StatusCode, body)
	}
	return nil
}
