package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=federations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=federations/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=federations/status,verbs=get;update;patch

type FederationReconciler struct {
	client.Client
}

func (r *FederationReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	federation := obj.(*topology.Federation)
	uri, err := r.getUri(ctx, federation)
	if err != nil {
		return fmt.Errorf("failed to parse federation uri secret; secret name: %s, error: %w", federation.Spec.UriSecret.Name, err)
	}
	return validateResponse(client.PutFederationUpstream(federation.Spec.Vhost, federation.Spec.Name, internal.GenerateFederationDefinition(federation, uri)))
}

func (r *FederationReconciler) getUri(ctx context.Context, federation *topology.Federation) (string, error) {
	if federation.Spec.UriSecret == nil {
		return "", fmt.Errorf("no uri secret provided")
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: federation.Spec.UriSecret.Name, Namespace: federation.Namespace}, secret); err != nil {
		return "", err
	}

	uri, ok := secret.Data["uri"]
	if !ok {
		return "", fmt.Errorf("could not find key 'uri' in secret %s", secret.Name)
	}

	return string(uri), nil
}

// DeleteFunc deletes federation from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *FederationReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	federation := obj.(*topology.Federation)
	if shouldSkipDeletion(ctx, federation.Spec.DeletionPolicy, federation.Spec.Name) {
		return nil
	}
	err := validateResponseForDeletion(client.DeleteFederationUpstream(federation.Spec.Vhost, federation.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find federation upstream parameter; no need to delete it", "federation", federation.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}
