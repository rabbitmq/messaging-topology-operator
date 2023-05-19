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

// +kubebuilder:rbac:groups=rabbitmq.com,resources=shovels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=shovels/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=shovels/status,verbs=get;update;patch

type ShovelReconciler struct {
	client.Client
}

func (r *ShovelReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	shovel := obj.(*topology.Shovel)
	srcUri, destUri, err := r.getUris(ctx, shovel)
	if err != nil {
		return fmt.Errorf("failed to parse shovel uri secret; secret name: %s, error: %w", shovel.Spec.UriSecret.Name, err)
	}
	definition, err := internal.GenerateShovelDefinition(shovel, srcUri, destUri)
	if err != nil {
		return fmt.Errorf("failed to generate shovel definition: %w", err)
	}
	return validateResponse(client.DeclareShovel(shovel.Spec.Vhost, shovel.Spec.Name, *definition))
}
func (r *ShovelReconciler) getUris(ctx context.Context, shovel *topology.Shovel) (string, string, error) {
	if shovel.Spec.UriSecret == nil {
		return "", "", fmt.Errorf("no uri secret provided")
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: shovel.Spec.UriSecret.Name, Namespace: shovel.Namespace}, secret); err != nil {
		return "", "", err
	}

	srcUri, ok := secret.Data["srcUri"]
	if !ok {
		return "", "", fmt.Errorf("could not find key 'srcUri' in secret %s", secret.Name)
	}

	destUri, ok := secret.Data["destUri"]
	if !ok {
		return "", "", fmt.Errorf("could not find key 'srcUri' in secret %s", secret.Name)
	}

	return string(srcUri), string(destUri), nil
}

// DeleteFunc deletes shovel configuration from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *ShovelReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	shovel := obj.(*topology.Shovel)
	err := validateResponseForDeletion(client.DeleteShovel(shovel.Spec.Vhost, shovel.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find shovel parameter; no need to delete it", "shovel", shovel.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}
